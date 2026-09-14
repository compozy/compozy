package agentplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
)

func loadClientMCP(root, dataDir string, raw json.RawMessage, pkg *Package) {
	if len(raw) == 0 {
		return
	}
	servers, err := clientMCPServers(root, raw)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: err.Error()})
		return
	}
	for _, name := range slices.Sorted(maps.Keys(servers)) {
		spec, err := decodeClientServer(name, servers[name], root, dataDir)
		if err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: "mcp:" + name, Message: err.Error()})
			continue
		}
		pkg.Servers = append(pkg.Servers, spec)
	}
}

func clientMCPServers(root string, raw json.RawMessage) (map[string]json.RawMessage, error) {
	var path string
	if json.Unmarshal(raw, &path) == nil {
		resolved, err := resolveContained(filepath.FromSlash(path), root)
		if err != nil {
			return nil, errors.New("mcpServers path must remain inside package root")
		}
		contents, _, err := fileutil.ReadRegularFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("read client MCP configuration: %w", err)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(contents, &document); err != nil || document == nil {
			return nil, errors.New("client MCP configuration must be an object")
		}
		raw = document[fieldMCPServers]
	}
	var servers map[string]json.RawMessage
	if json.Unmarshal(raw, &servers) != nil || servers == nil {
		return nil, errors.New("mcpServers must be a contained file path or an object")
	}
	return servers, nil
}

func decodeClientServer(name string, raw json.RawMessage, root, dataDir string) (ServerSpec, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return ServerSpec{}, errors.New("invalid client MCP server entry")
	}
	transport, ok := stringField(fields, "type", false)
	if !ok {
		return ServerSpec{}, errors.New("invalid client MCP transport")
	}
	if transport == "" {
		if _, present := fields["command"]; present {
			transport = transportStdio
		} else {
			transport = transportStreamableHTTP
		}
	}
	if transport == "http" {
		transport = transportStreamableHTTP
	}
	encodedTransport, err := json.Marshal(transport)
	if err != nil {
		return ServerSpec{}, fmt.Errorf("encode client MCP transport: %w", err)
	}
	fields["type"] = encodedTransport
	normalized, err := json.Marshal(fields)
	if err != nil {
		return ServerSpec{}, fmt.Errorf("normalize client MCP server: %w", err)
	}
	return decodeServer(name, normalized, root, dataDir)
}
