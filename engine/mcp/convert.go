package mcp

import (
	"strings"

	"github.com/compozy/compozy/engine/core"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
)

// ConvertRegisterMCPsFromMaps converts generic maps into strongly-typed Config values.
// Invalid entries are skipped; each resulting Config is validated.
func ConvertRegisterMCPsFromMaps(raw []map[string]any) []Config {
	out := make([]Config, 0, len(raw))
	for i := range raw {
		r := raw[i]
		var cfg Config
		if v, ok := r["id"].(string); ok {
			cfg.ID = strings.TrimSpace(v)
		}
		if v, ok := r["resource"].(string); ok {
			cfg.Resource = strings.TrimSpace(v)
		}
		if v, ok := r["url"].(string); ok {
			cfg.URL = strings.TrimSpace(v)
		}
		if v, ok := r["command"].(string); ok {
			cfg.Command = strings.TrimSpace(v)
		}
		if v, ok := r["proto"].(string); ok {
			cfg.Proto = strings.TrimSpace(v)
		}
		if v, ok := r["transport"].(string); ok {
			cfg.Transport = transportFromString(v)
		}
		cfg.Headers = core.ToStringMap(r["headers"])
		cfg.Env = core.ToStringMap(r["env"])
		if d, ok := core.ParseAnyDuration(r["start_timeout"]); ok {
			cfg.StartTimeout = d
		}
		if ms, ok := core.ParseAnyInt(r["max_sessions"]); ok {
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
	case "streamable-http":
		return mcpproxy.TransportStreamableHTTP
	default:
		return mcpproxy.TransportSSE
	}
}
