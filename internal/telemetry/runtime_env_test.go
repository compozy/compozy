package telemetry_test

import (
	"log/slog"
	"testing"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/version"
	"go.opentelemetry.io/otel"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
)

// UT-013: the exporters read OTEL_EXPORTER_OTLP_* from the process themselves; the endpoint config resolved must win.
func TestInitPrefersTheConfigEndpointOverTheEnvironment(t *testing.T) {
	// not parallel: t.Setenv sets the process environment the exporters read.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://env:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://signal-env:4318/v1/traces")

	t.Run("Should export to the config.toml endpoint", func(t *testing.T) {
		cfg := environmentConfig(map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://env:4318"},
			func(otel *config.ObservabilityOTelConfig) {
				otel.Enabled = true
				otel.Endpoint = "http://cfg:4318"
			})
		started := startRuntime(t, cfg, nil, &fakeNetwork{})
		started.exportSpan(t)

		requests, _ := started.network.captured()
		assertRequestURLs(t, requests, "http://cfg:4318/v1/traces")
		if host := started.runtime.Status().EndpointHost; host != "cfg:4318" {
			t.Fatalf("Status().EndpointHost = %q, want cfg:4318", host)
		}
	})
}

// UT-010 (global providers; the connection half is in runtime_test.go)
func TestInitInstallsTheGlobalProviders(t *testing.T) {
	// not parallel: Init replaces the process-wide OTel providers.
	tracerProvider, meterProvider := otel.GetTracerProvider(), otel.GetMeterProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(tracerProvider)
		otel.SetMeterProvider(meterProvider)
	})

	t.Run("Should install no-op global providers when export is disabled", func(t *testing.T) {
		runtime, err := telemetry.Init(
			t.Context(),
			config.DefaultWithHome(config.HomePaths{}).Observability,
			nil,
			telemetry.Registries{},
			version.Info{Version: testServiceVersion},
			slog.New(slog.DiscardHandler),
		)
		if err != nil {
			t.Fatalf("Init() error = %v", err)
		}
		if err := runtime.Shutdown(t.Context()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}

		if _, span := telemetry.Tracer(telemetry.ScopeSession).Start(t.Context(), "x"); span.IsRecording() {
			t.Fatal("global tracer span is recording, want the no-op tracer")
		}
		if _, ok := otel.GetMeterProvider().(metricnoop.MeterProvider); !ok {
			t.Fatalf("global meter provider = %T, want no-op", otel.GetMeterProvider())
		}
		if _, ok := telemetry.Meter(telemetry.ScopeSession).(metricnoop.Meter); !ok {
			t.Fatalf("Meter() = %T, want the no-op meter", telemetry.Meter(telemetry.ScopeSession))
		}
	})
}
