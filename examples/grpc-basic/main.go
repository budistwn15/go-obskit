package main

import (
	"net"

	"github.com/budistwn15/go-obskit/adapters/grpcx/v2"
	obskit "github.com/budistwn15/go-obskit/v2"
	"google.golang.org/grpc"
)

func main() {
	log := obskit.New(obskit.Config{Service: "grpc-api", Environment: "production"})
	server := grpc.NewServer(
		grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)),
		grpc.StreamInterceptor(grpcx.StreamServerInterceptor(log)),
	)
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Error("listen failed", "error", err)
		return
	}
	if err := server.Serve(listener); err != nil {
		log.Error("serve failed", "error", err)
	}
}
