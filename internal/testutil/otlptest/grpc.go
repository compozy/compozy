package otlptest

import (
	"context"
	"errors"
	"net"
	"testing"

	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// NewGRPCCollector starts an OTLP/gRPC collector that stops at test cleanup. Payloads holds each request's raw
// message bytes as they arrived on the wire, before decoding.
func NewGRPCCollector(t testing.TB) *Collector {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for the OTLP/gRPC collector: %v", err)
	}
	collector := &Collector{URL: "http://" + listener.Addr().String()}
	server := grpc.NewServer(grpc.ForceServerCodec(recordingCodec{collector: collector}))
	coltracepb.RegisterTraceServiceServer(server, grpcTraces{collector: collector})
	colmetricpb.RegisterMetricsServiceServer(server, grpcMetrics{collector: collector})
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		if err := <-served; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("OTLP/gRPC collector Serve() error = %v", err)
		}
	})
	return collector
}

// recordingCodec is the proto codec that also records every received export request's raw bytes.
type recordingCodec struct {
	collector *Collector
}

func (recordingCodec) Name() string { return "proto" }

func (recordingCodec) Marshal(v any) ([]byte, error) {
	message, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("otlptest: gRPC codec marshals only proto messages")
	}
	return proto.Marshal(message)
}

func (c recordingCodec) Unmarshal(data []byte, v any) error {
	message, ok := v.(proto.Message)
	if !ok {
		return errors.New("otlptest: gRPC codec unmarshals only proto messages")
	}
	switch v.(type) {
	case *coltracepb.ExportTraceServiceRequest:
		c.collector.record(TracesPath, data)
	case *colmetricpb.ExportMetricsServiceRequest:
		c.collector.record(MetricsPath, data)
	}
	return proto.Unmarshal(data, message)
}

type grpcTraces struct {
	coltracepb.UnimplementedTraceServiceServer
	collector *Collector
}

func (s grpcTraces) Export(
	_ context.Context,
	export *coltracepb.ExportTraceServiceRequest,
) (*coltracepb.ExportTraceServiceResponse, error) {
	s.collector.keep(export)
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type grpcMetrics struct {
	colmetricpb.UnimplementedMetricsServiceServer
	collector *Collector
}

func (s grpcMetrics) Export(
	_ context.Context,
	export *colmetricpb.ExportMetricsServiceRequest,
) (*colmetricpb.ExportMetricsServiceResponse, error) {
	s.collector.keepMetrics(export)
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}
