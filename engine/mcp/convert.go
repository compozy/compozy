package mcp

import (
	"strings"

	"github.com/compozy/compozy/engine/core"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

const (
	keyID           = "id"
	keyResource     = "resource"
	keyURL          = "url"
	keyCommand      = "command"
	keyProto        = "proto"
	keyTransport    = "transport"
	keyHeaders      = "headers"
	keyEnv          = "env"
	keyStartTimeout = "start_timeout"
	keyMaxSessions  = "max_sessions"
)

func getTrimmedString(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// ConvertRegisterMCPsFromMaps converts generic maps into strongly-typed Config values.
// Invalid entries are skipped; each resulting Config is validated.
func ConvertRegisterMCPsFromMaps(raw []map[string]any) []Config {
	out := make([]Config, 0, len(raw))
	for _, r := range raw {
		var cfg Config
		if v := getTrimmedString(r, keyID); v != "" {
			cfg.ID = v
		}
		if v := getTrimmedString(r, keyResource); v != "" {
			cfg.Resource = v
		}
		if v := getTrimmedString(r, keyURL); v != "" {
			cfg.URL = v
		}
		if v := getTrimmedString(r, keyCommand); v != "" {
			cfg.Command = v
		}
		if v := getTrimmedString(r, keyProto); v != "" {
			cfg.Proto = v
		}
		if v := getTrimmedString(r, keyTransport); v != "" {
			cfg.Transport = transportFromString(v)
		}
		cfg.Headers = core.ToStringMap(r[keyHeaders])
		cfg.Env = core.ToStringMap(r[keyEnv])
		if d, ok := core.ParseAnyDuration(r[keyStartTimeout]); ok {
			cfg.StartTimeout = d
		}
		if ms, ok := core.ParseAnyInt(r[keyMaxSessions]); ok {
			cfg.MaxSessions = ms
		}
		cfg.SetDefaults()
		if err := cfg.Validate(); err != nil {
			continue
		}
		out = append(out, cfg)
	}
	return out
}

func transportFromString(s string) mcpproxy.TransportType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "stdio":
		return mcpproxy.TransportStdio
	case "sse":
		return mcpproxy.TransportSSE
	case "http":
		return mcpproxy.TransportSSE
	case "streamable-http":
		return mcpproxy.TransportStreamableHTTP
	default:
		return mcpproxy.TransportSSE
	}
}
