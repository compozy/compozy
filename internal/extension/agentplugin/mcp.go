package agentplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/mcppolicy"
)

func loadMCP(root string, dataDir string, manifestSchema string, pkg *Package) {
	path := filepath.Join(root, "mcp.json")
	info, err := os.Lstat(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: "cannot inspect mcp.json"})
		}
		return
	}
	if !info.Mode().IsRegular() {
		pkg.Diagnostics = append(
			pkg.Diagnostics,
			Diagnostic{Scope: scopeMCP, Message: "mcp.json must be a regular file"},
		)
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: "cannot read mcp.json"})
		return
	}
	servers, err := decodeMCPDocument(content, manifestSchema)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: err.Error()})
		return
	}
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		server, err := decodeServer(name, servers[name], root, dataDir)
		if err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: "mcp:" + name, Message: err.Error()})
			continue
		}
		pkg.Servers = append(pkg.Servers, server)
	}
}

func decodeMCPDocument(content []byte, manifestSchema string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(content, &fields); err != nil || fields == nil {
		return nil, errors.New("invalid mcp.json: root must be an object")
	}
	for field := range fields {
		if field != fieldSchema && field != fieldMCPServers {
			return nil, fmt.Errorf("invalid mcp.json: unknown field %q", field)
		}
	}
	var schema string
	if raw, exists := fields[fieldSchema]; !exists || isJSONNull(raw) || json.Unmarshal(raw, &schema) != nil {
		return nil, errors.New("invalid mcp.json: $schema must be a string")
	}
	if schema != MCPSchemaID {
		mcpVersion := schemaVersion(schema)
		if mcpVersion != "" && mcpVersion != schemaVersion(manifestSchema) {
			return nil, errors.New("mcp.json schema version must match plugin.json")
		}
		return nil, fmt.Errorf("unsupported mcp.json schema %q", schema)
	}
	rawServers, exists := fields[fieldMCPServers]
	if !exists || isJSONNull(rawServers) {
		return nil, errors.New("invalid mcp.json: mcpServers is required")
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(rawServers, &servers); err != nil || servers == nil {
		return nil, errors.New("invalid mcp.json: mcpServers must be an object")
	}
	return servers, nil
}

func decodeServer(name string, raw json.RawMessage, root string, dataDir string) (ServerSpec, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	transport, ok := stringField(fields, "type", true)
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	switch transport {
	case transportStdio:
		return decodeStdioServer(name, fields, root, dataDir)
	case transportStreamableHTTP, transportSSE:
		return decodeRemoteServer(name, transport, fields)
	default:
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
}

func decodeStdioServer(
	name string,
	fields map[string]json.RawMessage,
	root string,
	dataDir string,
) (ServerSpec, error) {
	if !onlyFields(fields, "type", "command", "args", "env", "cwd") {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	command, ok := stringField(fields, "command", true)
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	args, ok := stringSliceField(fields, "args")
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	env, ok := stringMapField(fields, "env")
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	cwd, ok := optionalStringField(fields, "cwd")
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	command, normalizedCWD, args, env, err := normalizeStdioPaths(root, dataDir, command, cwd, args, env)
	if err != nil {
		return ServerSpec{}, err
	}
	return ServerSpec{
		Name: name, Transport: transportStdio, Command: command, CWD: normalizedCWD, Args: args, Env: env,
	}, nil
}

func decodeRemoteServer(
	name string,
	transport string,
	fields map[string]json.RawMessage,
) (ServerSpec, error) {
	if transport == transportSSE {
		return ServerSpec{}, errors.New("sse transport is not supported")
	}
	if !onlyFields(fields, "type", "url", "headers") {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	url, ok := stringField(fields, fieldURL, true)
	if !ok || url == "" {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	headers, ok := stringMapField(fields, "headers")
	if !ok {
		return ServerSpec{}, errors.New("invalid mcp server entry")
	}
	if err := mcppolicy.ValidateRemoteURL(url); err != nil {
		return ServerSpec{}, err
	}
	if err := mcppolicy.ValidateHeaders(headers, nil, mcppolicy.SourcePackageFixed, false); err != nil {
		return ServerSpec{}, err
	}
	return ServerSpec{Name: name, Transport: transport, URL: url, Headers: headers}, nil
}

func onlyFields(fields map[string]json.RawMessage, allowed ...string) bool {
	set := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		set[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := set[field]; !ok {
			return false
		}
	}
	return true
}

func stringField(fields map[string]json.RawMessage, name string, required bool) (string, bool) {
	raw, exists := fields[name]
	if !exists {
		return "", !required
	}
	var value string
	if isJSONNull(raw) || json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func optionalStringField(fields map[string]json.RawMessage, name string) (*string, bool) {
	raw, exists := fields[name]
	if !exists {
		return nil, true
	}
	var value string
	if isJSONNull(raw) || json.Unmarshal(raw, &value) != nil {
		return nil, false
	}
	return &value, true
}

func stringSliceField(fields map[string]json.RawMessage, name string) ([]string, bool) {
	raw, exists := fields[name]
	if !exists {
		return nil, true
	}
	return decodeStringSlice(raw)
}

func stringMapField(fields map[string]json.RawMessage, name string) (map[string]string, bool) {
	raw, exists := fields[name]
	if !exists {
		return nil, true
	}
	if isJSONNull(raw) {
		return nil, false
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, false
	}
	values := make(map[string]string, len(entries))
	for key, entry := range entries {
		if isJSONNull(entry) {
			return nil, false
		}
		var value string
		if err := json.Unmarshal(entry, &value); err != nil {
			return nil, false
		}
		values[key] = value
	}
	return values, true
}

func schemaVersion(schema string) string {
	if !strings.HasPrefix(schema, schemaPrefix) {
		return ""
	}
	remainder := strings.TrimPrefix(schema, schemaPrefix)
	version, _, ok := strings.Cut(remainder, "/")
	if !ok {
		return ""
	}
	return version
}
