// Package grpcx provides structured gRPC server and client interceptors.
package grpcx

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor logs one event after each unary server call.
func UnaryServerInterceptor(log *slog.Logger) grpc.UnaryServerInterceptor {
	log = fallbackLogger(log)
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		resp, err := handler(ctx, req)
		logRPC(log, ctx, "rpc.server.complete", "inbound", methodName(info.FullMethod), time.Since(started), err)
		return resp, err
	}
}

// StreamServerInterceptor logs one event after each server stream completes.
func StreamServerInterceptor(log *slog.Logger) grpc.StreamServerInterceptor {
	log = fallbackLogger(log)
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		started := time.Now()
		err := handler(srv, stream)
		logRPC(log, stream.Context(), "rpc.server.stream.complete", "inbound", methodName(info.FullMethod), time.Since(started), err)
		return err
	}
}

// UnaryClientInterceptor logs one event after each unary client call.
func UnaryClientInterceptor(log *slog.Logger) grpc.UnaryClientInterceptor {
	log = fallbackLogger(log)
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		started := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		logRPC(log, ctx, "rpc.client.complete", "outbound", methodName(method), time.Since(started), err)
		return err
	}
}

// StreamClientInterceptor logs when the client stream reaches a terminal operation.
func StreamClientInterceptor(log *slog.Logger) grpc.StreamClientInterceptor {
	log = fallbackLogger(log)
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		started := time.Now()
		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			logRPC(log, ctx, "rpc.client.stream.complete", "outbound", methodName(method), time.Since(started), err)
			return nil, err
		}
		return &clientStream{ClientStream: stream, finish: func(err error) {
			logRPC(log, ctx, "rpc.client.stream.complete", "outbound", methodName(method), time.Since(started), terminalError(err))
		}}, nil
	}
}

type clientStream struct {
	grpc.ClientStream
	once   sync.Once
	finish func(error)
}

func (s *clientStream) SendMsg(msg any) error {
	err := s.ClientStream.SendMsg(msg)
	if err != nil {
		s.once.Do(func() { s.finish(err) })
	}
	return err
}

func (s *clientStream) RecvMsg(msg any) error {
	err := s.ClientStream.RecvMsg(msg)
	if err != nil {
		s.once.Do(func() { s.finish(err) })
	}
	return err
}

func (s *clientStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	if err != nil {
		s.once.Do(func() { s.finish(err) })
	}
	return err
}

func terminalError(err error) error {
	if err == io.EOF {
		return nil
	}
	return err
}

func logRPC(log *slog.Logger, ctx context.Context, action, direction string, method rpcMethod, duration time.Duration, err error) {
	code := status.Code(err)
	outcome := "success"
	level := slog.LevelInfo
	if code != codes.OK {
		outcome = "failure"
		level = slog.LevelError
	}
	var attrs [11]slog.Attr
	attrs[0] = slog.String("event.action", action)
	attrs[1] = slog.String("event.category", "network")
	attrs[2] = slog.String("event.outcome", outcome)
	attrs[3] = slog.Int64("event.duration", duration.Nanoseconds())
	attrs[4] = slog.String("network.direction", direction)
	attrs[5] = slog.String("rpc.system", "grpc")
	attrs[6] = slog.String("rpc.service", method.service)
	attrs[7] = slog.String("rpc.method", method.method)
	attrs[8] = slog.Int("rpc.grpc.status_code", int(code))
	n := 9
	if err != nil {
		attrs[n] = slog.String("error.type", code.String())
		attrs[n+1] = slog.String("error.message", err.Error())
		n += 2
	}
	safeLog(log, ctx, level, "grpc call completed", attrs[:n])
}

type rpcMethod struct {
	service string
	method  string
}

func methodName(full string) rpcMethod {
	full = strings.Trim(full, "/")
	service, method, ok := strings.Cut(full, "/")
	if !ok {
		return rpcMethod{method: full}
	}
	return rpcMethod{service: service, method: method}
}

func fallbackLogger(log *slog.Logger) *slog.Logger {
	if log == nil {
		return slog.Default()
	}
	return log
}

func safeLog(log *slog.Logger, ctx context.Context, level slog.Level, message string, attrs []slog.Attr) {
	defer func() { _ = recover() }()
	log.LogAttrs(ctx, level, message, attrs...)
}
