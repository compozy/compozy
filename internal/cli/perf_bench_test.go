package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

var (
	benchmarkTableHeaders = []string{"Name", "Provider", "Model", "Tools", "Permissions"}
	benchmarkTableRows    = buildBenchmarkTableRows(512)
	benchmarkSSEPayload   = buildBenchmarkSSEPayload(1024)
	benchmarkRequestBody  = map[string]any{
		"agent_name": "coder",
		"workspace":  "ws-benchmark",
		"prompt":     strings.Repeat("benchmark payload ", 8),
	}
)

func BenchmarkRenderHumanTableLarge(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = renderHumanTable("Agents", benchmarkTableHeaders, benchmarkTableRows)
	}
}

func BenchmarkRenderToonArrayLarge(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = renderToonArray("agents", []string{"name", "provider", "model", "tools", "permissions"}, benchmarkTableRows)
	}
}

func BenchmarkDecodeSSELargeStream(b *testing.B) {
	ctx := context.Background()
	handler := func(_ SSEEvent) error { return nil }

	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkSSEPayload)))
	for b.Loop() {
		if err := decodeSSE(ctx, io.NopCloser(strings.NewReader(benchmarkSSEPayload)), handler); err != nil {
			b.Fatalf("decodeSSE() error = %v", err)
		}
	}
}

func BenchmarkDoRequestPostJSON(b *testing.B) {
	ctx := context.Background()
	client := &unixSocketClient{
		socketPath: "/tmp/agh.sock",
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				if req.Body != nil {
					if _, err := io.Copy(io.Discard, req.Body); err != nil {
						return nil, err
					}
					if err := req.Body.Close(); err != nil {
						return nil, err
					}
				}
				return newHTTPResponse(http.StatusNoContent, ""), nil
			}),
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		response, err := client.doRequest(ctx, http.MethodPost, "/api/sessions", nil, benchmarkRequestBody)
		if err != nil {
			b.Fatalf("doRequest() error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			b.Fatalf("response.Body.Close() error = %v", err)
		}
	}
}

func buildBenchmarkTableRows(count int) [][]string {
	rows := make([][]string, 0, count)
	for i := range count {
		rows = append(rows, []string{
			fmt.Sprintf("agent-%03d", i),
			"codex",
			"gpt-5.4",
			fmt.Sprintf("%d", 4+(i%3)),
			"approve-reads",
		})
	}
	return rows
}

func buildBenchmarkSSEPayload(events int) string {
	var builder strings.Builder
	builder.Grow(events * 96)

	for i := range events {
		builder.WriteString("id: ")
		fmt.Fprintf(&builder, "%d", i)
		builder.WriteString("\n")
		builder.WriteString("event: agent_message\n")
		builder.WriteString(`data: {"id":"evt-`)
		fmt.Fprintf(&builder, "%d", i)
		builder.WriteString(`","type":"agent_message","text":"hello benchmark"}`)
		builder.WriteString("\n\n")
	}

	return builder.String()
}
