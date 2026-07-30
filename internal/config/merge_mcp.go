package config

type mcpOverlay struct {
	OAuth mcpOAuthGlobalOverlay `toml:"oauth"`
}

type mcpOAuthGlobalOverlay struct {
	ClientMetadataURL *string `toml:"client_metadata_url"`
	RedirectURI       *string `toml:"redirect_uri"`
}

func (o mcpOverlay) Apply(dst *MCPConfig) {
	o.OAuth.Apply(&dst.OAuth)
}

func (o mcpOAuthGlobalOverlay) Apply(dst *MCPOAuthConfig) {
	if o.ClientMetadataURL != nil {
		dst.ClientMetadataURL = *o.ClientMetadataURL
	}
	if o.RedirectURI != nil {
		dst.RedirectURI = *o.RedirectURI
	}
}
