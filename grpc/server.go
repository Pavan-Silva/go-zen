package grpc

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Server wraps grpc.Server with Zen-style setup
type Server struct {
	*grpc.Server
	addr string
}

// NewServer creates a new gRPC server with optional interceptors
func NewServer(addr string, opts ...grpc.ServerOption) *Server {
	server := grpc.NewServer(opts...)
	return &Server{
		Server: server,
		addr:   addr,
	}
}

// ListenAndServe starts the gRPC server
func (s *Server) ListenAndServe() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	log.Printf("gRPC server listening on %s", s.addr)
	return s.Server.Serve(lis)
}

// EnableReflection enables gRPC server reflection for debugging
func (s *Server) EnableReflection() {
	reflection.Register(s.Server)
}

// UnaryInterceptor is a function type for unary interceptors
type UnaryInterceptor = grpc.UnaryServerInterceptor

// ChainUnary creates chained unary interceptors
func ChainUnary(interceptors ...UnaryInterceptor) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(interceptors...)
}

// StreamInterceptor is a function type for stream interceptors
type StreamInterceptor = grpc.StreamServerInterceptor

// ChainStream creates chained stream interceptors
func ChainStream(interceptors ...StreamInterceptor) grpc.ServerOption {
	return grpc.ChainStreamInterceptor(interceptors...)
}
