package http2grpc_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/v-electrolux/http2grpc"
)

const (
	GrpcContentType          = 0
	GrpcPlusProtoContentType = 1
)

type TestHttpResData struct {
	cfgBodyAsStatusMessage bool

	backendHttpResStatusCode int
	backendHttpResBody       []byte

	httpTrailerPredeclare bool

	expGrpcResStatusCode int
	expGrpcResStatusMsg  string
	expGrpcResBody       []byte
}

func TestOkHttpFromBackend(t *testing.T) {
	data := TestHttpResData{
		backendHttpResStatusCode: 200,
		backendHttpResBody:       []byte("http query executed successfully"),

		expGrpcResStatusCode: 0,
		expGrpcResStatusMsg:  "",
		expGrpcResBody:       []byte("http query executed successfully"),
	}
	test2DMatrixForHttp(t, data)
}

func TestUnauthorizedHttpFromBackendEnabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: true,

		backendHttpResStatusCode: 401,
		backendHttpResBody:       []byte("user unauthenticated"),

		expGrpcResStatusCode: 16,
		expGrpcResStatusMsg:  "user unauthenticated",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func TestUnauthorizedHttpFromBackendDisabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: false,

		backendHttpResStatusCode: 401,
		backendHttpResBody:       []byte("user unauthenticated"),

		expGrpcResStatusCode: 16,
		expGrpcResStatusMsg:  "",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func TestForbiddenHttpFromBackendEnabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: true,

		backendHttpResStatusCode: 403,
		backendHttpResBody:       []byte("forbidden"),

		expGrpcResStatusCode: 7,
		expGrpcResStatusMsg:  "forbidden",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func TestForbiddenHttpFromBackendDisabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: false,

		backendHttpResStatusCode: 403,
		backendHttpResBody:       []byte("forbidden"),

		expGrpcResStatusCode: 7,
		expGrpcResStatusMsg:  "",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func TestInternalErrorHttpFromBackendEnabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: true,

		backendHttpResStatusCode: 500,
		backendHttpResBody:       []byte("internal server error"),

		expGrpcResStatusCode: 2,
		expGrpcResStatusMsg:  "internal server error",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func TestInternalErrorHttpFromBackendDisabledBodyAsMsg(t *testing.T) {
	data := TestHttpResData{
		cfgBodyAsStatusMessage: false,

		backendHttpResStatusCode: 500,
		backendHttpResBody:       []byte("internal server error"),

		expGrpcResStatusCode: 2,
		expGrpcResStatusMsg:  "",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	test1DMatrixForHttp(t, data)
}

func test1DMatrixForHttp(t *testing.T, data TestHttpResData) {
	t.Helper()

	t.Run("true", func(t *testing.T) {
		data.httpTrailerPredeclare = true
		testHttpRequest(t, data)
	})
	t.Run("false", func(t *testing.T) {
		data.httpTrailerPredeclare = false
		testHttpRequest(t, data)
	})
}

func test2DMatrixForHttp(t *testing.T, data TestHttpResData) {
	t.Helper()

	t.Run("true,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.httpTrailerPredeclare = true
		testHttpRequest(t, data)
	})
	t.Run("false,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.httpTrailerPredeclare = true
		testHttpRequest(t, data)
	})
	t.Run("true,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.httpTrailerPredeclare = false
		testHttpRequest(t, data)
	})
	t.Run("false,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.httpTrailerPredeclare = false
		testHttpRequest(t, data)
	})
}

func testHttpRequest(t *testing.T, data TestHttpResData) {
	t.Helper()

	cfg := http2grpc.CreateConfig()
	cfg.BodyAsStatusMessage = data.cfgBodyAsStatusMessage
	cfg.LogLevel = "info"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(data.backendHttpResStatusCode)
		rw.Write(data.backendHttpResBody)
	})

	handler, err := http2grpc.New(ctx, next, cfg, "http2grpc_status_code")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(recorder, req)
	resp := recorder.Result()

	assertStatusCode(t, resp, http.StatusOK)
	assertBody(t, resp, data.expGrpcResBody)
	assertHeader(t, resp, "Content-Length", "")
	assertHeader(t, resp, "Content-Type", "application/grpc")
	assertArrayHeader(t, resp, "Trailer", []string{"grpc-status", "grpc-message"})
	assertTrailer(t, resp, "grpc-status", strconv.Itoa(data.expGrpcResStatusCode))
	assertTrailer(t, resp, "grpc-message", data.expGrpcResStatusMsg)
}

