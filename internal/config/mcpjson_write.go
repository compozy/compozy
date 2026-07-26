package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
)

// PutMCPSidecarServer upserts one MCP server definition in the selected sidecar
// target and returns the merged effective config after validation.
func PutMCPSidecarServer(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	server MCPServer,
) (Config, error) {
	if !target.isMCPSidecarTarget() {
		return Config{}, fmt.Errorf("config: write target %q is not an MCP sidecar", target.Kind())
	}

	normalized := cloneMCPServer(server)
	normalized.Name = normalizeMCPServerName(normalized.Name)
	if err := normalized.Validate("mcp_server"); err != nil {
		return Config{}, fmt.Errorf("config: validate MCP sidecar write: %w", err)
	}

	contents, _, err := readOptionalRegularFile(target.path, "MCP sidecar")
	if err != nil {
		return Config{}, err
	}

	document, err := loadEditableMCPJSONDocument(contents, target.path)
	if err != nil {
		return Config{}, err
	}
	if err := document.Put(normalized); err != nil {
		return Config{}, err
	}

	rendered, err := document.Bytes()
	if err != nil {
		return Config{}, err
	}

	finalCfg, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, rendered)
	if err != nil {
		return Config{}, err
	}
	if err := writePersistedFile(target.path, rendered); err != nil {
		return Config{}, err
	}
	return finalCfg, nil
}

// DeleteMCPSidecarServer removes one MCP server definition from the selected
// sidecar target when present and returns the merged effective config after
// validation.
func DeleteMCPSidecarServer(
	homePaths HomePaths,
	workspaceRoot string,
	target WriteTarget,
	name string,
) (Config, bool, error) {
	if !target.isMCPSidecarTarget() {
		return Config{}, false, fmt.Errorf("config: write target %q is not an MCP sidecar", target.Kind())
	}

	trimmedName := normalizeMCPServerName(name)
	if trimmedName == "" {
		return Config{}, false, errors.New("config: MCP sidecar delete requires a server name")
	}

	contents, _, err := readOptionalRegularFile(target.path, "MCP sidecar")
	if err != nil {
		return Config{}, false, err
	}

	document, err := loadEditableMCPJSONDocument(contents, target.path)
	if err != nil {
		return Config{}, false, err
	}
	deleted := document.Delete(trimmedName)
	if !deleted {
		cfg, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, contents)
		return cfg, false, err
	}

	rendered, err := document.Bytes()
	if err != nil {
		return Config{}, false, err
	}

	finalCfg, err := validateEffectiveConfigWrite(homePaths, workspaceRoot, target, rendered)
	if err != nil {
		return Config{}, false, err
	}
	if err := writePersistedFile(target.path, rendered); err != nil {
		return Config{}, false, err
	}
	return finalCfg, true, nil
}

type editableMCPJSONDocument struct {
	source string
	root   map[string]json.RawMessage
	camel  mcpJSONCollection
	snake  mcpJSONCollection
}

type mcpJSONCollection struct {
	key       string
	entries   map[string]json.RawMessage
	nameIndex map[string]string
	present   bool
}

func loadEditableMCPJSONDocument(content []byte, source string) (*editableMCPJSONDocument, error) {
	trimmedSource := strings.TrimSpace(source)
	if trimmedSource == "" {
		trimmedSource = MCPJSONName
	}

	document := &editableMCPJSONDocument{
		source: trimmedSource,
		root:   make(map[string]json.RawMessage),
		camel:  newMCPJSONCollection("mcpServers"),
		snake:  newMCPJSONCollection("mcp_servers"),
	}

	if len(bytes.TrimSpace(content)) == 0 {
		return document, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(content))
	var root map[string]json.RawMessage
	if err := decoder.Decode(&root); err != nil {
		return nil, fmt.Errorf("config: decode MCP JSON %q: %w", trimmedSource, err)
	}
	if err := ensureJSONEOF(decoder, trimmedSource); err != nil {
		return nil, err
	}
	document.root = root

	existingNames := make(map[string]string)
	var err error
	document.camel, err = loadMCPJSONCollection(root, "mcpServers", trimmedSource, existingNames)
	if err != nil {
		return nil, err
	}
	document.snake, err = loadMCPJSONCollection(root, "mcp_servers", trimmedSource, existingNames)
	if err != nil {
		return nil, err
	}

	return document, nil
}

func newMCPJSONCollection(key string) mcpJSONCollection {
	return mcpJSONCollection{
		key:       key,
		entries:   make(map[string]json.RawMessage),
		nameIndex: make(map[string]string),
	}
}

