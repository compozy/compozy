package telemetry_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// UT-010 (the global tracer half is in runtime_env_test.go), UT-017, UT-018
func TestInitInstallsNoopProvidersWhenExportCannotRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  config.ObservabilityConfig
		want telemetry.State
	}{
		{
			name: "Should stay disabled by default and open no connection",
			cfg:  config.DefaultWithHome(config.HomePaths{}).Observability,
			want: telemetry.StateDisabled,
		},
		{
			name: "Should report inactive when local observability is disabled",
			cfg: func() config.ObservabilityConfig {
				cfg := exportConfig(nil)
				cfg.Enabled = false
				return cfg
			}(),
			want: telemetry.StateInactiveObservabilityDisabled,
		},
		{
			name: "Should stay disabled when OTEL_SDK_DISABLED is true",
			cfg: environmentConfig(
				map[string]string{"OTEL_SDK_DISABLED": "true"},
				func(otel *config.ObservabilityOTelConfig) {
					otel.Enabled = true
				},
			),
			want: telemetry.StateDisabled,
		},
		{
			name: "Should stay disabled when traces and metrics are both off",
			cfg: exportConfig(func(otel *config.ObservabilityOTelConfig) {
				otel.Traces = false
				otel.Metrics = false
			}),
			want: telemetry.StateDisabled,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			started := startRuntime(t, tt.cfg, nil, &fakeNetwork{})
			assertNoopProviders(t, started)
			_, span := started.tracerProvider.Tracer(telemetry.ScopeSession).
				Start(t.Context(), telemetry.SpanSessionTurn)
			span.End()
			if err := started.runtime.Shutdown(t.Context()); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}

			status := started.runtime.Status()
			if status.State != tt.want || status.Traces.Enabled || status.Metrics.Enabled {
				t.Fatalf("Status() = %+v, want state %q with both signals off", status, tt.want)
			}
			requests, dials := started.network.captured()
			if len(requests) != 0 || len(dials) != 0 {
				t.Fatalf("network saw %d requests and %d dials, want none", len(requests), len(dials))
			}
		})
	}
}

func assertNoopProviders(t *testing.T, started *startedRuntime) {
	t.Helper()
	if _, ok := started.tracerProvider.(tracenoop.TracerProvider); !ok {
		t.Fatalf("installed tracer provider = %T, want no-op", started.tracerProvider)
	}
	if _, ok := started.meterProvider.(metricnoop.MeterProvider); !ok {
		t.Fatalf("installed meter provider = %T, want no-op", started.meterProvider)
	}
}

// UT-011, UT-012
func TestInitTargetsTheResolvedEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("Should export over HTTP to localhost:4318 without an endpoint", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(nil), nil, &fakeNetwork{})
		started.exportSpan(t)
		started.exportMetrics(t.Context(), t)

		requests, _ := started.network.captured()
		assertRequestURLs(t, requests, "http://localhost:4318/v1/traces", "http://localhost:4318/v1/metrics")
		status := started.runtime.Status()
		if status.Protocol != config.OTelProtocolHTTPProtobuf || status.EndpointHost != "localhost:4318" {
			t.Fatalf(
				"Status() target = %q → %q, want http/protobuf → localhost:4318",
				status.Protocol,
				status.EndpointHost,
			)
		}
	})

	t.Run("Should export over gRPC to localhost:4317 without an endpoint", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Protocol = config.OTelProtocolGRPC
			otel.Metrics = false
		}), nil, &fakeNetwork{})
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		started.attemptSpanExport(ctx, t)

		requests, dials := started.network.captured()
		if len(requests) != 0 || len(dials) == 0 || !strings.HasSuffix(dials[0], ":4317") {
			t.Fatalf(
				"gRPC export used %d HTTP requests and dials %v, want gRPC dials to port 4317",
				len(requests),
				dials,
			)
		}
		status := started.runtime.Status()
		if status.Protocol != config.OTelProtocolGRPC || status.EndpointHost != "localhost:4317" {
			t.Fatalf("Status() target = %q → %q, want grpc → localhost:4317", status.Protocol, status.EndpointHost)
		}
	})

	t.Run("Should export to OTEL_EXPORTER_OTLP_ENDPOINT when config.toml sets no endpoint", func(t *testing.T) {
		t.Parallel()

		cfg := environmentConfig(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://env:4318"},
			func(otel *config.ObservabilityOTelConfig) { otel.Enabled = true })
		started := startRuntime(t, cfg, nil, &fakeNetwork{})
		started.exportSpan(t)

		requests, _ := started.network.captured()
		assertRequestURLs(t, requests, "http://env:4318/v1/traces")
	})
}

