package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
)

// MCPDefinitionDigest hashes definitions before any Vault value is resolved.
func MCPDefinitionDigest(server MCPServer) (string, error) {
	server = normalizeMCPServerResourceSpec(server)
	definition := struct {
		Transport                              MCPServerTransport
		Command                                string
		Args                                   []string
		CWD                                    string
		URL                                    string
		Env, Headers, SecretEnv, SecretHeaders map[string]string
		ReadOnlyTools                          []string
		Reference                              bool
		Auth                                   MCPAuthConfig
		InputValues                            map[string]MCPInputValue
		CatalogEntry, CatalogVersion           string
	}{
		Transport: server.EffectiveTransport(), Command: server.Command, Args: server.Args,
		CWD: server.CWD, URL: server.URL, Env: server.Env, Headers: server.Headers,
		SecretEnv: server.SecretEnv, SecretHeaders: server.SecretHeaders,
		ReadOnlyTools: server.ReadOnlyTools, Reference: server.Reference, Auth: server.Auth,
		InputValues: server.InputValues, CatalogEntry: server.CatalogEntry, CatalogVersion: server.CatalogVersion,
	}
	data, err := json.Marshal(definition, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}