func loadMCPJSONCollection(
	root map[string]json.RawMessage,
	key string,
	source string,
	existingNames map[string]string,
) (mcpJSONCollection, error) {
	collection := newMCPJSONCollection(key)
	raw, ok := root[key]
	if !ok {
		return collection, nil
	}
	collection.present = true
	if len(bytes.TrimSpace(raw)) == 0 {
		return collection, nil
	}

	if err := json.Unmarshal(raw, &collection.entries); err != nil {
		return collection, fmt.Errorf("config: decode MCP JSON %q %q: %w", source, key, err)
	}
	for actualName := range collection.entries {
		normalized := normalizeMCPServerName(actualName)
		if normalized == "" {
			continue
		}
		if prior, ok := collection.nameIndex[normalized]; ok {
			return collection, fmt.Errorf(
				"config: decode MCP JSON %q %q: duplicate MCP server name %q in %q and %q",
				source,
				key,
				normalized,
				prior,
				actualName,
			)
		}
		if prior, ok := existingNames[normalized]; ok {
			return collection, fmt.Errorf(
				"config: decode MCP JSON %q: duplicate MCP server name %q across top-level collections in %q and %q",
				source,
				normalized,
				prior,
				actualName,
			)
		}
		collection.nameIndex[normalized] = actualName
		existingNames[normalized] = actualName
	}

	return collection, nil
}

func (d *editableMCPJSONDocument) Put(server MCPServer) error {
	collection := d.collectionForPut(server.Name)
	actualName := strings.TrimSpace(server.Name)
	if existingName, ok := collection.nameIndex[server.Name]; ok {
		actualName = existingName
	}

	rawServer, err := marshalMCPJSONServer(server)
	if err != nil {
		return err
	}

	collection.present = true
	collection.entries[actualName] = rawServer
	collection.nameIndex[server.Name] = actualName
	d.setCollection(collection)
	return nil
}

func (d *editableMCPJSONDocument) Delete(name string) bool {
	deletedSnake := d.snake.delete(name)
	deletedCamel := d.camel.delete(name)
	return deletedSnake || deletedCamel
}

func (d *editableMCPJSONDocument) Bytes() ([]byte, error) {
	root := make(map[string]json.RawMessage, len(d.root)+2)
	maps.Copy(root, d.root)

	if d.camel.present {
		payload, err := json.Marshal(d.camel.entries)
		if err != nil {
			return nil, fmt.Errorf("config: encode MCP JSON %q %q: %w", d.source, d.camel.key, err)
		}
		root[d.camel.key] = payload
	}
	if d.snake.present {
		payload, err := json.Marshal(d.snake.entries)
		if err != nil {
			return nil, fmt.Errorf("config: encode MCP JSON %q %q: %w", d.source, d.snake.key, err)
		}
		root[d.snake.key] = payload
	}

	payload, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("config: encode MCP JSON %q: %w", d.source, err)
	}
	payload = append(payload, '\n')
	return payload, nil
}

func (d *editableMCPJSONDocument) collectionForPut(name string) mcpJSONCollection {
	normalized := normalizeMCPServerName(name)
	if _, ok := d.snake.nameIndex[normalized]; ok {
		return d.snake
	}
	if _, ok := d.camel.nameIndex[normalized]; ok {
		return d.camel
	}

	switch {
	case d.snake.present && !d.camel.present:
		return d.snake
	case d.camel.present && !d.snake.present:
		return d.camel
	case d.snake.present && d.camel.present:
		return d.snake
	default:
		collection := d.camel
		collection.present = true
		return collection
	}
}

func (d *editableMCPJSONDocument) setCollection(collection mcpJSONCollection) {
	switch collection.key {
	case d.snake.key:
		d.snake = collection
	default:
		d.camel = collection
	}
}

func (c *mcpJSONCollection) delete(name string) bool {
	normalized := normalizeMCPServerName(name)
	actualName, ok := c.nameIndex[normalized]
	if !ok {
		return false
	}
	delete(c.entries, actualName)
	delete(c.nameIndex, normalized)
	c.present = true
	return true
}

func marshalMCPJSONServer(server MCPServer) (json.RawMessage, error) {
	type writableMCPJSONServer struct {
		Transport      MCPServerTransport `json:"transport,omitempty"`
		Command        string             `json:"command,omitempty"`
		Args           []string           `json:"args,omitempty"`
		Env            map[string]string  `json:"env,omitempty"`
		SecretEnv      map[string]string  `json:"secret_env,omitempty"`
		URL            string             `json:"url,omitempty"`
		Auth           *MCPAuthConfig     `json:"auth,omitempty"`
		CatalogEntry   string             `json:"catalog_entry,omitempty"`
		CatalogVersion string             `json:"catalog_version,omitempty"`
	}
	var auth *MCPAuthConfig
	normalizedAuth := normalizeMCPAuthConfig(server.Auth)
	if !normalizedAuth.IsZero() {
		auth = &normalizedAuth
	}
	payload, err := json.Marshal(writableMCPJSONServer{
		Transport:      MCPServerTransport(strings.TrimSpace(string(server.Transport))),
		Command:        strings.TrimSpace(server.Command),
		Args:           append([]string(nil), server.Args...),
		Env:            mergeStringMaps(nil, server.Env),
		SecretEnv:      mergeStringMaps(nil, server.SecretEnv),
		URL:            strings.TrimSpace(server.URL),
		Auth:           auth,
		CatalogEntry:   strings.TrimSpace(server.CatalogEntry),
		CatalogVersion: strings.TrimSpace(server.CatalogVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("config: encode MCP server %q: %w", server.Name, err)
	}
	return payload, nil
}
