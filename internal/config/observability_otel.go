package config

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/vault"
)

// OTLP wire protocols accepted by [observability.otel].
const (
	OTelProtocolHTTPProtobuf = "http/protobuf"
	OTelProtocolGRPC         = "grpc"
)

// Standard OpenTelemetry variables honored for values config.toml does not write.
const (
	otelEnvEndpoint    = "OTEL_EXPORTER_OTLP_ENDPOINT"
	otelEnvProtocol    = "OTEL_EXPORTER_OTLP_PROTOCOL"
	otelEnvHeaders     = "OTEL_EXPORTER_OTLP_HEADERS"
	otelEnvServiceName = "OTEL_SERVICE_NAME"
	otelEnvSDKDisabled = "OTEL_SDK_DISABLED"
)

const (
	otelConfigPath           = "observability.otel"
	otelHeaderVaultNamespace = "observability"
	otelHeaderVaultPrefix    = "vault:" + otelHeaderVaultNamespace + "/"
	minOTelMetricInterval    = 5 * time.Second
)

var (
	otelConfigSegments     = strings.Split(otelConfigPath, ".")
	otelServiceNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	// otelHeaderNamePattern is the HTTP token grammar the OTLP exporters accept for header names.
	otelHeaderNamePattern = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
)

// ObservabilityOTelConfig configures OTLP trace and metric export.
type ObservabilityOTelConfig struct {
	Enabled  bool   `toml:"enabled"`
	Protocol string `toml:"protocol"`
	// Endpoint is the collector base URL; empty selects the protocol's local default.
	Endpoint string `toml:"endpoint"`
	// Headers maps header names to env: or vault:observability/ refs, never literal values.
	Headers        map[string]string `toml:"headers,omitempty"`
	Traces         bool              `toml:"traces"`
	Metrics        bool              `toml:"metrics"`
	SampleRatio    float64           `toml:"sample_ratio"`
	MetricInterval time.Duration     `toml:"metric_interval"`
	ServiceName    string            `toml:"service_name"`
	// SlowDBWriteMS is the write-span threshold; 0 disables write spans.
	SlowDBWriteMS      int  `toml:"slow_db_write_ms"`
	ForwardToProviders bool `toml:"forward_to_providers"`

	env otelEnvironment
}

// otelEnvironment records what ApplyOTelEnvironment supplied. It stays unexported so it is never
// rendered, persisted, or hashed: environment headers carry literal credentials.
type otelEnvironment struct {
	// endpoint, protocol, and serviceName name the variable that supplied the field, "" otherwise.
	endpoint, protocol, serviceName string
	headers                         map[string]string
	headersInvalid                  bool
	disabled                        bool
}

// ApplyOTelEnvironment fills [observability.otel] from standard OTEL_* variables. Callers apply it
// before the config file overlay so any value written in config.toml wins.
func ApplyOTelEnvironment(cfg *Config, getenv func(string) string) {
	if cfg == nil || getenv == nil {
		return
	}
	otel := &cfg.Observability.OTel
	if value := strings.TrimSpace(getenv(otelEnvEndpoint)); value != "" {
		otel.Endpoint, otel.env.endpoint = value, otelEnvEndpoint
	}
	if value := strings.TrimSpace(getenv(otelEnvProtocol)); value != "" {
		otel.Protocol, otel.env.protocol = value, otelEnvProtocol
	}
	if value := strings.TrimSpace(getenv(otelEnvServiceName)); value != "" {
		otel.ServiceName, otel.env.serviceName = value, otelEnvServiceName
	}
	if value := getenv(otelEnvHeaders); strings.TrimSpace(value) != "" {
		headers, ok := parseOTelHeaders(value)
		otel.env.headers, otel.env.headersInvalid = headers, !ok
	}
	otel.env.disabled = strings.EqualFold(strings.TrimSpace(getenv(otelEnvSDKDisabled)), "true")
}

// EnvHeaders returns headers from OTEL_EXPORTER_OTLP_HEADERS. A non-empty headers table in
// config.toml replaces them entirely.
func (c ObservabilityOTelConfig) EnvHeaders() map[string]string {
	return maps.Clone(c.env.headers)
}

// EnvDisabled reports OTEL_SDK_DISABLED=true, which forces export off regardless of config.
func (c ObservabilityOTelConfig) EnvDisabled() bool {
	return c.env.disabled
}

