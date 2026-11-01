package grpcx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	obskitlogger "github.com/budistwn15/go-obskit/v2/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestUnaryServerSchemaAndBehavior(t *testing.T) {
	var output bytes.Buffer
	interceptor := UnaryServerInterceptor(slog.New(slog.NewJSONHandler(&output, nil)))
	wantErr := status.Error(codes.NotFound, "missing")
	response, err := interceptor(context.Background(), "request", &grpc.UnaryServerInfo{FullMethod: "/billing.v1.Invoice/Get"}, func(context.Context, any) (any, error) {
		return "response", wantErr
	})
	if response != "response" || !errors.Is(err, wantErr) {
		t.Fatalf("response=%v error=%v", response, err)
	}
	event := decodeEvent(t, output.Bytes())
	assertRPCEvent(t, event, "billing.v1.Invoice", "Get", int(codes.NotFound), "failure")
}

func TestUnaryClientPreservesResult(t *testing.T) {
	var output bytes.Buffer
	interceptor := UnaryClientInterceptor(slog.New(slog.NewJSONHandler(&output, nil)))
	if err := interceptor(context.Background(), "/billing.v1.Invoice/Create", nil, nil, nil, func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	event := decodeEvent(t, output.Bytes())
	assertRPCEvent(t, event, "billing.v1.Invoice", "Create", int(codes.OK), "success")
}

func TestUnaryServerKeepsTraceCorrelationWithoutPayload(t *testing.T) {
	var output bytes.Buffer
	log := obskitlogger.New(obskitlogger.Config{Output: &output})
	ctx := obskitlogger.WithMeta(context.Background(), obskitlogger.ContextMeta{TraceID: "trace-1", SpanID: "span-1"})
	interceptor := UnaryServerInterceptor(log)
	_, _ = interceptor(ctx, map[string]string{"secret": "must-not-log"}, &grpc.UnaryServerInfo{FullMethod: "/billing.v1.Invoice/Get"}, func(context.Context, any) (any, error) {
		return nil, nil
	})
	event := decodeEvent(t, output.Bytes())
	if event["trace.id"] != "trace-1" || event["span.id"] != "span-1" {
		t.Fatalf("trace fields=%v", event)
	}
	if strings.Contains(output.String(), "must-not-log") {
		t.Fatal("gRPC payload must not be logged")
	}
}

func BenchmarkUnaryServerInterceptor(b *testing.B) {
	interceptor := UnaryServerInterceptor(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	info := &grpc.UnaryServerInfo{FullMethod: "/billing.v1.Invoice/Get"}
	handler := func(context.Context, any) (any, error) { return nil, nil }
	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		_, _ = interceptor(ctx, nil, info, handler)
	}
}

func BenchmarkStreamServerInterceptor(b *testing.B) {
	interceptor := StreamServerInterceptor(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	info := &grpc.StreamServerInfo{FullMethod: "/billing.v1.Invoice/Watch"}
	handler := func(any, grpc.ServerStream) error { return nil }
	stream := fakeServerStream{ctx: context.Background()}
	b.ResetTimer()
	for b.Loop() {
		_ = interceptor(nil, stream, info, handler)
	}
}

func TestStreamInterceptorsLogTerminalStatus(t *testing.T) {
	var serverOutput bytes.Buffer
	server := StreamServerInterceptor(slog.New(slog.NewJSONHandler(&serverOutput, nil)))
	wantErr := status.Error(codes.ResourceExhausted, "full")
	if err := server(nil, fakeServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: "/billing.v1.Invoice/Watch"}, func(any, grpc.ServerStream) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("server error=%v", err)
	}
	assertRPCEvent(t, decodeEvent(t, serverOutput.Bytes()), "billing.v1.Invoice", "Watch", int(codes.ResourceExhausted), "failure")

	var clientOutput bytes.Buffer
	client := StreamClientInterceptor(slog.New(slog.NewJSONHandler(&clientOutput, nil)))
	stream, err := client(context.Background(), &grpc.StreamDesc{}, nil, "/billing.v1.Invoice/Watch", func(context.Context, *grpc.StreamDesc, *grpc.ClientConn, string, ...grpc.CallOption) (grpc.ClientStream, error) {
		return fakeClientStream{ctx: context.Background()}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := stream.RecvMsg(nil); err != io.EOF {
		t.Fatalf("recv error=%v", err)
	}
	assertRPCEvent(t, decodeEvent(t, clientOutput.Bytes()), "billing.v1.Invoice", "Watch", int(codes.OK), "success")
}

func decodeEvent(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var event map[string]any
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func assertRPCEvent(t *testing.T, event map[string]any, service, method string, code int, outcome string) {
	t.Helper()
	if event["rpc.system"] != "grpc" || event["rpc.service"] != service || event["rpc.method"] != method {
		t.Fatalf("rpc fields=%v", event)
	}
	if int(event["rpc.grpc.status_code"].(float64)) != code || event["event.outcome"] != outcome {
		t.Fatalf("status fields=%v", event)
	}
	if _, ok := event["event.duration"].(float64); !ok {
		t.Fatalf("event.duration type=%T", event["event.duration"])
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s fakeServerStream) Context() context.Context { return s.ctx }

type fakeClientStream struct{ ctx context.Context }

func (s fakeClientStream) Header() (metadata.MD, error) { return nil, nil }
func (s fakeClientStream) Trailer() metadata.MD         { return nil }
func (s fakeClientStream) CloseSend() error             { return nil }
func (s fakeClientStream) Context() context.Context     { return s.ctx }
func (s fakeClientStream) SendMsg(any) error            { return nil }
func (s fakeClientStream) RecvMsg(any) error            { return io.EOF }
