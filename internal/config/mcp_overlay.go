package config

type mcpServerOverlay struct {
	Name           *string             `toml:"name"`
	Transport      *MCPServerTransport `toml:"transport"`
	Command        *string             `toml:"command"`
	Args           *[]string           `toml:"args"`
	Env            *map[string]string  `toml:"env"`
	SecretEnv      *map[string]string  `toml:"secret_env"`
	URL            *string             `toml:"url"`
	Auth           mcpAuthOverlay      `toml:"auth"`
	CatalogEntry   *string             `toml:"catalog_entry"`
	CatalogVersion *string             `toml:"catalog_version"`
}

func (o mcpServerOverlay) Apply(dst *MCPServer) {
	if o.Name != nil {
		dst.Name = *o.Name
	}
	if o.Transport != nil {
		dst.Transport = *o.Transport
	}
	if o.Command != nil {
		dst.Command = *o.Command
	}
	if o.Args != nil {
		dst.Args = append([]string(nil), (*o.Args)...)
	}
	if o.Env != nil {
		dst.Env = mergeStringMaps(dst.Env, *o.Env)
	}
	if o.SecretEnv != nil {
		dst.SecretEnv = mergeStringMaps(dst.SecretEnv, *o.SecretEnv)
	}
	if o.URL != nil {
		dst.URL = *o.URL
	}
	if o.CatalogEntry != nil {
		dst.CatalogEntry = *o.CatalogEntry
	}
	if o.CatalogVersion != nil {
		dst.CatalogVersion = *o.CatalogVersion
	}
	o.Auth.Apply(&dst.Auth)
}
