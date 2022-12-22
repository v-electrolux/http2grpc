package grpc

import (
	"net/http"
)

// from https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
const (
	OK                  = 0  //nolint:revive // taken from gRPC spec
	CANCELLED           = 1  //nolint:revive // taken from gRPC spec
	UNKNOWN             = 2  //nolint:revive // taken from gRPC spec
	INVALID_ARGUMENT    = 3  //nolint:revive // taken from gRPC spec
	DEADLINE_EXCEEDED   = 4  //nolint:revive // taken from gRPC spec
	NOT_FOUND           = 5  //nolint:revive // taken from gRPC spec
	ALREADY_EXISTS      = 6  //nolint:revive // taken from gRPC spec
	PERMISSION_DENIED   = 7  //nolint:revive // taken from gRPC spec
	RESOURCE_EXHAUSTED  = 8  //nolint:revive // taken from gRPC spec
	FAILED_PRECONDITION = 9  //nolint:revive // taken from gRPC spec
	ABORTED             = 10 //nolint:revive // taken from gRPC spec
	OUT_OF_RANGE        = 11 //nolint:revive // taken from gRPC spec
	UNIMPLEMENTED       = 12 //nolint:revive // taken from gRPC spec
	INTERNAL            = 13 //nolint:revive // taken from gRPC spec
	UNAVAILABLE         = 14 //nolint:revive // taken from gRPC spec
	DATA_LOSS           = 15 //nolint:revive // taken from gRPC spec
	UNAUTHENTICATED     = 16 //nolint:revive // taken from gRPC spec
)

// HTTP2grpc map based on https://github.com/grpc/grpc/blob/master/doc/http-grpc-status-mapping.md
var (
	HTTP2grpc = map[int]int{ //nolint:gochecknoglobals // static map from gRPC spec
		http.StatusBadRequest:         INTERNAL,
		http.StatusUnauthorized:       UNAUTHENTICATED,
		http.StatusForbidden:          PERMISSION_DENIED,
		http.StatusNotFound:           UNIMPLEMENTED,
		http.StatusTooManyRequests:    UNAVAILABLE,
		http.StatusBadGateway:         UNAVAILABLE,
		http.StatusServiceUnavailable: UNAVAILABLE,
		http.StatusGatewayTimeout:     UNAVAILABLE,
		// if other, code must be UNKNOWN
	}
)