type TestGrpcResData struct {
	cfgBodyAsStatusMessage bool

	backendGrpcResStatusCode  int
	backendGrpcResStatusMsg   string
	backendGrpcResBody        []byte
	backendGrpcResContentType int

	httpTrailerPredeclare bool

	expGrpcResStatusCode int
	expGrpcResStatusMsg  string
	expGrpcResBody       []byte
}

func TestOkGrpcFromBackend(t *testing.T) {
	data := TestGrpcResData{
		backendGrpcResStatusCode: 0,
		backendGrpcResStatusMsg:  "",
		backendGrpcResBody:       []byte("pretend to be real grpc body in protobuf format"),

		expGrpcResStatusCode: 0,
		expGrpcResStatusMsg:  "",
		expGrpcResBody:       []byte("pretend to be real grpc body in protobuf format"),
	}
	testMatrixForGrpc(t, data)
}

func TestDeadlineExceededGrpcFromBackend(t *testing.T) {
	data := TestGrpcResData{
		backendGrpcResStatusCode: 4,
		backendGrpcResStatusMsg:  "some deadline exceeded",
		backendGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},

		expGrpcResStatusCode: 4,
		expGrpcResStatusMsg:  "some deadline exceeded",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	testMatrixForGrpc(t, data)
}

func TestUnknownGrpcFromBackend(t *testing.T) {
	data := TestGrpcResData{
		backendGrpcResStatusCode: 2,
		backendGrpcResStatusMsg:  "something strange and unexpected happen",
		backendGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},

		expGrpcResStatusCode: 2,
		expGrpcResStatusMsg:  "something strange and unexpected happen",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	testMatrixForGrpc(t, data)
}

func TestPermissionDeniedGrpcFromBackend(t *testing.T) {
	data := TestGrpcResData{
		backendGrpcResStatusCode: 7,
		backendGrpcResStatusMsg:  "user unauthorized to do some action",
		backendGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},

		expGrpcResStatusCode: 7,
		expGrpcResStatusMsg:  "user unauthorized to do some action",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	testMatrixForGrpc(t, data)
}

func TestUnauthenticatedGrpcFromBackend(t *testing.T) {
	data := TestGrpcResData{
		backendGrpcResStatusCode: 16,
		backendGrpcResStatusMsg:  "user unauthenticated",
		backendGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},

		expGrpcResStatusCode: 16,
		expGrpcResStatusMsg:  "user unauthenticated",
		expGrpcResBody:       []byte{0x00, 0x00, 0x00, 0x00, 0x00},
	}
	testMatrixForGrpc(t, data)
}

func testMatrixForGrpc(t *testing.T, data TestGrpcResData) {
	t.Helper()

	t.Run("true,0,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.backendGrpcResContentType = GrpcContentType
		data.httpTrailerPredeclare = true
		testGrpcRequest(t, data)
	})
	t.Run("false,0,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.backendGrpcResContentType = GrpcContentType
		data.httpTrailerPredeclare = true
		testGrpcRequest(t, data)
	})
	t.Run("true,1,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.backendGrpcResContentType = GrpcPlusProtoContentType
		data.httpTrailerPredeclare = true
		testGrpcRequest(t, data)
	})
	t.Run("false,1,true", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.backendGrpcResContentType = GrpcPlusProtoContentType
		data.httpTrailerPredeclare = true
		testGrpcRequest(t, data)
	})
	t.Run("true,0,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.backendGrpcResContentType = GrpcContentType
		data.httpTrailerPredeclare = false
		testGrpcRequest(t, data)
	})
	t.Run("false,0,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.backendGrpcResContentType = GrpcContentType
		data.httpTrailerPredeclare = false
		testGrpcRequest(t, data)
	})
	t.Run("true,1,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = true
		data.backendGrpcResContentType = GrpcPlusProtoContentType
		data.httpTrailerPredeclare = false
		testGrpcRequest(t, data)
	})
	t.Run("false,1,false", func(t *testing.T) {
		data.cfgBodyAsStatusMessage = false
		data.backendGrpcResContentType = GrpcPlusProtoContentType
		data.httpTrailerPredeclare = false
		testGrpcRequest(t, data)
	})
}

