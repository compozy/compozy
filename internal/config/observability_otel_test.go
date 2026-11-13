package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestObservabilityOTelConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should default OTel export to off with the documented values [UT-001]", func(t *testing.T) {
		t.Parallel()

		got := defaultObservabilityConfig().OTel
		want := ObservabilityOTelConfig{
			Enabled:            false,
			Protocol:           "http/protobuf",
			Endpoint:           "",
			Traces:             true,
			Metrics:            true,
			SampleRatio:        1.0,
			MetricInterval:     60 * time.Second,
			ServiceName:        "compozy-daemon",
			SlowDBWriteMS:      50,
			ForwardToProviders: false,
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("defaultObservabilityConfig().OTel = %#v, want %#v", got, want)
		}
	})

	t.Run("Should accept documented values and boundaries", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*ObservabilityOTelConfig)
		}{
			{name: "Should accept the defaults", mutate: func(*ObservabilityOTelConfig) {}},
			{
				name: "Should accept a gRPC collector URL with a base path",
				mutate: func(c *ObservabilityOTelConfig) {
					c.Protocol = "grpc"
					c.Endpoint = "https://otlp.example.test:4317/otlp"
				},
			},
			{
				name:   "Should accept an HTTP collector URL with a base path and no port",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https://otlp-gateway.example/otlp" },
			},
			{
				name:   "Should accept an IPv6 loopback collector",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "http://[::1]:4318" },
			},
			{
				name: "Should accept env and observability vault header refs [UT-004]",
				mutate: func(c *ObservabilityOTelConfig) {
					c.Headers = map[string]string{"api-key": "vault:observability/nr", "Authorization": "env:NR_KEY"}
				},
			},
			{
				name:   "Should accept a zero sample ratio [UT-005]",
				mutate: func(c *ObservabilityOTelConfig) { c.SampleRatio = 0 },
			},
			{
				name:   "Should accept a full sample ratio [UT-005]",
				mutate: func(c *ObservabilityOTelConfig) { c.SampleRatio = 1 },
			},
			{
				name:   "Should accept the minimum metric interval [UT-006]",
				mutate: func(c *ObservabilityOTelConfig) { c.MetricInterval = 5 * time.Second },
			},
			{
				name:   "Should accept a zero slow write threshold",
				mutate: func(c *ObservabilityOTelConfig) { c.SlowDBWriteMS = 0 },
			},
			{
				name:   "Should accept a service name with dots and underscores [UT-007]",
				mutate: func(c *ObservabilityOTelConfig) { c.ServiceName = "compozy-daemon.dev_1" },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := defaultObservabilityConfig().OTel
				tt.mutate(&cfg)
				if err := cfg.Validate(true); err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
			})
		}
	})

	t.Run("Should report the documented validation errors", func(t *testing.T) {
		t.Parallel()

		const (
			endpointErr    = "observability.otel.endpoint must be an http(s) URL with a host"
			sampleRatioErr = "observability.otel.sample_ratio must be between 0 and 1"
			serviceNameErr = "observability.otel.service_name must match ^[A-Za-z0-9._-]{1,64}$"
		)
		tests := []struct {
			name   string
			mutate func(*ObservabilityOTelConfig)
			want   string
		}{
			{
				name:   "Should reject an unsupported protocol [UT-002]",
				mutate: func(c *ObservabilityOTelConfig) { c.Protocol = "thrift" },
				want:   `observability.otel.protocol must be "http/protobuf" or "grpc"`,
			},
			{
				name:   "Should reject an endpoint without a URL scheme [UT-003]",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "collector:4318" },
				want:   endpointErr,
			},
			{
				name:   "Should reject a non-HTTP endpoint scheme [UT-003]",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "ftp://x" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint without a host [UT-003]",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "http://" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint with a path but no host [UT-003]",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https:///v1/traces" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint with embedded credentials without echoing them",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https://user:s3cret@collector.example:4318" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint with a query string",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https://collector.example:4318?token=s3cret" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint with an empty query string",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https://collector.example:4318?" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint with a fragment",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "https://collector.example:4318#s3cret" },
				want:   endpointErr,
			},
			{
				name:   "Should reject an endpoint port outside the TCP range",
				mutate: func(c *ObservabilityOTelConfig) { c.Endpoint = "http://collector.example:65536" },
				want:   endpointErr,
			},
			{
				name:   "Should reject a literal header value without echoing it [UT-004]",
				mutate: func(c *ObservabilityOTelConfig) { c.Headers = map[string]string{"api-key": "abc123"} },
				want:   `observability.otel.headers."api-key" must be an env: or vault:observability/ ref`,
			},
			{
				name:   "Should reject a header name with a space",
				mutate: func(c *ObservabilityOTelConfig) { c.Headers = map[string]string{"api key": "env:NR_KEY"} },
				want:   `observability.otel.headers."api key" must be a valid HTTP header name`,
			},
			{
				name:   "Should reject a header name with a line break",
				mutate: func(c *ObservabilityOTelConfig) { c.Headers = map[string]string{"x\r\nHost": "env:NR_KEY"} },
				want:   `observability.otel.headers."x\r\nHost" must be a valid HTTP header name`,
			},
			{
				name:   "Should reject an empty header name",
				mutate: func(c *ObservabilityOTelConfig) { c.Headers = map[string]string{"": "env:NR_KEY"} },
				want:   `observability.otel.headers."" must be a valid HTTP header name`,
			},
			{
				name:   "Should reject a header ref outside the observability vault namespace [UT-004]",
				mutate: func(c *ObservabilityOTelConfig) { c.Headers = map[string]string{"api-key": "vault:other/x"} },
				want:   `observability.otel.headers."api-key" is invalid: vault:other/x must use vault:observability/<path>`,
			},
			{
				name:   "Should reject a negative sample ratio [UT-005]",
				mutate: func(c *ObservabilityOTelConfig) { c.SampleRatio = -0.1 },
				want:   sampleRatioErr,
			},
			{
				name:   "Should reject a sample ratio above one [UT-005]",
				mutate: func(c *ObservabilityOTelConfig) { c.SampleRatio = 1.5 },
				want:   sampleRatioErr,
			},
			{
				name:   "Should reject a NaN sample ratio",
				mutate: func(c *ObservabilityOTelConfig) { c.SampleRatio = math.NaN() },
				want:   sampleRatioErr,
			},
			{
				name:   "Should reject a metric interval below five seconds [UT-006]",
				mutate: func(c *ObservabilityOTelConfig) { c.MetricInterval = 4 * time.Second },
				want:   "observability.otel.metric_interval must be at least 5s",
			},
			{
				name:   "Should reject a negative slow write threshold [UT-007]",
				mutate: func(c *ObservabilityOTelConfig) { c.SlowDBWriteMS = -1 },
				want:   "observability.otel.slow_db_write_ms must be >= 0",
			},
			{
				name:   "Should reject a blank service name [UT-007]",
				mutate: func(c *ObservabilityOTelConfig) { c.ServiceName = "  " },
				want:   serviceNameErr,
			},
			{
				name:   "Should reject a service name with spaces [UT-007]",
				mutate: func(c *ObservabilityOTelConfig) { c.ServiceName = "a b" },
				want:   serviceNameErr,
			},
			{
				name:   "Should reject a service name longer than 64 characters [UT-007]",
				mutate: func(c *ObservabilityOTelConfig) { c.ServiceName = strings.Repeat("a", 65) },
				want:   serviceNameErr,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := defaultObservabilityConfig().OTel
				tt.mutate(&cfg)
				err := cfg.Validate(true)
				if err == nil || err.Error() != tt.want {
					t.Fatalf("Validate() error = %v, want %q", err, tt.want)
				}
			})
		}
	})

	t.Run("Should reject malformed header refs under their header key [UT-004]", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			ref  string
		}{
			{name: "Should reject an env ref with an invalid variable name", ref: "env:1BAD"},
			{name: "Should reject an env ref without a variable name", ref: "env:"},
			{name: "Should reject an observability vault ref without a path", ref: "vault:observability/"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				cfg := defaultObservabilityConfig().OTel
				cfg.Headers = map[string]string{"api-key": tt.ref}
				err := cfg.Validate(true)
				if want := `observability.otel.headers."api-key" is invalid: `; err == nil ||
					!strings.HasPrefix(err.Error(), want) {
					t.Fatalf("Validate() error = %v, want prefix %q", err, want)
				}
			})
		}
	})
}

