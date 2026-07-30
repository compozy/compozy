package config

// MCPConfig controls global MCP client behavior.
type MCPConfig struct {
	OAuth MCPOAuthConfig `toml:"oauth"`
}

// MCPOAuthConfig controls the Compozy MCP OAuth client identity.
type MCPOAuthConfig struct {
	ClientMetadataURL string `toml:"client_metadata_url,omitempty"`
	RedirectURI       string `toml:"redirect_uri,omitempty"`
}
