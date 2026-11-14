package runtimemcp

import (
	"strings"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcppolicy"
)

// Check shapes before conversion so malformed values and overlapping aliases cannot be dropped.
func representableCopyShape(transport string, d object) bool {
	for _, aliases := range [][]string{{"env", "environment"}, {"headers", "http_headers", "header"}, {"url", "httpUrl"}} {
		count := 0
		for _, key := range aliases {
			if has(d, key) {
				count++
			}
		}
		if count > 1 {
			return false
		}
	}
	for _, key := range []string{"env", "environment", "headers", "http_headers", "header"} {
		if !has(d, key) {
			continue
		}
		values := obj(d[key])
		if values == nil {
			return false
		}
		for name, value := range values {
			if _, ok := value.(string); !ok {
				return false
			}
			if (key == "env" || key == "environment") && config.ValidateMCPStdioEnvName("env", name, true) != nil {
				return false
			}
		}
	}
	if has(d, "args") && !stringArray(d["args"]) {
		return false
	}
	if has(d, "command") {
		if _, ok := d["command"].(string); !ok && !stringArray(d["command"]) {
			return false
		}
	}
	if transport == "stdio" {
		command := str(d["command"])
		if parts := names(d["command"]); len(parts) > 0 {
			command = parts[0]
		}
		return strings.TrimSpace(command) != "" && !has(d, "url") && !has(d, "httpUrl") &&
			!has(d, "headers") && !has(d, "http_headers") && !has(d, "header") && !has(d, "auth") && !has(d, "oauth")
	}
	for _, key := range []string{"url", "httpUrl"} {
		if has(d, key) && strings.TrimSpace(str(d[key])) == "" {
			return false
		}
	}
	if has(d, "command") || has(d, "args") || has(d, "env") || has(d, "environment") {
		return false
	}
	headers := literalMap(d, "headers", "http_headers", "header")
	return mcppolicy.ValidateHeaders(
		nil,
		headers,
		mcppolicy.SourceOperatorSecret,
		has(d, "auth") || has(d, "oauth"),
	) == nil
}

func stringArray(value any) bool {
	switch values := value.(type) {
	case []string:
		return true
	case []any:
		for _, value := range values {
			if _, ok := value.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}