func testGrpcRequest(t *testing.T, data TestGrpcResData) {
	t.Helper()

	cfg := http2grpc.CreateConfig()
	cfg.BodyAsStatusMessage = data.cfgBodyAsStatusMessage
	cfg.LogLevel = "info"

	ctx := context.Background()
	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch data.backendGrpcResContentType {
		case GrpcContentType:
			rw.Header().Set("Content-Type", "application/grpc")
		case GrpcPlusProtoContentType:
			rw.Header().Set("Content-Type", "application/grpc+proto")
		}

		if data.httpTrailerPredeclare {
			rw.Header().Set("Trailer", "grpc-status")
			rw.Header().Add("Trailer", "grpc-message")
		}

		rw.WriteHeader(http.StatusOK)
		rw.Write(data.backendGrpcResBody)

		if data.httpTrailerPredeclare {
			rw.Header().Set("grpc-status", strconv.Itoa(data.backendGrpcResStatusCode))
			rw.Header().Set("grpc-message", data.backendGrpcResStatusMsg)
		} else {
			rw.Header().Set(http.TrailerPrefix+"grpc-status", strconv.Itoa(data.backendGrpcResStatusCode))
			rw.Header().Set(http.TrailerPrefix+"grpc-message", data.backendGrpcResStatusMsg)
		}
	})

	handler, err := http2grpc.New(ctx, next, cfg, "http2grpc")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	if err != nil {
		t.Fatal(err)
	}

	handler.ServeHTTP(recorder, req)
	resp := recorder.Result()

	if !recorder.Flushed {
		t.Errorf("expected rw.Flushed to be true")
	}
	assertStatusCode(t, resp, http.StatusOK)
	assertBody(t, resp, data.expGrpcResBody)
	switch data.backendGrpcResContentType {
	case GrpcContentType:
		assertHeader(t, resp, "Content-Type", "application/grpc")
	case GrpcPlusProtoContentType:
		assertHeader(t, resp, "Content-Type", "application/grpc+proto")
	}
	assertHeader(t, resp, "Content-Length", "")
	assertTrailer(t, resp, "grpc-status", strconv.Itoa(data.expGrpcResStatusCode))
	assertTrailer(t, resp, "grpc-message", data.expGrpcResStatusMsg)
}

func assertStatusCode(t *testing.T, res *http.Response, expected int) {
	t.Helper()

	got := res.StatusCode
	if got != expected {
		t.Errorf("expected status code value: `%d`, got value: `%d`", expected, got)
	}
}

func assertBody(t *testing.T, res *http.Response, expected []byte) {
	t.Helper()

	got, _ := io.ReadAll(res.Body)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected body value: `%s`, got value: `%s`", expected, got)
	}
}

func assertHeader(t *testing.T, res *http.Response, key string, expected string) {
	t.Helper()

	got := res.Header.Get(key)
	if got != expected {
		t.Errorf("expected header %s value: `%s`, got value: `%s`", key, expected, got)
	}
}

func assertArrayHeader(t *testing.T, res *http.Response, key string, expected []string) {
	t.Helper()

	got := res.Header.Values(key)
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("expected header %s value: `%s`, got value: `%s`", key, expected, got)
	}
}

func assertTrailer(t *testing.T, res *http.Response, key string, expected string) {
	t.Helper()

	got := res.Trailer.Get(key)
	if got != expected {
		t.Errorf("expected trailer %s value: `%s`, got value: `%s`", key, expected, got)
	}
}
