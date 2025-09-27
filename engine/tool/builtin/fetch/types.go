package fetch

import "time"

const toolID = "cp__fetch"

type toolConfig struct {
	Timeout        time.Duration
	MaxBodyBytes   int64
	MaxRedirects   int
	AllowedMethods map[string]struct{}
}

type Args struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      any               `json:"body"`
	TimeoutMs int               `json:"timeout_ms"`
}
