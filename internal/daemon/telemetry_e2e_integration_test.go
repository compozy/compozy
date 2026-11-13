//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

const e2eOTLPKey = "e2e-otlp-key-value-7f3a"

// E2E-002
func TestDaemonE2EOTelExportReportsDegradedMetrics(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should report rejected metrics as degraded while turns and traces stay healthy", func(t *testing.T) {
		t.Parallel()

		receiver := newOTLPReceiver(t)
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			Env: map[string]string{"TEST_OTLP_KEY": e2eOTLPKey},
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  mockFixturePath(t, "multi_agent_fixture.json"),
				FixtureAgent: "alpha",
				AgentName:    "mock-alpha",
			}},
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Observability.OTel.Enabled = true
				cfg.Observability.OTel.Endpoint = receiver.URL
				cfg.Observability.OTel.MetricInterval = 5 * time.Second
				cfg.Observability.OTel.Headers = map[string]string{"api-key": "env:TEST_OTLP_KEY"}
			}},
		})
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		defer cancel()

		session := createFixtureBackedSession(t, ctx, harness, "mock-alpha", "otel-session")
		stream, err := harness.PromptSession(ctx, session.ID, "hello alpha")
		if err != nil || len(stream) == 0 {
			t.Fatalf("PromptSession() = %d updates, %v; want a completed turn", len(stream), err)
		}

		otel := waitForOTelStatus(ctx, t, harness, func(otel *compozycontract.OTelStatusPayload) bool {
			return otel.Metrics.LastErrorClass == "http_401" && !otel.Traces.LastExportAt.IsZero()
		})
		if otel.State != "degraded" || otel.Metrics.FailedExports == 0 || !otel.Traces.Enabled ||
			otel.Traces.LastErrorClass != "" {
			t.Fatalf("status otel = %+v, want degraded metrics and healthy traces", otel)
		}
		if keys := receiver.headerValues("/v1/traces"); len(keys) == 0 || keys[0] != e2eOTLPKey {
			t.Fatalf("receiver trace api-key headers = %v, want a successful authenticated trace export", keys)
		}
		if keys := receiver.headerValues("/v1/metrics"); len(keys) == 0 || keys[0] != e2eOTLPKey {
			t.Fatalf("receiver api-key headers = %v, want the env:TEST_OTLP_KEY value", keys)
		}
		logs, err := os.ReadFile(harness.HomePaths.LogFile)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", harness.HomePaths.LogFile, err)
		}
		if strings.Contains(string(logs), e2eOTLPKey) {
			t.Fatal("daemon log contains the resolved header value")
		}
	})
}

// otlpReceiver accepts OTLP trace exports, rejects metric exports with 401, and records the api-key header.
type otlpReceiver struct {
	*httptest.Server
	mu   sync.Mutex
	keys map[string][]string
}

func newOTLPReceiver(t *testing.T) *otlpReceiver {
	t.Helper()
	receiver := &otlpReceiver{keys: make(map[string][]string)}
	receiver.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, err := io.Copy(io.Discard, req.Body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receiver.mu.Lock()
		receiver.keys[req.URL.Path] = append(receiver.keys[req.URL.Path], req.Header.Get("api-key"))
		receiver.mu.Unlock()
		if req.URL.Path == "/v1/metrics" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(receiver.Close)
	return receiver
}

func (r *otlpReceiver) headerValues(path string) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.keys[path]...)
}

// waitForOTelStatus polls compozy status --json until ready reports true for its otel section.
func waitForOTelStatus(
	ctx context.Context,
	t *testing.T,
	harness *e2etest.RuntimeHarness,
	ready func(*compozycontract.OTelStatusPayload) bool,
) *compozycontract.OTelStatusPayload {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var last *compozycontract.OTelStatusPayload
	for {
		stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, "status", "-o", "json")
		if err != nil {
			t.Fatalf("compozy status -o json error = %v; stderr=%s", err, stderr)
		}
		var status compozycontract.StatusPayload
		if err := json.Unmarshal([]byte(stdout), &status); err != nil {
			t.Fatalf("decode compozy status error = %v; stdout=%s", err, stdout)
		}
		if last = status.OTel; last != nil && ready(last) {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("status otel = %+v, still not ready when the wait ended: %v", last, ctx.Err())
		case <-ticker.C:
		}
	}
}
