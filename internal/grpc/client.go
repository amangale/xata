package grpc

import (
	"xata/internal/o11y"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DefaultMaxRecvMsgBytes raises the per-call receive limit above gRPC's 4 MiB
// default. Callers can override it with their own grpc.WithDefaultCallOptions.
const DefaultMaxRecvMsgBytes = 16 * 1024 * 1024

// Default retry policy for gRPC clients
const RetryPolicy = `{
	"methodConfig": [{
		"name": [{}],
		"retryPolicy": {
			"maxAttempts": 5,
			"initialBackoff": "1s",
			"maxBackoff": "5s",
			"backoffMultiplier": 2,
			"retryableStatusCodes": ["UNAVAILABLE"]
		}
	}]
}`

// ClientConnection is a wrapper around grpc.ClientConnection
type ClientConnection struct {
	*grpc.ClientConn
}

// NewClient creates a new gRPC client connection with proper instrumentation and settings
func NewClient(o *o11y.O, url string, extraOpts ...grpc.DialOption) (*ClientConnection, error) {
	logger := o.Logger()
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(DefaultMaxRecvMsgBytes)),
		o11y.GRPCUnaryInterceptorLogs(&logger),
		grpc.WithDefaultServiceConfig(RetryPolicy),
	}
	opts = append(opts, o11y.GRPCClientStatHandlers(o)...)
	opts = append(opts, extraOpts...)

	conn, err := grpc.NewClient(url, opts...)
	if err != nil {
		return nil, err
	}

	return &ClientConnection{ClientConn: conn}, nil
}
