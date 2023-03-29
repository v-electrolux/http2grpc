package http2grpc

import (
	"bufio"
	"context"
	"fmt"
	"github.com/v-electrolux/http2grpc/grpc"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
)

const (
	GrpcStatusHeaderName               = "grpc-status"
	GrpcMessageHeaderName              = "grpc-message"
	TrailerHeaderName                  = "Trailer"
	ContentLengthHeaderName            = "Content-Length"
	ContentTypeHeaderName              = "Content-Type"
	ContentTypeHeaderGrpcValue         = "application/grpc"
	ContentTypeHeaderGrpcWithBodyValue = "application/grpc+proto"
)

//nolint:gochecknoglobals // TODO exchange for traefik log when available
var (
	// EmptyGrpcBody format 1 byte for Compressed-Flag, 4 bytes for Message-Length, 0 bytes for Message
	// all from gRPC spec https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md
	EmptyGrpcBody = []byte{0x00, 0x00, 0x00, 0x00, 0x00}

	LoggerINFO  = log.New(ioutil.Discard, "INFO:  http2grpc: ", log.Ldate|log.Ltime|log.Lshortfile)
	LoggerDEBUG = log.New(ioutil.Discard, "DEBUG: http2grpc: ", log.Ldate|log.Ltime|log.Lshortfile)
)

type Config struct {
	LogLevel            string `yaml:"logLevel"`
	BodyAsStatusMessage bool   `yaml:"bodyAsStatusMessage"`
}

func CreateConfig() *Config {
	return &Config{
		BodyAsStatusMessage: false,
		LogLevel:            "info",
	}
}

type HTTP2Grpc struct {
	next   http.Handler
	config *Config
	name   string
}

func New(_ context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	switch config.LogLevel {
	case "info":
		LoggerINFO.SetOutput(os.Stdout)
	case "debug":
		LoggerINFO.SetOutput(os.Stdout)
		LoggerDEBUG.SetOutput(os.Stdout)
	default:
		return nil, fmt.Errorf("ERROR: http2grpc: %s", config.LogLevel)
	}

	return &HTTP2Grpc{
		next:   next,
		name:   name,
		config: config,
	}, nil
}

func (h *HTTP2Grpc) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	LoggerDEBUG.Printf("ServeHTTP started")

	bodyAsStatusMessage := h.config.BodyAsStatusMessage

	LoggerDEBUG.Printf("ServeHTTP config read")

	rwMod := newHTTP2grpcModifier(rw, bodyAsStatusMessage)

	LoggerDEBUG.Printf("ServeHTTP http2grpcModifier created")
	h.next.ServeHTTP(rwMod, req)
	LoggerDEBUG.Printf("ServeHTTP completed")
	LoggerINFO.Printf("executed successful")
}

type http2grpcModifier struct {
	responseWriter        http.ResponseWriter
	responseWriterFlusher http.Flusher
	// sentHTTPStatusCode stores original http status code for use in Write method
	sentHTTPStatusCode int
	// backendUseGrpc is whether the backend send response in grpc format
	backendUseGrpc bool
	// bodyAsStatusMessage enforce convert body to utf8 string and set as grpc status message.
	bodyAsStatusMessage bool
	// headerSent is whether the headers have already been sent, either through Write or WriteHeader.
	headerSent bool
}

func newHTTP2grpcModifier(rw http.ResponseWriter, bodyAsStatusMessage bool) http.ResponseWriter {
	http2grpcMod := &http2grpcModifier{
		responseWriter:        rw,
		responseWriterFlusher: nil,
		sentHTTPStatusCode:    http.StatusOK,
		backendUseGrpc:        false,
		bodyAsStatusMessage:   bodyAsStatusMessage,
		headerSent:            false,
	}

	if flusher, ok := rw.(http.Flusher); ok {
		http2grpcMod.responseWriterFlusher = flusher
	}

	return http2grpcMod
}

func (h *http2grpcModifier) Header() http.Header {
	LoggerDEBUG.Printf("Header() called, headers: %+v", h.responseWriter.Header())

	return h.responseWriter.Header()
}

