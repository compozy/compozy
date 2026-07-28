package config

func (o providerOverlay) Apply(dst *ProviderConfig) {
	if o.Command != nil {
		dst.Command = *o.Command
	}
	if o.DisplayName != nil {
		dst.DisplayName = *o.DisplayName
	}
	if o.Models != nil {
		o.Models.Apply(&dst.Models)
	}
	if o.Harness != nil {
		dst.Harness = *o.Harness
	}
	if o.RuntimeProvider != nil {
		dst.RuntimeProvider = *o.RuntimeProvider
	}
	if o.Transport != nil {
		dst.Transport = *o.Transport
	}
	if o.BaseURL != nil {
		dst.BaseURL = *o.BaseURL
	}
	if o.AuthMode != nil {
		dst.AuthMode = *o.AuthMode
	}
	if o.EnvPolicy != nil {
		dst.EnvPolicy = *o.EnvPolicy
	}
	if o.HomePolicy != nil {
		dst.HomePolicy = *o.HomePolicy
	}
	if o.NoneSecurity != nil {
		dst.NoneSecurity = *o.NoneSecurity
	}
	if o.AuthStatusCmd != nil {
		dst.AuthStatusCmd = *o.AuthStatusCmd
	}
	if o.AuthLoginCmd != nil {
		dst.AuthLoginCmd = *o.AuthLoginCmd
	}
	if o.SessionMCP != nil {
		dst.SessionMCP = new(*o.SessionMCP)
	}
	if len(o.CredentialSlots) > 0 {
		dst.CredentialSlots = applyProviderCredentialOverlays(o.CredentialSlots)
	}
	if len(o.MCPServers) > 0 {
		dst.MCPServers = applyMCPServerOverlays(dst.MCPServers, o.MCPServers)
	}
}

func (o providerModelsOverlay) Apply(dst *ProviderModelsConfig) {
	if o.Default != nil {
		dst.Default = *o.Default
	}
	if o.Curated != nil {
		dst.Curated = cloneProviderModelConfigs(o.Curated)
	}
	o.Discovery.Apply(&dst.Discovery)
	o.Reasoning.applyTo(&dst.Reasoning)
}

func (o modelCatalogOverlay) Apply(dst *ModelCatalogConfig) {
	o.Sources.Apply(&dst.Sources)
}

func (o modelCatalogSourcesOverlay) Apply(dst *ModelCatalogSourcesConfig) {
	o.ModelsDev.Apply(&dst.ModelsDev)
}

func (o modelsDevSourceOverlay) Apply(dst *ModelsDevSourceConfig) {
	if o.Enabled != nil {
		dst.Enabled = new(*o.Enabled)
	}
	if o.Endpoint != nil {
		dst.Endpoint = *o.Endpoint
	}
	if o.TTL != nil {
		dst.TTL = *o.TTL
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
}

func applyProviderCredentialOverlays(overlays []providerCredentialOverlay) []ProviderCredentialSlot {
	slots := make([]ProviderCredentialSlot, 0, len(overlays))
	for _, overlay := range overlays {
		var slot ProviderCredentialSlot
		if overlay.Name != nil {
			slot.Name = *overlay.Name
		}
		if overlay.TargetEnv != nil {
			slot.TargetEnv = *overlay.TargetEnv
		}
		if overlay.SecretRef != nil {
			slot.SecretRef = *overlay.SecretRef
		}
		if overlay.Kind != nil {
			slot.Kind = *overlay.Kind
		}
		if overlay.Required != nil {
			slot.Required = *overlay.Required
		}
		slots = append(slots, slot)
	}
	return slots
}