func assertRequestURLs(t *testing.T, requests []capturedRequest, want ...string) {
	t.Helper()
	seen := make(map[string]bool, len(requests))
	for _, request := range requests {
		seen[request.url] = true
	}
	for _, url := range want {
		if !seen[url] {
			t.Fatalf("exporter requests = %v, want a request to %s", requestURLs(requests), url)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("exporter requests = %v, want only %v", requestURLs(requests), want)
	}
}

func requestURLs(requests []capturedRequest) []string {
	urls := make([]string, 0, len(requests))
	for _, request := range requests {
		urls = append(urls, request.url)
	}
	return urls
}

// UT-014
func TestInitSendsResolvedHeaders(t *testing.T) {
	t.Parallel()

	t.Run("Should send the resolved header value and never expose it", func(t *testing.T) {
		t.Parallel()

		const secret = "k123"
		cfg := exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Headers = map[string]string{"api-key": "vault:observability/nr"}
		})
		started := startRuntime(t, cfg, fakeResolver{"vault:observability/nr": secret}, &fakeNetwork{})
		started.exportSpan(t)
		started.exportMetrics(t.Context(), t)
		if err := started.runtime.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}

		requests, _ := started.network.captured()
		if len(requests) < 2 {
			t.Fatalf("exporter sent %d requests, want trace and metric exports", len(requests))
		}
		for _, request := range requests {
			if got := request.header.Get("api-key"); got != secret {
				t.Fatalf("request %s header api-key = %q, want the resolved value", request.url, got)
			}
			if strings.Contains(string(request.body), secret) {
				t.Fatalf("request %s payload contains the header value", request.url)
			}
		}
		if status := fmt.Sprintf("%+v", started.runtime.Status()); strings.Contains(status, secret) {
			t.Fatalf("Status() = %s, want no header value", status)
		}
		if logs := started.logs.String(); strings.Contains(logs, secret) {
			t.Fatalf("logs = %s, want no header value", logs)
		}
	})

	t.Run("Should send OTEL_EXPORTER_OTLP_HEADERS when config.toml sets no headers", func(t *testing.T) {
		t.Parallel()

		cfg := environmentConfig(map[string]string{"OTEL_EXPORTER_OTLP_HEADERS": "x-team=payments"},
			func(otel *config.ObservabilityOTelConfig) { otel.Enabled = true })
		started := startRuntime(t, cfg, nil, &fakeNetwork{})
		started.exportSpan(t)

		requests, _ := started.network.captured()
		if len(requests) != 1 || requests[0].header.Get("x-team") != "payments" {
			t.Fatalf("exporter requests = %d, want one carrying the environment header", len(requests))
		}
	})

	// US-001.AC-6 holds for either protocol: over gRPC the headers travel as call metadata.
	t.Run("Should send the resolved header value as gRPC metadata on both signals", func(t *testing.T) {
		t.Parallel()

		const secret = "k123"
		receiver := newMetadataReceiver(t)
		cfg := exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Protocol = config.OTelProtocolGRPC
			otel.Endpoint = receiver.endpoint
			otel.Headers = map[string]string{"api-key": "vault:observability/nr"}
		})
		started := startRuntime(t, cfg, fakeResolver{"vault:observability/nr": secret},
			&fakeNetwork{connect: loopbackDial})
		started.exportSpan(t)
		started.exportMetrics(t.Context(), t)

		for signal, method := range map[string]string{
			"traces":  "/opentelemetry.proto.collector.trace.v1.TraceService/Export",
			"metrics": "/opentelemetry.proto.collector.metrics.v1.MetricsService/Export",
		} {
			if got := receiver.metadata(method).Get("api-key"); len(got) != 1 || got[0] != secret {
				t.Fatalf("%s export metadata api-key = %v, want the resolved value", signal, got)
			}
		}
	})

	// Business Rule 5: the replacement lives in Init, not in the config layer, which exposes both header sets.
	t.Run("Should replace OTEL_EXPORTER_OTLP_HEADERS entirely when config.toml sets headers", func(t *testing.T) {
		t.Parallel()

		cfg := environmentConfig(map[string]string{"OTEL_EXPORTER_OTLP_HEADERS": "x-team=payments,api-key=env-value"},
			func(otel *config.ObservabilityOTelConfig) {
				otel.Enabled = true
				otel.Headers = map[string]string{"api-key": "vault:observability/nr"}
			})
		started := startRuntime(t, cfg, fakeResolver{"vault:observability/nr": "file-value"}, &fakeNetwork{})
		started.exportSpan(t)

		requests, _ := started.network.captured()
		if len(requests) != 1 {
			t.Fatalf("exporter sent %d requests, want one trace export", len(requests))
		}
		if header := requests[0].header; header.Get("api-key") != "file-value" || header.Get("x-team") != "" {
			t.Fatalf("request headers = %v, want only the config.toml api-key and no environment header", header)
		}
	})

	// The assertions above only prove a span without attributes carries no header value. This one puts the value
	// where the sanitizer must catch it: a registered attribute value that contains it.
	for _, secret := range []string{
		"otlp-header-secret-3d9f1c",
		// Shorter than the eight bytes heuristic discovery requires: header values are classified secrets.
		"q7zx91",
	} {
		t.Run(
			"Should export a header value that reaches an attribute as invalid and forget it at shutdown: "+secret,
			func(t *testing.T) {
				t.Parallel()

				cfg := exportConfig(func(otel *config.ObservabilityOTelConfig) {
					otel.Headers = map[string]string{"api-key": "vault:observability/nr"}
				})
				started := startRuntimeWithRegistries(
					t,
					cfg,
					fakeResolver{"vault:observability/nr": secret},
					&fakeNetwork{},
					testRegistries([]string{secret}),
				)
				started.exportSpanWith(t, trace.WithAttributes(attribute.String(string(telemetry.AttrAgent), secret)))

				requests, _ := started.network.captured()
				if len(requests) != 1 {
					t.Fatalf("exporter sent %d requests, want one trace export", len(requests))
				}
				if body := requests[0].body; bytes.Contains(body, []byte(secret)) ||
					!bytes.Contains(body, []byte(telemetry.ValueInvalid)) {
					t.Fatalf("trace export = %q, want the header value exported as %q", body, telemetry.ValueInvalid)
				}

				if err := started.runtime.Shutdown(t.Context()); err != nil {
					t.Fatalf("Shutdown() error = %v", err)
				}
				if redact.ContainsRegisteredSecret(secret) {
					t.Fatal("header value is still registered for redaction after Shutdown")
				}
			},
		)
	}
}

