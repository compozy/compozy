package config

// MCPServer describes an MCP server passed through to the agent runtime.
type MCPServer struct {
	Name           string             `json:"name"                      yaml:"name"                      toml:"name"`
	Transport      MCPServerTransport `json:"transport,omitempty"       yaml:"transport,omitempty"       toml:"transport,omitempty"`
	Command        string             `json:"command,omitempty"         yaml:"command,omitempty"         toml:"command,omitempty"`
	Args           []string           `json:"args,omitempty"            yaml:"args,omitempty"            toml:"args,omitempty"`
	Env            map[string]string  `json:"env,omitempty"             yaml:"env,omitempty"             toml:"env,omitempty"`
	SecretEnv      map[string]string  `json:"secret_env,omitempty"      yaml:"secret_env,omitempty"      toml:"secret_env,omitempty"`
	URL            string             `json:"url,omitempty"             yaml:"url,omitempty"             toml:"url,omitempty"`
	Auth           MCPAuthConfig      `json:"auth"                      yaml:"auth,omitempty"            toml:"auth,omitempty"`
	CatalogEntry   string             `json:"catalog_entry,omitempty"   yaml:"catalog_entry,omitempty"   toml:"catalog_entry,omitempty"`
	CatalogVersion string             `json:"catalog_version,omitempty" yaml:"catalog_version,omitempty" toml:"catalog_version,omitempty"`
}