func TestApplyOTelEnvironment(t *testing.T) {
	t.Parallel()

	t.Run("Should fill export settings from trimmed standard OTEL variables", func(t *testing.T) {
		t.Parallel()

		otel := applyOTelLayers(t, map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": " http://env:4318 ",
			"OTEL_EXPORTER_OTLP_PROTOCOL": " grpc ",
			"OTEL_SERVICE_NAME":           " env-daemon ",
			"OTEL_EXPORTER_OTLP_HEADERS":  "api-key=k%20123, x-tenant = acme",
			"OTEL_SDK_DISABLED":           "TRUE",
		}, "")
		if otel.Endpoint != "http://env:4318" || otel.Protocol != "grpc" || otel.ServiceName != "env-daemon" {
			t.Fatalf("OTel endpoint/protocol/service = %q/%q/%q, want trimmed environment values",
				otel.Endpoint, otel.Protocol, otel.ServiceName)
		}
		if got, want := otel.EnvHeaders(), map[string]string{"api-key": "k 123", "x-tenant": "acme"}; !maps.Equal(
			got,
			want,
		) {
			t.Fatalf("EnvHeaders() = %#v, want %#v", got, want)
		}
		if !otel.EnvDisabled() {
			t.Fatal("EnvDisabled() = false, want true for OTEL_SDK_DISABLED=TRUE")
		}
		if len(otel.Headers) != 0 {
			t.Fatalf("Headers = %#v, want environment headers kept apart from config refs", otel.Headers)
		}
	})

	t.Run("Should name the variable behind an invalid value while export is enabled", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			variable string
			value    string
			want     string
		}{
			{
				name:     "Should reject an unsupported protocol variable",
				variable: "OTEL_EXPORTER_OTLP_PROTOCOL",
				value:    "http/json",
				want:     `OTEL_EXPORTER_OTLP_PROTOCOL must be "http/protobuf" or "grpc"`,
			},
			{
				name:     "Should reject an endpoint variable without a URL scheme",
				variable: "OTEL_EXPORTER_OTLP_ENDPOINT",
				value:    "localhost:4318",
				want:     "OTEL_EXPORTER_OTLP_ENDPOINT must be an http(s) URL with a host",
			},
			{
				name:     "Should reject an endpoint variable with embedded credentials without echoing them",
				variable: "OTEL_EXPORTER_OTLP_ENDPOINT",
				value:    "https://user:s3cret@collector.example:4318",
				want:     "OTEL_EXPORTER_OTLP_ENDPOINT must be an http(s) URL with a host",
			},
			{
				name:     "Should reject a service name variable outside the name grammar",
				variable: "OTEL_SERVICE_NAME",
				value:    "my service",
				want:     "OTEL_SERVICE_NAME must match ^[A-Za-z0-9._-]{1,64}$",
			},
			{
				name:     "Should reject malformed headers without echoing their values",
				variable: "OTEL_EXPORTER_OTLP_HEADERS",
				value:    "api-key:secret-token",
				want:     "OTEL_EXPORTER_OTLP_HEADERS must be a comma-separated list of name=value pairs",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				otel := applyOTelLayers(
					t,
					map[string]string{tt.variable: tt.value},
					"[observability.otel]\nenabled = true\n",
				)
				err := otel.Validate(true)
				if err == nil || err.Error() != tt.want {
					t.Fatalf("Validate() error = %v, want %q", err, tt.want)
				}
			})
		}
	})

	t.Run("Should ignore invalid OTEL variables while export cannot run", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name                 string
			file                 string
			sdkDisabled          string
			observabilityEnabled bool
		}{
			{name: "Should ignore them while export is disabled", observabilityEnabled: true},
			{
				name:                 "Should ignore them while OTEL_SDK_DISABLED forces export off",
				file:                 "[observability.otel]\nenabled = true\n",
				sdkDisabled:          "true",
				observabilityEnabled: true,
			},
			{
				name: "Should ignore them while the parent observability switch is off",
				file: "[observability.otel]\nenabled = true\n",
			},
			{
				name:                 "Should ignore them while traces and metrics are both off",
				file:                 "[observability.otel]\nenabled = true\ntraces = false\nmetrics = false\n",
				observabilityEnabled: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				otel := applyOTelLayers(t, map[string]string{
					"OTEL_EXPORTER_OTLP_ENDPOINT": "localhost:4318",
					"OTEL_EXPORTER_OTLP_PROTOCOL": "http/json",
					"OTEL_SERVICE_NAME":           "my service",
					"OTEL_EXPORTER_OTLP_HEADERS":  "api-key:secret-token",
					"OTEL_SDK_DISABLED":           tt.sdkDisabled,
				}, tt.file)
				if err := otel.Validate(tt.observabilityEnabled); err != nil {
					t.Fatalf("Validate() error = %v, want OTEL variables for other tools ignored", err)
				}
				if got, want := otel.EnvDisabled(), tt.sdkDisabled != ""; got != want {
					t.Fatalf("EnvDisabled() = %t, want %t", got, want)
				}
			})
		}
	})

	t.Run("Should report a config.toml value under its key after it replaces an environment value", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			variable string
			envValue string
			key      string
			value    string
			want     string
		}{
			{
				name:     "Should name the protocol key",
				variable: "OTEL_EXPORTER_OTLP_PROTOCOL",
				envValue: "grpc",
				key:      "protocol",
				value:    "thrift",
				want:     `observability.otel.protocol must be "http/protobuf" or "grpc"`,
			},
			{
				name:     "Should name the endpoint key",
				variable: "OTEL_EXPORTER_OTLP_ENDPOINT",
				envValue: "http://env:4318",
				key:      "endpoint",
				value:    "collector:4318",
				want:     "observability.otel.endpoint must be an http(s) URL with a host",
			},
			{
				name:     "Should name the service name key",
				variable: "OTEL_SERVICE_NAME",
				envValue: "env-daemon",
				key:      "service_name",
				value:    "a b",
				want:     "observability.otel.service_name must match ^[A-Za-z0-9._-]{1,64}$",
			},
		}
		for _, tt := range tests {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s while export enabled is %t", tt.name, enabled), func(t *testing.T) {
					t.Parallel()

					file := fmt.Sprintf("[observability.otel]\nenabled = %t\n%s = %q\n", enabled, tt.key, tt.value)
					err := applyOTelLayers(t, map[string]string{tt.variable: tt.envValue}, file).Validate(true)
					if err == nil || err.Error() != tt.want {
						t.Fatalf("Validate() error = %v, want %q", err, tt.want)
					}
				})
			}
		}
	})

	t.Run("Should accept a config.toml headers table that replaces malformed environment headers", func(t *testing.T) {
		t.Parallel()

		otel := applyOTelLayers(
			t,
			map[string]string{"OTEL_EXPORTER_OTLP_HEADERS": "api-key:secret-token"},
			"[observability.otel]\nenabled = true\n\n[observability.otel.headers]\n\"api-key\" = \"env:NR_KEY\"\n",
		)
		if err := otel.Validate(true); err != nil {
			t.Fatalf("Validate() error = %v, want malformed environment headers replaced by the file table", err)
		}
	})

	t.Run("Should keep environment headers when config.toml declares an empty headers table", func(t *testing.T) {
		t.Parallel()

		otel := applyOTelLayers(
			t,
			map[string]string{"OTEL_EXPORTER_OTLP_HEADERS": "api-key=env-key"},
			"[observability.otel.headers]\n",
		)
		if got, want := otel.EnvHeaders(), map[string]string{"api-key": "env-key"}; !maps.Equal(got, want) {
			t.Fatalf("EnvHeaders() = %#v, want %#v", got, want)
		}
	})

	t.Run(
		"Should use the endpoint variable only when config.toml omits the endpoint [US-001 AC-3, AC-4]",
		func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name string
				file string
				want string
			}{
				{
					name: "Should use the variable when the file omits the endpoint",
					file: "[observability.otel]\nenabled = true\n",
					want: "http://env:4318",
				},
				{
					name: "Should treat an empty file endpoint as unset",
					file: "[observability.otel]\nenabled = true\nendpoint = \"\"\n",
					want: "http://env:4318",
				},
				{
					name: "Should keep the file endpoint over the variable",
					file: "[observability.otel]\nenabled = true\nendpoint = \"http://cfg:4318\"\n",
					want: "http://cfg:4318",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					otel := applyOTelLayers(
						t,
						map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://env:4318"},
						tt.file,
					)
					if otel.Endpoint != tt.want {
						t.Fatalf("Endpoint = %q, want %q", otel.Endpoint, tt.want)
					}
					if err := otel.Validate(true); err != nil {
						t.Fatalf("Validate() error = %v, want nil", err)
					}
				})
			}
		},
	)

	t.Run("Should redact invalid environment endpoints without blocking inactive export", func(t *testing.T) {
		t.Parallel()

		for _, endpoint := range []string{
			"https://collector.example/?auth=opaque-789456",
			"https://user:opaque-789456@collector.example/",
			"https://collector.example/#opaque-789456",
			"https://collector.example/?auth=%zz-opaque-789456",
			"http://collector.example:invalid/opaque-789456",
		} {
			t.Run("Should hide "+endpoint, func(t *testing.T) {
				t.Parallel()

				cfg := DefaultWithHome(HomePaths{})
				ApplyOTelEnvironment(&cfg, func(key string) string {
					if key == otelEnvEndpoint {
						return endpoint
					}
					return ""
				})
				if err := cfg.Observability.OTel.Validate(true); err != nil {
					t.Fatalf("Validate() error = %v, want inactive environment ignored", err)
				}
				values := RedactedConfigMap(&cfg)
				entry, ok := EntryByPath(FlattenConfigEntries(values), "observability.otel.endpoint")
				if !ok || entry.Value != RedactedValue() {
					t.Fatalf("endpoint entry = %#v, %t; want redacted", entry, ok)
				}
				if strings.Contains(fmt.Sprint(values), "opaque-789456") {
					t.Fatal("config map exposed endpoint credentials")
				}
				if cfg.Observability.OTel.Endpoint != endpoint {
					t.Fatal("config inspection changed the effective endpoint")
				}
			})
		}
	})

	t.Run("Should keep literal environment header values out of rendered config", func(t *testing.T) {
		t.Parallel()

		const secret = "ENV-HEADER-LITERAL-7f3a"
		cfg := DefaultWithHome(HomePaths{})
		ApplyOTelEnvironment(&cfg, func(key string) string {
			return map[string]string{"OTEL_EXPORTER_OTLP_HEADERS": "api-key=" + secret}[key]
		})
		if got := cfg.Observability.OTel.EnvHeaders()["api-key"]; got != secret {
			t.Fatalf("EnvHeaders()[api-key] = %q, want the environment value kept for the exporter", got)
		}
		hashInput, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("json.Marshal(config) error = %v", err)
		}
		renderings := map[string]string{
			"config snapshot hash input": string(hashInput),
			"config get entries":         fmt.Sprint(FlattenConfigEntries(RedactedConfigMap(&cfg))),
		}
		for name, rendered := range renderings {
			if strings.Contains(rendered, secret) {
				t.Fatalf("%s contains the literal OTEL_EXPORTER_OTLP_HEADERS value", name)
			}
		}
	})
}