// UT-015
func TestInitDegradesWhenExportCannotStart(t *testing.T) {
	t.Parallel()

	missingRef := func(otel *config.ObservabilityOTelConfig) {
		otel.Headers = map[string]string{"api-key": "vault:observability/missing"}
	}
	tests := []struct {
		name  string
		cfg   config.ObservabilityConfig
		refs  telemetry.SecretResolver
		class string
		log   string
	}{
		{
			name:  "Should boot with no-op providers and report header_ref_unresolved on both signals",
			cfg:   exportConfig(missingRef),
			refs:  fakeResolver{},
			class: "header_ref_unresolved",
			log:   "vault:observability/missing",
		},
		{
			name:  "Should report header_ref_unresolved when no secret resolver is available",
			cfg:   exportConfig(missingRef),
			class: "header_ref_unresolved",
			log:   "vault:observability/missing",
		},
		{
			name:  "Should boot with no-op providers when the endpoint cannot be used",
			cfg:   exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Endpoint = "http://[::1" }),
			class: "other",
			log:   "telemetry: exporter setup failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			started := startRuntime(t, tt.cfg, tt.refs, &fakeNetwork{})

			assertNoopProviders(t, started)
			status := started.runtime.Status()
			if status.State != telemetry.StateDegraded {
				t.Fatalf("Status().State = %q, want degraded", status.State)
			}
			for name, signal := range map[string]telemetry.SignalStatus{"traces": status.Traces, "metrics": status.Metrics} {
				if !signal.Enabled || signal.LastErrorClass != tt.class {
					t.Fatalf("Status().%s = %+v, want enabled with %s", name, signal, tt.class)
				}
			}
			if logs := started.logs.String(); !strings.Contains(logs, tt.log) {
				t.Fatalf("logs = %s, want %q", logs, tt.log)
			}
			requests, dials := started.network.captured()
			if len(requests) != 0 || len(dials) != 0 {
				t.Fatalf("network saw %d requests and %d dials, want none", len(requests), len(dials))
			}
		})
	}
}

