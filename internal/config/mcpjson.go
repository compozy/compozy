package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const (
	// MCPJSONName is the supported JSON sidecar filename for MCP server declarations.
	MCPJSONName = "mcp.json"
)

type mcpJSONDocument struct {
	MCPServersCamel map[string]mcpJSONServer `json:"mcpServers"`
	MCPServersSnake map[string]mcpJSONServer `json:"mcp_servers"`
}

type mcpJSONServer struct {
	Transport      MCPServerTransport `json:"transport,omitempty"`
	Command        string             `json:"command,omitempty"`
	Args           []string           `json:"args,omitempty"`
	Env            map[string]string  `json:"env,omitempty"`
	SecretEnv      map[string]string  `json:"secret_env,omitempty"`
	URL            string             `json:"url,omitempty"`
	Auth           MCPAuthConfig      `json:"auth"`
	CatalogEntry   string             `json:"catalog_entry,omitempty"`
	CatalogVersion string             `json:"catalog_version,omitempty"`
}

// ParseMCPServersJSON parses an MCP JSON document into canonical MCP server values.
// The document may use either `mcpServers` or `mcp_servers` as the top-level key.
func ParseMCPServersJSON(content []byte, source string) ([]MCPServer, error) {
	sourceName := strings.TrimSpace(source)
	if sourceName == "" {
		sourceName = MCPJSONName
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("config: decode MCP JSON %q: %w", sourceName, err)
	}
	if err := ensureJSONEOF(decoder, sourceName); err != nil {
		return nil, err
	}

	var document mcpJSONDocument
	if raw := root["mcpServers"]; len(raw) > 0 {
		if err := decodeStrictMCPJSONCollection(raw, &document.MCPServersCamel); err != nil {
			return nil, fmt.Errorf("config: decode MCP JSON %q mcpServers: %w", sourceName, err)
		}
	}
	if raw := root["mcp_servers"]; len(raw) > 0 {
		if err := decodeStrictMCPJSONCollection(raw, &document.MCPServersSnake); err != nil {
			return nil, fmt.Errorf("config: decode MCP JSON %q mcp_servers: %w", sourceName, err)
		}
	}

	servers := sortedMCPJSONServers(document.MCPServersCamel)
	servers = OverrideMCPServers(servers, sortedMCPJSONServers(document.MCPServersSnake))
	if err := validateUniqueMCPJSONServerNames(servers, sourceName); err != nil {
		return nil, err
	}
	for idx, server := range servers {
		if err := server.Validate(fmt.Sprintf("mcp.json %q[%d]", sourceName, idx)); err != nil {
			return nil, fmt.Errorf("config: validate MCP JSON %q: %w", sourceName, err)
		}
	}

	return servers, nil
}

// LoadMCPServersJSONFile parses an optional `mcp.json` file from disk.
// Missing files are treated as absent rather than as errors.
func LoadMCPServersJSONFile(path string) ([]MCPServer, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, nil
	}

	content, exists, err := readOptionalRegularFile(trimmed, "MCP JSON")
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	return ParseMCPServersJSON(content, trimmed)
}

func ensureJSONEOF(decoder *json.Decoder, source string) error {
	return ensureJSONDocumentEOF(decoder, source, "MCP JSON")
}

func decodeStrictMCPJSONCollection(raw json.RawMessage, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return ensureJSONEOF(decoder, "<nested>")
}

func sortedMCPJSONServers(values map[string]mcpJSONServer) []MCPServer {
	if len(values) == 0 {
		return nil
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]MCPServer, 0, len(names))
	for _, name := range names {
		entry := values[name]
		servers = append(servers, MCPServer{
			Name:           normalizeMCPServerName(name),
			Transport:      MCPServerTransport(strings.TrimSpace(string(entry.Transport))),
			Command:        strings.TrimSpace(entry.Command),
			Args:           append([]string(nil), entry.Args...),
			Env:            mergeStringMaps(nil, entry.Env),
			SecretEnv:      mergeStringMaps(nil, entry.SecretEnv),
			URL:            strings.TrimSpace(entry.URL),
			Auth:           normalizeMCPAuthConfig(entry.Auth),
			CatalogEntry:   strings.TrimSpace(entry.CatalogEntry),
			CatalogVersion: strings.TrimSpace(entry.CatalogVersion),
		})
	}

	return servers
}

func validateUniqueMCPJSONServerNames(servers []MCPServer, sourceName string) error {
	seen := make(map[string]int, len(servers))
	for idx, server := range servers {
		name := normalizeMCPServerName(server.Name)
		if name == "" {
			continue
		}
		if priorIdx, ok := seen[name]; ok {
			return fmt.Errorf(
				"config: validate MCP JSON %q: duplicate MCP server name %q after normalization at indexes %d and %d",
				sourceName,
				name,
				priorIdx,
				idx,
			)
		}
		seen[name] = idx
	}

	return nil
}