func TestObservabilityOTelConfigIsGlobalOnly(t *testing.T) {
	t.Parallel()

	t.Run("Should reject OTel export settings in workspace files", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			contents string
		}{
			{name: "Should reject the export switch", contents: "[observability.otel]\nenabled = true\n"},
			{
				name:     "Should reject a headers table",
				contents: "[observability.otel.headers]\n\"api-key\" = \"env:NR_KEY\"\n",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := loadWorkspaceConfigOverlayBytes([]byte(tt.contents), "workspace.toml")
				if want := "observability.otel settings are global-only"; err == nil ||
					!strings.Contains(err.Error(), want) {
					t.Fatalf("loadWorkspaceConfigOverlayBytes() error = %v, want %q", err, want)
				}
			})
		}
	})

	t.Run("Should keep accepting event retention settings in workspace files", func(t *testing.T) {
		t.Parallel()

		if _, err := loadWorkspaceConfigOverlayBytes(
			[]byte("[observability]\nretention_days = 7\n"),
			"workspace.toml",
		); err != nil {
			t.Fatalf("loadWorkspaceConfigOverlayBytes() error = %v, want nil", err)
		}
	})

	t.Run("Should keep OTel export writes out of workspace and profile layers", func(t *testing.T) {
		t.Parallel()

		path := []string{"observability", "otel", "enabled"}
		err := ValidateConfigWriteScope(WriteScopeWorkspace, path)
		if err == nil || !strings.Contains(err.Error(), "global-only") ||
			!strings.Contains(err.Error(), "--scope user") {
			t.Fatalf(
				"ValidateConfigWriteScope(workspace) error = %v, want global-only rejection with user-scope guidance",
				err,
			)
		}
		if err := ValidateConfigWriteScope(WriteScopeProfile, path); err == nil ||
			!strings.Contains(err.Error(), "machine-only") {
			t.Fatalf("ValidateConfigWriteScope(profile) error = %v, want profile denial", err)
		}
		if err := ValidateConfigWriteScope(WriteScopeUser, path); err != nil {
			t.Fatalf("ValidateConfigWriteScope(user) error = %v, want nil", err)
		}
	})

	t.Run(
		"Should refuse OTel export settings from repo-controlled files through the config loader",
		func(t *testing.T) {
			t.Parallel()

			// A cloned repository controls these files, so none may enable export, redirect the collector, or
			// reference an operator credential, whichever TOML spelling it uses.
			const profile = "dev"
			tests := []struct {
				name    string
				profile string
				path    func(homePaths HomePaths, workspaceRoot string) string
				content string
				want    string
			}{
				{
					name:    "Should refuse a workspace OTel table",
					path:    func(_ HomePaths, root string) string { return workspaceConfigFile(root) },
					content: "[observability.otel]\nenabled = true\nendpoint = \"https://collector.example\"\n",
					want:    "observability.otel settings are global-only",
				},
				{
					name:    "Should refuse a workspace headers table naming an environment variable",
					path:    func(_ HomePaths, root string) string { return workspaceConfigFile(root) },
					content: "[observability.otel.headers]\n\"api-key\" = \"env:AWS_SECRET_ACCESS_KEY\"\n",
					want:    "observability.otel settings are global-only",
				},
				{
					name:    "Should refuse a workspace OTel inline table",
					path:    func(_ HomePaths, root string) string { return workspaceConfigFile(root) },
					content: "observability = { otel = { enabled = true } }\n",
					want:    "observability.otel settings are global-only",
				},
				{
					name:    "Should refuse a workspace OTel dotted key",
					path:    func(_ HomePaths, root string) string { return workspaceConfigFile(root) },
					content: "observability.otel.endpoint = \"https://collector.example\"\n",
					want:    "observability.otel settings are global-only",
				},
				{
					name:    "Should refuse a workspace empty OTel table",
					path:    func(_ HomePaths, root string) string { return workspaceConfigFile(root) },
					content: "[observability.otel]\n",
					want:    "observability.otel settings are global-only",
				},
				{
					name:    "Should refuse a profile overlay OTel table",
					profile: profile,
					path:    func(homePaths HomePaths, _ string) string { return profileConfigFile(homePaths, profile) },
					content: "[observability.otel]\nenabled = true\n",
					want:    "observability.otel is machine-only",
				},
				{
					name:    "Should refuse a workspace profile overlay OTel table",
					profile: profile,
					path:    func(_ HomePaths, root string) string { return workspaceProfileConfigFile(root, profile) },
					content: "[observability.otel]\nenabled = true\n",
					want:    "observability.otel is machine-only",
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					t.Parallel()

					homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
					if err != nil {
						t.Fatalf("ResolveHomePathsFrom() error = %v", err)
					}
					workspaceRoot := filepath.Join(t.TempDir(), "workspace")
					writeFile(t, tt.path(homePaths, workspaceRoot), tt.content)
					options := []LoadOption{WithWorkspaceRoot(workspaceRoot)}
					if tt.profile != "" {
						options = append(options, WithProfile(tt.profile))
					}
					if _, err := LoadForHome(
						homePaths,
						options...); err == nil ||
						!strings.Contains(err.Error(), tt.want) {
						t.Fatalf("LoadForHome() error = %v, want %q", err, tt.want)
					}
				})
			}
		},
	)

	t.Run("Should keep loading workspace event retention settings through the config loader", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		writeFile(t, workspaceConfigFile(workspaceRoot), "[observability]\nretention_days = 3\n")
		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v, want nil", err)
		}
		if got, want := cfg.Observability.RetentionDays, 3; got != want {
			t.Fatalf("Observability.RetentionDays = %d, want %d", got, want)
		}
		if cfg.Observability.OTel.Enabled {
			t.Fatal("Observability.OTel.Enabled = true, want export off without a global opt-in")
		}
	})
}

