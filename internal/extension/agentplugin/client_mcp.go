package agentplugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
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
		spec, diagnostics, err := decodeClientServer(name, servers[name], root, dataDir)
		pkg.Diagnostics = append(pkg.Diagnostics, diagnostics...)
		if err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: "mcp:" + name, Message: err.Error()})
			continue
		}
		pkg.Servers = append(pkg.Servers, spec)
		deriveClientInputs(pkg, spec)
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

func decodeClientServer(name string, raw json.RawMessage, root, dataDir string) (ServerSpec, []Diagnostic, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return ServerSpec{}, nil, errors.New("invalid client MCP server entry")
	}
	transport, ok := stringField(fields, "type", false)
	if !ok {
		return ServerSpec{}, nil, errors.New("invalid client MCP transport")
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
		return ServerSpec{}, nil, fmt.Errorf("encode client MCP transport: %w", err)
	}
	fields["type"] = encodedTransport
	auth, authDiagnostics := decodeClientAuth(name, fields["auth"], transport)
	delete(fields, "auth")
	allowed := []string{"type", "command", "args", "env", "cwd", "url", "headers"}
	diagnostics := authDiagnostics
	for _, field := range slices.Sorted(maps.Keys(fields)) {
		if !slices.Contains(allowed, field) {
			delete(fields, field)
			diagnostics = append(
				diagnostics,
				Diagnostic{
					Code:    ClientFieldIgnored,
					Scope:   "mcp:" + name,
					Message: fmt.Sprintf("%q is not supported and was ignored", field),
				},
			)
		}
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return ServerSpec{}, nil, fmt.Errorf("normalize client MCP server: %w", err)
	}
	var spec ServerSpec
	if transport == transportStreamableHTTP || transport == transportSSE {
		spec, err = decodeRemoteServer(name, transport, fields, true)
	} else {
		spec, err = decodeServer(name, normalized, root, dataDir)
	}
	spec.Auth = auth
	return spec, diagnostics, err
}

func discoverClientMCP(root string, pkg *Package) json.RawMessage {
	selected := ""
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		_, err := os.Lstat(filepath.Join(root, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: "cannot inspect " + name})
			return nil
		}
		if selected != "" {
			pkg.Diagnostics = append(
				pkg.Diagnostics,
				Diagnostic{Scope: scopeMCP, Message: name + " ignored because " + selected + " takes precedence"},
			)
			continue
		}
		selected = name
	}
	if selected == "" {
		return nil
	}
	raw, err := json.Marshal(selected)
	if err != nil {
		return nil
	}
	// Report discovery only after securely reading and decoding the selected file.
	servers, err := clientMCPServers(root, raw)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: selected + ": " + err.Error()})
		return nil
	}
	raw, err = json.Marshal(servers)
	if err != nil {
		pkg.Diagnostics = append(pkg.Diagnostics, Diagnostic{Scope: scopeMCP, Message: "cannot normalize " + selected})
		return nil
	}
	pkg.Diagnostics = append(
		pkg.Diagnostics,
		Diagnostic{Code: ClientMCPDiscovered, Scope: scopeMCP, Message: selected + " loaded from the plugin root"},
	)
	return raw
}