// Validate checks [observability.otel] after every layer is applied. A value from an OTEL_*
// variable is checked only while export can run, so variables set for other tools cannot block
// a daemon that exports nothing.
func (c ObservabilityOTelConfig) Validate() error {
	if err := c.validateSourcedValues(); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(c.Headers)) {
		if err := validateOTelHeader(name, c.Headers[name]); err != nil {
			return err
		}
	}
	switch {
	case math.IsNaN(c.SampleRatio) || c.SampleRatio < 0 || c.SampleRatio > 1:
		return errors.New(otelConfigPath + ".sample_ratio must be between 0 and 1")
	case c.MetricInterval < minOTelMetricInterval:
		return fmt.Errorf("%s.metric_interval must be at least %s", otelConfigPath, minOTelMetricInterval)
	case c.SlowDBWriteMS < 0:
		return errors.New(otelConfigPath + ".slow_db_write_ms must be >= 0")
	default:
		return nil
	}
}

func (c ObservabilityOTelConfig) validateSourcedValues() error {
	if c.Protocol != OTelProtocolHTTPProtobuf && c.Protocol != OTelProtocolGRPC {
		if subject, check := c.valueSubject(c.env.protocol, "protocol"); check {
			return fmt.Errorf("%s must be %q or %q", subject, OTelProtocolHTTPProtobuf, OTelProtocolGRPC)
		}
	}
	if !validOTelEndpoint(c.Endpoint) {
		if subject, check := c.valueSubject(c.env.endpoint, "endpoint"); check {
			return fmt.Errorf("%s must be an http(s) URL with a host", subject)
		}
	}
	if !otelServiceNamePattern.MatchString(c.ServiceName) {
		if subject, check := c.valueSubject(c.env.serviceName, "service_name"); check {
			return fmt.Errorf("%s must match %s", subject, otelServiceNamePattern)
		}
	}
	if c.exportCanRun() && c.env.headersInvalid {
		return errors.New(otelEnvHeaders + " must be a comma-separated list of name=value pairs")
	}
	return nil
}

// valueSubject names the source of an invalid value and reports whether it must be checked now.
func (c ObservabilityOTelConfig) valueSubject(envVar, key string) (string, bool) {
	if envVar == "" {
		return otelConfigPath + "." + key, true
	}
	return envVar, c.exportCanRun()
}

func (c ObservabilityOTelConfig) exportCanRun() bool {
	return c.Enabled && !c.env.disabled
}

func (c ObservabilityOTelConfig) clone() ObservabilityOTelConfig {
	c.Headers = maps.Clone(c.Headers)
	c.env.headers = maps.Clone(c.env.headers)
	return c
}

// validOTelEndpoint accepts "" (protocol default) or an http(s) URL with a host. A base path is
// allowed because hosted collectors serve OTLP below one.
func validOTelEndpoint(raw string) bool {
	if raw == "" {
		return true
	}
	endpoint, err := url.Parse(raw)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Hostname() == "" {
		return false
	}
	// Secrets belong in header refs; userinfo, a query, or a fragment would be echoed by config output.
	if endpoint.User != nil || endpoint.RawQuery != "" || endpoint.ForceQuery || endpoint.Fragment != "" {
		return false
	}
	port := endpoint.Port()
	_, err = strconv.ParseUint(port, 10, 16)
	return port == "" || err == nil
}

func validateOTelHeader(name, ref string) error {
	key := fmt.Sprintf("%s.headers.%q", otelConfigPath, name)
	if !otelHeaderNamePattern.MatchString(name) {
		return fmt.Errorf("%s must be a valid HTTP header name", key)
	}
	normalized := vault.NormalizeRef(ref)
	switch {
	case !vault.IsEnvRef(normalized) && !vault.IsSecretRef(normalized):
		return fmt.Errorf("%s must be an env: or %s ref", key, otelHeaderVaultPrefix)
	case vault.IsSecretRef(normalized) && !strings.HasPrefix(normalized, otelHeaderVaultPrefix):
		return fmt.Errorf("%s is invalid: %s must use %s<path>", key, normalized, otelHeaderVaultPrefix)
	}
	if err := vault.ValidateRefNamespace(normalized, otelHeaderVaultNamespace); err != nil {
		return fmt.Errorf("%s is invalid: %w", key, err)
	}
	return nil
}

// parseOTelHeaders reads the OTLP exporter header list: comma-separated name=value pairs with
// percent-encoded values.
func parseOTelHeaders(raw string) (map[string]string, bool) {
	headers := make(map[string]string)
	for entry := range strings.SplitSeq(raw, ",") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		name, value, found := strings.Cut(entry, "=")
		name = strings.TrimSpace(name)
		decoded, err := url.PathUnescape(value)
		if !found || err != nil || !otelHeaderNamePattern.MatchString(name) {
			return nil, false
		}
		headers[name] = strings.TrimSpace(decoded)
	}
	return headers, true
}