// applyOTelLayers applies OTEL_* variables and then a config.toml overlay, in loadConfigFromHome order.
func applyOTelLayers(t *testing.T, env map[string]string, file string) ObservabilityOTelConfig {
	t.Helper()

	cfg := DefaultWithHome(HomePaths{})
	ApplyOTelEnvironment(&cfg, func(key string) string { return env[key] })
	overlayPath := filepath.Join(t.TempDir(), "config.toml")
	writeFile(t, overlayPath, file)
	if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
		t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
	}
	return cfg.Observability.OTel
}

// Not parallel: t.Setenv. Invariant: config inspection resolves the OTEL_* layer the daemon boots with, from the
// process environment only. Owner: config loader.
func TestLoadForHomeAppliesOTelProcessEnvironment(t *testing.T) {
	t.Run("Should report the protocol from OTEL_EXPORTER_OTLP_PROTOCOL when config.toml sets none", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		t.Setenv(otelEnvProtocol, OTelProtocolGRPC)

		cfg, err := LoadForHome(homePaths)
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if got := cfg.Observability.OTel.Protocol; got != OTelProtocolGRPC {
			t.Fatalf("LoadForHome() otel protocol = %q, want %q from the process environment", got, OTelProtocolGRPC)
		}
	})

	t.Run("Should ignore OTEL_* variables from a workspace dotenv file", func(t *testing.T) {
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		writeFile(t, WorkspaceDotEnvFile(workspaceRoot), otelEnvProtocol+"="+OTelProtocolGRPC+"\n")
		t.Setenv(otelEnvProtocol, "")

		cfg, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		if got := cfg.Observability.OTel.Protocol; got != OTelProtocolHTTPProtobuf {
			t.Fatalf("LoadForHome() otel protocol = %q, want the default; a workspace dotenv must not set it", got)
		}
	})
}
