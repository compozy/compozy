package config

import (
	"fmt"

	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
)

func (o skillsOverlay) Apply(dst *SkillsConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.DisabledSkills != nil {
		dst.DisabledSkills = append([]string(nil), (*o.DisabledSkills)...)
	}
	if o.PollInterval != nil {
		dst.PollInterval = *o.PollInterval
	}
	if o.AllowedMarketplaceMCP != nil {
		dst.AllowedMarketplaceMCP = append([]string(nil), (*o.AllowedMarketplaceMCP)...)
	}
	if o.AllowedMarketplaceHooks != nil {
		dst.AllowedMarketplaceHooks = append([]string(nil), (*o.AllowedMarketplaceHooks)...)
	}
	o.Marketplace.Apply(&dst.Marketplace)
}

func (o extensionsOverlay) Apply(dst *ExtensionsConfig) {
	o.Marketplace.Apply(&dst.Marketplace)
	o.Resources.Apply(&dst.Resources)
}

func (o extensionsResourcesOverlay) Apply(dst *ExtensionsResourcesConfig) {
	if o.AllowedKinds != nil {
		dst.AllowedKinds = append([]resources.ResourceKind(nil), (*o.AllowedKinds)...)
	}
	if o.MaxScope != nil {
		dst.MaxScope = *o.MaxScope
	}
	o.SnapshotRateLimit.Apply(&dst.SnapshotRateLimit)
	o.OperatorWriteRateLimit.Apply(&dst.OperatorWriteRateLimit)
}

func (o extensionsRateLimitOverlay) Apply(dst *ExtensionsResourceRateLimitConfig) {
	if o.Requests != nil {
		dst.Requests = *o.Requests
	}
	if o.Window != nil {
		dst.Window = *o.Window
	}
	if o.Queue != nil {
		dst.Queue = *o.Queue
	}
}

func (o autonomyOverlay) Apply(dst *AutonomyConfig) {
	if o.BlockRecurrenceLimit != nil {
		dst.BlockRecurrenceLimit = *o.BlockRecurrenceLimit
	}
	o.Scheduler.Apply(&dst.Scheduler)
}

func (o schedulerOverlay) Apply(dst *SchedulerConfig) {
	if o.FanOutAfter != nil {
		dst.FanOutAfter = *o.FanOutAfter
	}
	if o.SpawnAfter != nil {
		dst.SpawnAfter = *o.SpawnAfter
	}
	if o.EventAfter != nil {
		dst.EventAfter = *o.EventAfter
	}
	if o.NeedsAttentionAfter != nil {
		dst.NeedsAttentionAfter = *o.NeedsAttentionAfter
	}
	if o.MinQueuedAge != nil {
		dst.MinQueuedAge = *o.MinQueuedAge
	}
}

func (o marketplaceOverlay) Apply(dst *MarketplaceConfig) {
	if o.Registry != nil {
		dst.Registry = *o.Registry
	}
	if o.BaseURL != nil {
		dst.BaseURL = *o.BaseURL
	}
}

func (o hooksOverlay) Apply(dst *HooksConfig) error {
	if len(o.Declarations) == 0 {
		return nil
	}

	merged := cloneHookDecls(dst.Declarations)
	index := make(map[string]int, len(merged))
	for i, decl := range merged {
		if name := strings.TrimSpace(decl.Name); name != "" {
			index[name] = i
		}
	}

	for idx := range o.Declarations {
		raw := &o.Declarations[idx]
		decl, err := raw.toHookDecl(hookspkg.HookSourceConfig, "")
		if err != nil {
			return fmt.Errorf("hooks.declarations[%d]: %w", idx, err)
		}

		name := strings.TrimSpace(decl.Name)
		if existingIdx, ok := index[name]; ok && name != "" {
			merged[existingIdx] = decl
			continue
		}

		merged = append(merged, decl)
		if name != "" {
			index[name] = len(merged) - 1
		}
	}

	dst.Declarations = merged
	return nil
}

func (o mcpAuthOverlay) Apply(dst *MCPAuthConfig) {
	if o.Type != nil {
		dst.Type = *o.Type
	}
	if o.IssuerURL != nil {
		dst.IssuerURL = *o.IssuerURL
	}
	if o.MetadataURL != nil {
		dst.MetadataURL = *o.MetadataURL
	}
	if o.AuthorizationURL != nil {
		dst.AuthorizationURL = *o.AuthorizationURL
	}
	if o.TokenURL != nil {
		dst.TokenURL = *o.TokenURL
	}
	if o.RevocationURL != nil {
		dst.RevocationURL = *o.RevocationURL
	}
	if o.ClientID != nil {
		dst.ClientID = *o.ClientID
	}
	if o.ClientSecretRef != nil {
		dst.ClientSecretRef = *o.ClientSecretRef
	}
	if o.Scopes != nil {
		dst.Scopes = append([]string(nil), (*o.Scopes)...)
	}
}
