package runtimemcp

import (
	"encoding/json/v2"
	"errors"
	"maps"
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	shellquote "github.com/kballard/go-shellquote"

	"github.com/compozy/compozy/internal/definitiondigest"
)

type object = map[string]any

func obj(value any) object {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return nil
}
func str(value any) string {
	if result, ok := value.(string); ok {
		return result
	}
	return ""
}
func boolean(value any) bool {
	if result, ok := value.(bool); ok {
		return result
	}
	return false
}
func has(value object, key string) bool { v, ok := value[key]; return ok && v != nil }
func names(value any) []string {
	result := []string{}
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case []string:
		result = append(result, v...)
	case map[string]any:
		result = slices.Sorted(maps.Keys(v))
	}
	return result
}
func keys(value any) []string              { return slices.Sorted(maps.Keys(obj(value))) }
func contains(value any, name string) bool { return slices.Contains(names(value), name) }

func digest(value any, key []byte) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	// JSON map ordering is canonicalized before hashing; raw values never enter the snapshot.
	var canonical any
	if err := json.Unmarshal(data, &canonical); err != nil {
		return ""
	}
	data, err = json.Marshal(canonical, json.Deterministic(true))
	if err != nil {
		return ""
	}
	valueDigest, err := definitiondigest.Sum(data, key)
	if err != nil {
		return ""
	}
	return valueDigest
}

func decodeServers(runtime string, doc object) (object, error) {
	key := "mcpServers"
	if runtime == runtimeCodex {
		key = "mcp_servers"
	}
	if runtime == runtimeOpenCode {
		key = "mcp"
		if m := obj(doc[key]); has(m, "servers") {
			return checkedObject(m["servers"])
		}
	}
	if !has(doc, key) {
		return object{}, nil
	}
	return checkedObject(doc[key])
}

func checkedObject(value any) (object, error) {
	m := obj(value)
	if m == nil {
		return nil, errors.New("invalid MCP configuration shape")
	}
	return m, nil
}

func describeServer(runtime, name, scope, path, label string, definition object, key []byte) (Server, error) {
	if runtime == runtimeCodex && has(definition, fieldCommand) && has(definition, fieldURL) {
		return Server{}, errors.New("conflicting MCP transports")
	}
	transport, err := serverTransport(runtime, definition)
	if err != nil {
		return Server{}, err
	}
	env := obj(definition[fieldEnv])
	if env == nil {
		env = obj(definition[fieldEnvironment])
	}
	headers := obj(definition[fieldHeaders])
	if headers == nil {
		headers = obj(definition[fieldHTTPHeaders])
	}
	if headers == nil {
		headers = obj(definition[fieldHeader])
	}
	enabled := true
	if v, ok := definition["enabled"].(bool); ok {
		enabled = v
	}
	if boolean(definition["disabled"]) {
		enabled = false
	}
	s := Server{
		Runtime:     runtime,
		Name:        name,
		Scope:       scope,
		Transport:   transport,
		SourcePath:  path,
		SourceLabel: label,
		Target: serverTarget(
			transport,
			definition,
		),
		EnabledInRuntime: enabled,
		Approval:         "not_required",
		Auth:             "unknown",
		// The HTTP wire encodes nil slices as null; clients read these lists as arrays.
		EnvKeys:    append([]string{}, keys(env)...),
		HeaderKeys: append([]string{}, keys(headers)...),
	}
	s.EnvKeys = append(s.EnvKeys, names(definition["env_vars"])...)
	for _, value := range obj(definition["env_http_headers"]) {
		if name := str(value); name != "" {
			s.EnvKeys = append(s.EnvKeys, name)
		}
	}
	if name := str(definition["bearer_token_env_var"]); name != "" {
		s.EnvKeys = append(s.EnvKeys, name)
	}
	s.HeaderKeys = append(s.HeaderKeys, keys(definition["env_http_headers"])...)
	slices.Sort(s.EnvKeys)
	s.EnvKeys = slices.Compact(s.EnvKeys)
	slices.Sort(s.HeaderKeys)
	s.HeaderKeys = slices.Compact(s.HeaderKeys)
	s.CopyBlockedReason = copyBlocked(s, definition)
	s.Copyable = s.CopyBlockedReason == ""
	s.DefinitionDigest = digest(definition, key)
	return s, nil
}

func serverTransport(runtime string, definition object) (string, error) {
	transport := str(definition[fieldType])
	switch transport {
	case "", scopeLocal:
		switch {
		case has(definition, fieldHTTPURL):
			transport = transportHTTP
		case has(definition, fieldURL):
			transport = transportHTTP
			if runtime == runtimeGemini || runtime == runtimeQwenCode {
				transport = "sse"
			}
		case has(definition, fieldCommand):
			transport = transportStdio
		default:
			return "", errors.New("invalid MCP server transport")
		}
	case "remote", "streamable-http", "streamable_http":
		transport = transportHTTP
	}
	if transport == transportStdio &&
		(str(definition[fieldCommand]) == "" && len(names(definition[fieldCommand])) == 0) {
		return "", errors.New("missing MCP command")
	}
	if transport == transportHTTP && !has(definition, fieldURL) && !has(definition, fieldHTTPURL) {
		return "", errors.New("missing MCP URL")
	}
	return transport, nil
}

func serverTarget(transport string, definition object) string {
	if transport == transportStdio {
		if parts := names(definition[fieldCommand]); len(parts) > 0 {
			return filepath.Base(strings.ReplaceAll(parts[0], "\\", "/"))
		}
		command := str(definition[fieldCommand])
		parts, err := shellquote.Split(strings.ReplaceAll(command, "\\", "/"))
		if err != nil || len(parts) == 0 {
			return ""
		}
		return filepath.Base(strings.ReplaceAll(parts[0], "\\", "/"))
	}
	raw := str(definition[fieldURL])
	if raw == "" {
		raw = str(definition[fieldHTTPURL])
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
