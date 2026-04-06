package grpc

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// LoggingInterceptor logs gRPC requests
func LoggingInterceptor() UnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		log.Printf("gRPC %s - %v - %s", info.FullMethod, duration, grpc.Code(err))
		return resp, err
	}
}

// RecoveryInterceptor recovers from panics
func RecoveryInterceptor() UnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("gRPC panic recovered: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// AuthInterceptor provides basic authentication
func AuthInterceptor(authFunc func(token string) bool) UnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md["authorization"]
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing auth token")
		}

		if !authFunc(tokens[0]) {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		return handler(ctx, req)
	}
}

// CORSInterceptor handles CORS for gRPC-Web (if needed)
func CORSInterceptor() UnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// gRPC handles CORS differently, but this is a placeholder
		return handler(ctx, req)
	}
}

// TimeoutInterceptor adds timeout to requests
func TimeoutInterceptor(timeout time.Duration) UnaryInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return handler(ctx, req)
	}
}