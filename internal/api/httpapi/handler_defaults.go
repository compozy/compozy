package httpapi

import "strings"

func handlerBoundHost(cfg *handlerConfig) string {
	boundHost := strings.TrimSpace(cfg.boundHost)
	if boundHost == "" {
		boundHost = strings.TrimSpace(cfg.config.HTTP.Host)
	}
	if boundHost == "" {
		return handlersLocalhostKey
	}
	return boundHost
}
