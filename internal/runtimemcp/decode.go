package runtimemcp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"maps"
	"slices"
)

type object = map[string]any

func obj(value any) object              { result, _ := value.(map[string]any); return result }
func str(value any) string              { result, _ := value.(string); return result }
func boolean(value any) bool            { result, _ := value.(bool); return result }
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

func digest(value any) string {
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
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func decodeServers(runtime string, doc object) (object, error) {
	key := "mcpServers"
	if runtime == "codex" {
		key = "mcp_servers"
	}
	if runtime == "opencode" {
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

func describeServer(runtime, name, scope, path, label string, definition object) (Server, error) {
	if runtime == "codex" && has(definition, "command") && has(definition, "url") {
		return Server{}, errors.New("conflicting MCP transports")
	}
	transport, err := serverTransport(runtime, definition)
	if err != nil {
		return Server{}, err
	}
	env := obj(definition["env"])
	if env == nil {
		env = obj(definition["environment"])
	}
	headers := obj(definition["headers"])
	if headers == nil {
		headers = obj(definition["http_headers"])
	}
	if headers == nil {
		headers = obj(definition["header"])
	}
	enabled := true
	if v, ok := definition["enabled"].(bool); ok {
		enabled = v
	}
	if boolean(definition["disabled"]) {
		enabled = false
	}
	s := Server{Runtime: runtime, Name: name, Scope: scope, Transport: transport, SourcePath: path, SourceLabel: label,
		EnabledInRuntime: enabled, Approval: "not_required", Auth: "unknown", EnvKeys: keys(env), HeaderKeys: keys(headers)}
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
	s.DefinitionDigest = digest(definition)
	return s, nil
}

func serverTransport(runtime string, definition object) (string, error) {
	transport := str(definition["type"])
	switch transport {
	case "", "local":
		switch {
		case has(definition, "httpUrl"):
			transport = "http"
		case has(definition, "url"):
			transport = "http"
			if runtime == "gemini" || runtime == "qwen-code" {
				transport = "sse"
			}
		case has(definition, "command"):
			transport = "stdio"
		default:
			return "", errors.New("invalid MCP server transport")
		}
	case "remote", "streamable-http", "streamable_http":
		transport = "http"
	}
	if transport == "stdio" && (str(definition["command"]) == "" && len(names(definition["command"])) == 0) {
		return "", errors.New("missing MCP command")
	}
	if transport == "http" && !has(definition, "url") && !has(definition, "httpUrl") {
		return "", errors.New("missing MCP URL")
	}
	return transport, nil
}
