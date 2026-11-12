package ginutil

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"github.com/compozy/compozy/internal/version"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

const probeRoute = "/api/probe/:id"

// IT-013
func TestRequestTracingHonorsTraceParentOverHTTPAndUDS(t *testing.T) {
	// Not parallel: each case installs the global OTel providers.
	cases := []struct {
		name        string
		export      bool
		ratio       float64
		flags       string
		wantSampled bool
	}{
		{
			name: "Should join an unsampled client trace when the ratio keeps it", export: true, ratio: 1,
			flags: "00", wantSampled: true,
		},
		{name: "Should drop an unsampled client trace at ratio zero", export: true, ratio: 0, flags: "00"},
		{
			name: "Should keep a sampled external parent at ratio zero", export: true, ratio: 0, flags: "01",
			wantSampled: true,
		},
		{name: "Should serve the request and record nothing while export is disabled", flags: "01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector := otlptest.NewCollector(t)
			var disabled telemetry.Runtime
			if tc.export {
				collector.Export(t, tc.ratio, telemetry.Registries{RouteTemplates: []string{probeRoute}})
			} else {
				disabled = initDisabledExport(t, collector.URL)
			}
			transports := probeTransports(t, probeEngine())
			for transport, client := range transports {
				parent := testParent(t)
				status := getProbe(
					t,
					client,
					"/api/probe/1",
					parent.TraceID().String()+"-"+parent.SpanID().String(),
					tc.flags,
				)
				if status != http.StatusOK {
					t.Fatalf("%s GET status = %d, want %d", transport, status, http.StatusOK)
				}
				if !tc.export {
					continue
				}
				spans := traceSpans(collector.Flush(t), parent.TraceID())
				if !tc.wantSampled {
					if len(spans) != 0 {
						t.Fatalf("%s exported %d spans for an unsampled trace, want none", transport, len(spans))
					}
					continue
				}
				if len(spans) != 1 {
					t.Fatalf("%s exported %d spans for the client trace, want 1", transport, len(spans))
				}
				assertServerSpan(t, transport, spans[0], parent)
			}
			if !tc.export {
				if err := disabled.Shutdown(t.Context()); err != nil {
					t.Fatalf("telemetry Runtime.Shutdown() error = %v", err)
				}
				if got := len(collector.Payloads(otlptest.TracesPath)); got != 0 {
					t.Fatalf("collector received %d trace requests while export was disabled, want none", got)
				}
			}
		})
	}

	t.Run("Should name a request without a matching route HTTP <METHOD>", func(t *testing.T) {
		collector := otlptest.NewCollector(t)
		collector.Export(t, 1, telemetry.Registries{RouteTemplates: []string{probeRoute}})
		for transport, client := range probeTransports(t, probeEngine()) {
			parent := testParent(t)
			status := getProbe(t, client, "/api/missing", parent.TraceID().String()+"-"+parent.SpanID().String(), "01")
			if status != http.StatusNotFound {
				t.Fatalf("%s GET status = %d, want %d", transport, status, http.StatusNotFound)
			}
			spans := traceSpans(collector.Flush(t), parent.TraceID())
			if len(spans) != 1 || spans[0].Name != "HTTP GET" {
				t.Fatalf("%s spans = %+v, want one span named HTTP GET", transport, spans)
			}
		}
	})
}

// initDisabledExport installs the telemetry runtime with export switched off but pointed at endpoint, so a
// regression that exports anyway reaches the collector.
func initDisabledExport(t *testing.T, endpoint string) telemetry.Runtime {
	t.Helper()
	cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{}).Observability
	cfg.Enabled = true
	cfg.OTel.Enabled = false
	cfg.OTel.Endpoint = endpoint
	cfg.OTel.Protocol = compozyconfig.OTelProtocolHTTPProtobuf
	cfg.OTel.SampleRatio = 1
	runtime, err := telemetry.Init(
		t.Context(),
		cfg,
		nil,
		telemetry.Registries{RouteTemplates: []string{probeRoute}},
		version.Info{Version: "test"},
		slog.New(slog.DiscardHandler),
	)
	if err != nil {
		t.Fatalf("telemetry.Init() error = %v", err)
	}
	return runtime
}

func probeEngine() *gin.Engine {
	engine := NewEngine()
	QuietDebug(func() { engine.GET(probeRoute, func(c *gin.Context) { c.Status(http.StatusOK) }) })
	return engine
}

// probeTransports serves engine over loopback TCP and a Unix socket, the HTTP and UDS listeners of the daemon.
func probeTransports(t *testing.T, engine *gin.Engine) map[string]*http.Client {
	t.Helper()
	var listenConfig net.ListenConfig
	tcp, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(tcp) error = %v", err)
	}
	socket := filepath.Join(shortTempDir(t), "probe.sock")
	uds, err := listenConfig.Listen(t.Context(), "unix", socket)
	if err != nil {
		t.Fatalf("net.Listen(unix) error = %v", err)
	}
	for _, listener := range []net.Listener{tcp, uds} {
		server := &http.Server{Handler: engine, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				t.Errorf("Serve(%s) error = %v", listener.Addr(), err)
			}
		}()
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Errorf("Server.Close() error = %v", err)
			}
		})
	}
	dial := func(network, address string) func(context.Context, string, string) (net.Conn, error) {
		return func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, address)
		}
	}
	return map[string]*http.Client{
		"HTTP": {Transport: &http.Transport{DialContext: dial("tcp", tcp.Addr().String())}, Timeout: 10 * time.Second},
		"UDS":  {Transport: &http.Transport{DialContext: dial("unix", socket)}, Timeout: 10 * time.Second},
	}
}

// shortTempDir keeps the socket path under the platform's Unix socket length limit.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	return dir
}

// testParent is a client span context with a random trace ID.
func testParent(t *testing.T) trace.SpanContext {
	t.Helper()
	var traceID trace.TraceID
	if _, err := rand.Read(traceID[:]); err != nil {
		t.Fatalf("rand.Read() error = %v", err)
	}
	return trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}})
}

func getProbe(t *testing.T, client *http.Client, path, ids, flags string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://compozy"+path, http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("traceparent", "00-"+ids+"-"+flags)
	req.Header.Set("tracestate", "vendor=ignored")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s error = %v", path, err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body error = %v", err)
	}
	return resp.StatusCode
}

func traceSpans(spans []otlptest.Span, traceID trace.TraceID) []otlptest.Span {
	matched := make([]otlptest.Span, 0, len(spans))
	for _, span := range spans {
		if span.TraceID == traceID {
			matched = append(matched, span)
		}
	}
	return matched
}

func assertServerSpan(t *testing.T, transport string, span otlptest.Span, parent trace.SpanContext) {
	t.Helper()
	if span.Name != "GET "+probeRoute || span.ParentSpanID != parent.SpanID() || span.Scope != telemetry.ScopeHTTP {
		t.Fatalf("%s span = %q parent %s scope %q, want GET %s under the client span %s from %s",
			transport, span.Name, span.ParentSpanID, span.Scope, probeRoute, parent.SpanID(), telemetry.ScopeHTTP)
	}
	if span.Attributes["http.route"] != probeRoute || span.Attributes["http.response.status_code"] != int64(200) {
		t.Fatalf("%s span attributes = %v, want the route template and status 200", transport, span.Attributes)
	}
}