// UT-021
func TestInitHonorsSignalSwitches(t *testing.T) {
	t.Parallel()

	t.Run("Should install a no-op meter and report metrics disabled", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Metrics = false }),
			nil, &fakeNetwork{})
		if _, ok := started.meterProvider.(metricnoop.MeterProvider); !ok {
			t.Fatalf("installed meter provider = %T, want no-op", started.meterProvider)
		}
		started.exportSpan(t)

		status := started.runtime.Status()
		if status.State != telemetry.StateOK || !status.Traces.Enabled || status.Metrics.Enabled {
			t.Fatalf("Status() = %+v, want ok with traces on and metrics off", status)
		}
		requests, _ := started.network.captured()
		assertRequestURLs(t, requests, "http://localhost:4318/v1/traces")
	})

	t.Run("Should install a no-op tracer and report traces disabled", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) { otel.Traces = false }),
			nil, &fakeNetwork{})
		if _, ok := started.tracerProvider.(tracenoop.TracerProvider); !ok {
			t.Fatalf("installed tracer provider = %T, want no-op", started.tracerProvider)
		}
		started.exportMetrics(t.Context(), t)

		status := started.runtime.Status()
		if status.State != telemetry.StateOK || status.Traces.Enabled || !status.Metrics.Enabled {
			t.Fatalf("Status() = %+v, want ok with traces off and metrics on", status)
		}
	})
}

// UT-027
func TestInitReportsOnlyTheEndpointHost(t *testing.T) {
	t.Parallel()

	t.Run("Should drop userinfo, path, and query from the endpoint host", func(t *testing.T) {
		t.Parallel()

		started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) {
			otel.Endpoint = "https://user:pw@host:4318/v1?token=x"
		}), nil, &fakeNetwork{})

		if host := started.runtime.Status().EndpointHost; host != "host:4318" {
			t.Fatalf("Status().EndpointHost = %q, want host:4318", host)
		}
	})
}

// US-010.AC-1, US-008.AC-3: status reports the sample ratio and provider forwarding the configuration chose.
func TestInitReportsTheConfiguredPolicyInStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ratio      float64
		forwarding bool
	}{
		{name: "Should report a fractional sample ratio with provider forwarding on", ratio: 0.25, forwarding: true},
		{name: "Should report provider forwarding off", ratio: 0.5, forwarding: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			started := startRuntime(t, exportConfig(func(otel *config.ObservabilityOTelConfig) {
				otel.SampleRatio = tt.ratio
				otel.ForwardToProviders = tt.forwarding
			}), nil, &fakeNetwork{})

			status := started.runtime.Status()
			if status.SampleRatio != tt.ratio || status.ForwardToProviders != tt.forwarding {
				t.Fatalf("Status() = %+v, want sample ratio %v with provider forwarding %t",
					status, tt.ratio, tt.forwarding)
			}
		})
	}
}

// UT-071, UT-073: the forwarded variables carry the target and header resolved at Init, and only while export runs.
func TestProviderEnvForwardsOnlyAStartedExport(t *testing.T) {
	t.Parallel()

	forwarding := func(mutate func(*config.ObservabilityOTelConfig)) func(*config.ObservabilityOTelConfig) {
		return func(otel *config.ObservabilityOTelConfig) {
			otel.ForwardToProviders = true
			otel.Endpoint = "http://collector.local:4318"
			otel.Headers = map[string]string{"api-key": "vault:observability/nr"}
			if mutate != nil {
				mutate(otel)
			}
		}
	}
	inactive := exportConfig(forwarding(nil))
	inactive.Enabled = false
	tests := []struct {
		name string
		cfg  config.ObservabilityConfig
		refs telemetry.SecretResolver
		want []string
	}{
		{
			name: "Should forward the endpoint, protocol, and resolved header value",
			cfg:  exportConfig(forwarding(nil)),
			refs: fakeResolver{"vault:observability/nr": "k 1/2,x=y"},
			want: []string{
				"OTEL_EXPORTER_OTLP_ENDPOINT=http://collector.local:4318",
				"OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf",
				"OTEL_EXPORTER_OTLP_HEADERS=api-key=k%201%2F2%2Cx=y",
			},
		},
		{
			name: "Should forward nothing while forwarding is off",
			cfg: exportConfig(forwarding(func(otel *config.ObservabilityOTelConfig) {
				otel.ForwardToProviders = false
			})),
			refs: fakeResolver{"vault:observability/nr": "k1"},
		},
		{
			name: "Should forward nothing while export is disabled",
			cfg:  exportConfig(forwarding(func(otel *config.ObservabilityOTelConfig) { otel.Enabled = false })),
			refs: fakeResolver{"vault:observability/nr": "k1"},
		},
		{
			name: "Should forward nothing while observability is disabled",
			cfg:  inactive,
			refs: fakeResolver{"vault:observability/nr": "k1"},
		},
		{
			name: "Should forward nothing when export degraded at init",
			cfg:  exportConfig(forwarding(nil)),
			refs: fakeResolver{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			started := startRuntime(t, tt.cfg, tt.refs, &fakeNetwork{})
			if got := started.runtime.ProviderEnv(); !slices.Equal(got, tt.want) {
				t.Fatalf("ProviderEnv() = %q, want %q", got, tt.want)
			}
		})
	}
}

