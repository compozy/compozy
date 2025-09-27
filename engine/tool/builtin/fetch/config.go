package fetch

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

const (
	defaultTimeout      = 5 * time.Second
	defaultMaxBodyBytes = 2 << 20
	defaultRedirects    = 5
)

func loadToolConfig(ctx context.Context) toolConfig {
	cfg := config.DefaultNativeToolsConfig()
	if appCfg := config.FromContext(ctx); appCfg != nil {
		cfg = appCfg.Runtime.NativeTools
	}
	log := logger.FromContext(ctx)
	timeout := cfg.Fetch.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	maxBody := cfg.Fetch.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = defaultMaxBodyBytes
	}
	redirects := cfg.Fetch.MaxRedirects
	if redirects <= 0 {
		redirects = defaultRedirects
	}
	methods := cfg.Fetch.AllowedMethods
	allowed := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		upper := strings.ToUpper(strings.TrimSpace(method))
		if upper == "" {
			continue
		}
		allowed[upper] = struct{}{}
	}
	if len(allowed) == 0 {
		if methods == nil {
			log.Warn("Fetch allowed methods configuration empty; using defaults")
			for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"} {
				allowed[method] = struct{}{}
			}
		} else {
			log.Warn("Fetch allowed methods configuration produced no valid entries; fetch tool will reject all requests")
		}
	}
	return toolConfig{
		Timeout:        timeout,
		MaxBodyBytes:   maxBody,
		MaxRedirects:   redirects,
		AllowedMethods: allowed,
	}
}
