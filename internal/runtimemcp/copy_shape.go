package runtimemcp

import (
	"strings"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcppolicy"
)

// Check shapes before conversion so malformed values and overlapping aliases cannot be dropped.
func representableCopyShape(transport string, d object) bool {
	if !representableAliases(d) || !representableMaps(d) {
		return false
	}
	if has(d, fieldArgs) && !stringArray(d[fieldArgs]) {
		return false
	}
	if has(d, fieldCommand) {
		if _, ok := d[fieldCommand].(string); !ok && !stringArray(d[fieldCommand]) {
			return false
		}
	}
	if transport == transportStdio {
		return representableStdioShape(d)
	}
	for _, key := range []string{fieldURL, fieldHTTPURL} {
		if has(d, key) && strings.TrimSpace(str(d[key])) == "" {
			return false
		}
	}
	if has(d, fieldCommand) || has(d, fieldArgs) || has(d, fieldEnv) || has(d, fieldEnvironment) {
		return false
	}
	headers := literalMap(d, fieldHeaders, fieldHTTPHeaders, fieldHeader)
	return mcppolicy.ValidateHeaders(
		nil,
		headers,
		mcppolicy.SourceOperatorSecret,
		has(d, fieldAuth) || has(d, authOAuth),
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

func representableAliases(d object) bool {
	for _, aliases := range [][]string{
		{fieldEnv, fieldEnvironment},
		{fieldHeaders, fieldHTTPHeaders, fieldHeader},
		{fieldURL, fieldHTTPURL},
	} {
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
	return true
}

func representableMaps(d object) bool {
	for _, key := range []string{fieldEnv, fieldEnvironment, fieldHeaders, fieldHTTPHeaders, fieldHeader} {
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
			if (key == fieldEnv || key == fieldEnvironment) &&
				config.ValidateMCPStdioEnvName(fieldEnv, name, true) != nil {
				return false
			}
		}
	}
	return true
}

func representableStdioShape(d object) bool {
	command := str(d[fieldCommand])
	if parts := names(d[fieldCommand]); len(parts) > 0 {
		command = parts[0]
	}
	return strings.TrimSpace(command) != "" && !has(d, fieldURL) && !has(d, fieldHTTPURL) &&
		!has(d, fieldHeaders) && !has(d, fieldHTTPHeaders) &&
		!has(d, fieldHeader) && !has(d, fieldAuth) && !has(d, authOAuth)
}