// UT-038
func TestSamplerHonorsSampledParentsAndTheRatioForTheRest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ratio   float64
		parent  trace.TraceFlags
		remote  bool
		sampled bool
	}{
		{name: "Should drop a new root at ratio 0", ratio: 0, sampled: false},
		{name: "Should drop an unsampled remote parent at ratio 0", ratio: 0, remote: true, sampled: false},
		{
			name: "Should keep a sampled remote parent at ratio 0", ratio: 0, remote: true,
			parent: trace.FlagsSampled, sampled: true,
		},
		{name: "Should keep an unsampled remote parent at ratio 1", ratio: 1, remote: true, sampled: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exported := tracetest.NewInMemoryExporter()
			provider := sdktrace.NewTracerProvider(
				sdktrace.WithSampler(telemetry.NewSampler(tt.ratio)),
				sdktrace.WithSyncer(exported),
			)
			t.Cleanup(func() {
				if err := provider.Shutdown(context.Background()); err != nil {
					t.Errorf("Shutdown() error = %v", err)
				}
			})
			ctx := t.Context()
			traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
			if err != nil {
				t.Fatalf("TraceIDFromHex() error = %v", err)
			}
			parent := trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     trace.SpanID{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
				TraceFlags: tt.parent,
				Remote:     true,
			})
			if tt.remote {
				ctx = trace.ContextWithRemoteSpanContext(ctx, parent)
			}
			_, span := provider.Tracer(telemetry.ScopeHTTP).Start(ctx, http.MethodGet+" "+testRoute)
			span.End()

			spans := exported.GetSpans()
			if got := len(spans) == 1; got != tt.sampled {
				t.Fatalf("exported %d spans, want sampled = %t", len(spans), tt.sampled)
			}
			if tt.sampled && tt.remote && spans[0].SpanContext.TraceID() != parent.TraceID() {
				t.Fatalf("span trace id = %s, want the parent's %s", spans[0].SpanContext.TraceID(), parent.TraceID())
			}
		})
	}
}

// TestModelLabelClassifiesWhenMetricsRecord installs the process-wide model classifier, so it does not run in parallel.
func TestModelLabelClassifiesWhenMetricsRecord(t *testing.T) {
	t.Run("Should label a model by the classification current when the metric records", func(t *testing.T) {
		var cataloged atomic.Bool
		reg := testRegistries(nil)
		reg.Models = func(_, model string) telemetry.ModelClass {
			switch {
			case model == "claude-opus-5-5" && cataloged.Load():
				return telemetry.ModelCatalog
			case model == "/home/ana/private/model.gguf":
				return telemetry.ModelCustom
			default:
				return telemetry.ModelUnknown
			}
		}
		startRuntimeWithRegistries(t, exportConfig(nil), nil, &fakeNetwork{}, reg)

		if got := telemetry.ModelLabel("claude", "claude-opus-5-5"); got != telemetry.ValueOther {
			t.Fatalf("ModelLabel() before the catalog knows the model = %q, want %q", got, telemetry.ValueOther)
		}
		cataloged.Store(true)
		for model, want := range map[string]string{
			"claude-opus-5-5":              "claude-opus-5-5",
			"/home/ana/private/model.gguf": telemetry.ValueCustom,
			"my model v2":                  telemetry.ValueOther,
		} {
			if got := telemetry.ModelLabel("claude", model); got != want {
				t.Fatalf("ModelLabel(%q) = %q, want %q", model, got, want)
			}
		}
	})

	t.Run("Should export recorded model sentinels unchanged", func(t *testing.T) {
		started := startRuntime(t, exportConfig(nil), nil, &fakeNetwork{})
		started.exportSpanWith(t, trace.WithAttributes(
			telemetry.AttrGenAIModel.String(telemetry.ValueCustom),
		))

		requests, _ := started.network.captured()
		if len(requests) != 1 || !bytes.Contains(requests[0].body, []byte(telemetry.ValueCustom)) {
			t.Fatalf(
				"trace export = %d requests, want the %q model sentinel kept",
				len(requests),
				telemetry.ValueCustom,
			)
		}
	})
}