func (h *http2grpcModifier) Write(buf []byte) (int, error) {
	LoggerDEBUG.Printf("Write() called, headers: %+v", h.responseWriter.Header())

	isHTTPResponseFromBackend := !h.backendUseGrpc
	isNotOkStatusFromBackend := h.sentHTTPStatusCode != http.StatusOK

	LoggerDEBUG.Printf("Write() isHTTPResponseFromBackend: %t, isNotOkStatusFromBackend: %t",
		isHTTPResponseFromBackend, isNotOkStatusFromBackend)

	h.WriteHeader(http.StatusOK)

	if isHTTPResponseFromBackend && isNotOkStatusFromBackend && h.bodyAsStatusMessage {
		LoggerDEBUG.Printf("Write() `grpc-message` header set to %s", string(buf))
		h.responseWriter.Header().Set(GrpcMessageHeaderName, string(buf))
	}

	var body []byte
	if isHTTPResponseFromBackend && isNotOkStatusFromBackend {
		body = EmptyGrpcBody
	} else {
		body = buf
	}

	count, err := h.responseWriter.Write(body)
	LoggerDEBUG.Printf("Write() body wrote, length %d", len(body))

	// need for gRPC stream, because response can be buffered
	// delaying messages via stream
	if h.responseWriterFlusher != nil {
		h.responseWriterFlusher.Flush()
	}

	return count, err
}

func (h *http2grpcModifier) WriteHeader(statusCode int) {
	LoggerDEBUG.Printf("WriteHeader() called, begin to send headers: %+v, exiting", h.responseWriter.Header())

	if h.headerSent {
		LoggerDEBUG.Printf("WriteHeader() headers already sent, exiting")
		return
	}

	if isHTTPResponseFromBackend := !h.checkResponseInGrpcFormat(); isHTTPResponseFromBackend {
		LoggerDEBUG.Printf("WriteHeader() converting http to grpc")
		h.convertHTTPToGrpc(statusCode)
	} else {
		LoggerDEBUG.Printf("WriteHeader() grpc leave as is, headers: %+v", h.responseWriter.Header())
		h.responseWriter.WriteHeader(http.StatusOK)
		h.backendUseGrpc = true
	}

	h.sentHTTPStatusCode = statusCode
	h.headerSent = true
	LoggerDEBUG.Printf("WriteHeader() headers sent: %+v, exiting", h.responseWriter.Header())
}

func (h *http2grpcModifier) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	LoggerDEBUG.Printf("Hijack() called")

	hijacker, ok := h.responseWriter.(http.Hijacker)

	if !ok {
		return nil, nil, fmt.Errorf("ERROR: http2grpc: %T is not a http.Hijacker", h.responseWriter)
	}

	return hijacker.Hijack()
}

func (h *http2grpcModifier) Flush() {
	LoggerDEBUG.Printf("Flush() called")
	h.WriteHeader(http.StatusOK)

	if flusher, ok := h.responseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (h *http2grpcModifier) convertHTTPToGrpc(statusCode int) {
	// gRPC status code and message send in trailers because of gRPC implementation over HTTP/2
	h.responseWriter.Header().Set(TrailerHeaderName, GrpcStatusHeaderName)
	h.responseWriter.Header().Add(TrailerHeaderName, GrpcMessageHeaderName)

	// always set application/grpc because of gRPC implementation over HTTP/2
	h.responseWriter.Header().Set(ContentTypeHeaderName, ContentTypeHeaderGrpcValue)

	// drop the body and delete content length
	h.responseWriter.Header().Del(ContentLengthHeaderName)

	// always set HTTP OK because of gRPC implementation over HTTP/2
	h.responseWriter.WriteHeader(http.StatusOK)

	grpcCodeString := getGrpcStatusCode(statusCode)
	h.responseWriter.Header().Set(GrpcStatusHeaderName, grpcCodeString)

	if !h.bodyAsStatusMessage {
		h.responseWriter.Header().Set(GrpcMessageHeaderName, "")
	}
}

func (h *http2grpcModifier) checkResponseInGrpcFormat() bool {
	contentType := h.responseWriter.Header().Get(ContentTypeHeaderName)
	contentTypeIsGrpc := (contentType == ContentTypeHeaderGrpcValue) ||
		(contentType == ContentTypeHeaderGrpcWithBodyValue)

	return contentTypeIsGrpc
}

func getGrpcStatusCode(httpStatusCode int) string {
	var grpcCodeString string
	if httpStatusCode == http.StatusOK {
		grpcCodeString = strconv.Itoa(grpc.OK)
	} else if grpcCode, ok := grpc.HTTP2grpc[httpStatusCode]; ok {
		grpcCodeString = strconv.Itoa(grpcCode)
	} else {
		grpcCodeString = strconv.Itoa(grpc.UNKNOWN)
	}

	return grpcCodeString
}
