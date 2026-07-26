package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
)

const (
	// SkillResourceKind is the canonical desired-state resource kind for skill definitions.
	SkillResourceKind     resources.ResourceKind = "skill"
	skillResourceMaxBytes                        = 512 << 10
)

// SkillResourceSpec is the resource-backed metadata index for one parsed skill.
//
// The full SKILL.md body intentionally remains outside the resource record and
// is loaded through the skills package when callers need content.
type SkillResourceSpec struct {
	Name                   string              `json:"name"`
	Description            string              `json:"description"`
	Version                string              `json:"version,omitempty"`
	Metadata               map[string]any      `json:"metadata,omitempty"`
	Source                 string              `json:"source"`
	Dir                    string              `json:"dir,omitempty"`
	FilePath               string              `json:"file_path,omitempty"`
	Enabled                bool                `json:"enabled"`
	ActivationGates        ActivationGates     `json:"activation_gates,omitzero"`
	MCPServers             []MCPServerDecl     `json:"mcp_servers,omitempty"`
	Hooks                  []hookspkg.HookDecl `json:"hooks,omitempty"`
	Provenance             *Provenance         `json:"provenance,omitempty"`
	InstalledFrom          string              `json:"installed_from,omitempty"`
	InstalledFromBundle    string              `json:"installed_from_bundle,omitempty"`
	InstalledFromExtension string              `json:"installed_from_extension,omitempty"`
}

// NewResourceCodec builds the canonical skill resource codec.
func NewResourceCodec() (resources.KindCodec[SkillResourceSpec], error) {
	return resources.NewJSONCodec(SkillResourceKind, skillResourceMaxBytes, validateSkillResourceSpec)
}

// SkillToResourceSpec converts a parsed skill into its canonical resource spec.
func SkillToResourceSpec(skill *Skill) SkillResourceSpec {
	if skill == nil {
		return SkillResourceSpec{}
	}
	return SkillResourceSpec{
		Name:                   strings.TrimSpace(skill.Meta.Name),
		Description:            strings.TrimSpace(skill.Meta.Description),
		Version:                strings.TrimSpace(skill.Meta.Version),
		Metadata:               cloneMetadataMap(skill.Meta.Metadata),
		Source:                 skillSourceName(skill.Source),
		Dir:                    strings.TrimSpace(skill.Dir),
		FilePath:               strings.TrimSpace(skill.FilePath),
		Enabled:                skill.Enabled,
		ActivationGates:        cloneActivationGates(skill.ActivationGates),
		MCPServers:             cloneMCPServerDecls(skill.MCPServers),
		Hooks:                  cloneSkillHookDecls(skill.Hooks),
		Provenance:             cloneProvenance(skill.Provenance),
		InstalledFrom:          strings.TrimSpace(skill.InstalledFrom),
		InstalledFromBundle:    strings.TrimSpace(skill.InstalledFromBundle),
		InstalledFromExtension: strings.TrimSpace(skill.InstalledFromExtension),
	}
}

// SkillFromResourceSpec converts a canonical resource spec into the runtime skill shape.
func SkillFromResourceSpec(spec SkillResourceSpec) (*Skill, error) {
	source, err := skillSourceFromName(spec.Source)
	if err != nil {
		return nil, err
	}
	skill := &Skill{
		Meta: SkillMeta{
			Name:        strings.TrimSpace(spec.Name),
			Description: strings.TrimSpace(spec.Description),
			Version:     strings.TrimSpace(spec.Version),
			Metadata:    cloneMetadataMap(spec.Metadata),
		},
		Source:                 source,
		Dir:                    strings.TrimSpace(spec.Dir),
		FilePath:               strings.TrimSpace(spec.FilePath),
		Enabled:                spec.Enabled,
		ActivationGates:        cloneActivationGates(spec.ActivationGates),
		Activation:             SkillActivation{Active: true},
		MCPServers:             cloneMCPServerDecls(spec.MCPServers),
		Hooks:                  cloneSkillHookDecls(spec.Hooks),
		Provenance:             cloneProvenance(spec.Provenance),
		InstalledFrom:          strings.TrimSpace(spec.InstalledFrom),
		InstalledFromBundle:    strings.TrimSpace(spec.InstalledFromBundle),
		InstalledFromExtension: strings.TrimSpace(spec.InstalledFromExtension),
	}
	refreshSkillHookDecls(skill)
	return skill, nil
}

func validateSkillResourceSpec(
	_ context.Context,
	scope resources.ResourceScope,
	spec SkillResourceSpec,
) (SkillResourceSpec, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return SkillResourceSpec{}, err
	}
	activationGates, err := normalizeActivationGates(spec.ActivationGates)
	if err != nil {
		return SkillResourceSpec{}, fmt.Errorf("%w: %v", resources.ErrValidation, err)
	}

	normalized := SkillResourceSpec{
		Name:                   strings.TrimSpace(spec.Name),
		Description:            strings.TrimSpace(spec.Description),
		Version:                strings.TrimSpace(spec.Version),
		Metadata:               cloneMetadataMap(spec.Metadata),
		Source:                 strings.TrimSpace(spec.Source),
		Dir:                    strings.TrimSpace(spec.Dir),
		FilePath:               strings.TrimSpace(spec.FilePath),
		Enabled:                spec.Enabled,
		ActivationGates:        activationGates,
		MCPServers:             cloneMCPServerDecls(spec.MCPServers),
		Hooks:                  cloneSkillHookDecls(spec.Hooks),
		Provenance:             cloneProvenance(spec.Provenance),
		InstalledFrom:          strings.TrimSpace(spec.InstalledFrom),
		InstalledFromBundle:    strings.TrimSpace(spec.InstalledFromBundle),
		InstalledFromExtension: strings.TrimSpace(spec.InstalledFromExtension),
	}
	if normalized.Name == "" {
		return SkillResourceSpec{}, fmt.Errorf("%w: skill.name is required", resources.ErrValidation)
	}
	if normalized.Description == "" {
		return SkillResourceSpec{}, fmt.Errorf("%w: skill.description is required", resources.ErrValidation)
	}
	if _, err := skillSourceFromName(normalized.Source); err != nil {
		return SkillResourceSpec{}, fmt.Errorf("%w: %v", resources.ErrValidation, err)
	}
	for idx, server := range normalized.MCPServers {
		normalized.MCPServers[idx] = normalizeMCPServerDecl(server)
		if strings.TrimSpace(normalized.MCPServers[idx].Name) == "" {
			return SkillResourceSpec{}, fmt.Errorf(
				"%w: skill.mcp_servers[%d].name is required",
				resources.ErrValidation,
				idx,
			)
		}
		if strings.TrimSpace(normalized.MCPServers[idx].Command) == "" {
			return SkillResourceSpec{}, fmt.Errorf(
				"%w: skill.mcp_servers[%d].command is required",
				resources.ErrValidation,
				idx,
			)
		}
		if err := toConfigMCPServer(normalized.MCPServers[idx]).Validate("skill.mcp_servers"); err != nil {
			return SkillResourceSpec{}, fmt.Errorf("%w: skill.mcp_servers[%d]: %v", resources.ErrValidation, idx, err)
		}
	}
	for idx, hook := range normalized.Hooks {
		if err := hookspkg.ValidateHookDecl(hook); err != nil {
			return SkillResourceSpec{}, fmt.Errorf("%w: skill.hooks[%d]: %v", resources.ErrValidation, idx, err)
		}
	}
	if err := validateSkillResourceProvenance(normalized.Provenance); err != nil {
		return SkillResourceSpec{}, err
	}

	return normalized, nil
}

func validateSkillResourceProvenance(provenance *Provenance) error {
	if provenance == nil {
		return nil
	}
	if strings.TrimSpace(provenance.Hash) == "" {
		return fmt.Errorf("%w: skill.provenance.hash is required", resources.ErrValidation)
	}
	if strings.TrimSpace(provenance.Registry) == "" {
		return fmt.Errorf("%w: skill.provenance.registry is required", resources.ErrValidation)
	}
	if strings.TrimSpace(provenance.Slug) == "" {
		return fmt.Errorf("%w: skill.provenance.slug is required", resources.ErrValidation)
	}
	if strings.TrimSpace(provenance.Version) == "" {
		return fmt.Errorf("%w: skill.provenance.version is required", resources.ErrValidation)
	}
	if provenance.InstalledAt.IsZero() {
		provenance.InstalledAt = time.Time{}
	}
	return nil
}

func normalizeMCPServerDecl(decl MCPServerDecl) MCPServerDecl {
	normalized := MCPServerDecl{
		Name:      strings.TrimSpace(decl.Name),
		Command:   strings.TrimSpace(decl.Command),
		Args:      append([]string(nil), decl.Args...),
		Env:       cloneStringMap(decl.Env),
		SecretEnv: cloneStringMap(decl.SecretEnv),
	}
	for idx := range normalized.Args {
		normalized.Args[idx] = strings.TrimSpace(normalized.Args[idx])
	}
	if len(normalized.Env) > 0 {
		for key, value := range normalized.Env {
			trimmedKey := strings.TrimSpace(key)
			delete(normalized.Env, key)
			if trimmedKey == "" {
				continue
			}
			normalized.Env[trimmedKey] = strings.TrimSpace(value)
		}
		if len(normalized.Env) == 0 {
			normalized.Env = nil
		}
	}
	if len(normalized.SecretEnv) > 0 {
		for key, value := range normalized.SecretEnv {
			trimmedKey := strings.TrimSpace(key)
			delete(normalized.SecretEnv, key)
			if trimmedKey == "" {
				continue
			}
			normalized.SecretEnv[trimmedKey] = strings.TrimSpace(value)
		}
		if len(normalized.SecretEnv) == 0 {
			normalized.SecretEnv = nil
		}
	}
	return normalized
}

func skillSourceFromName(source string) (SkillSource, error) {
	switch strings.TrimSpace(source) {
	case skillSourceName(SourceBundled):
		return SourceBundled, nil
	case skillSourceName(SourceMarketplace):
		return SourceMarketplace, nil
	case skillSourceName(SourceUser):
		return SourceUser, nil
	case skillSourceName(SourceAdditional):
		return SourceAdditional, nil
	case skillSourceName(SourceWorkspace):
		return SourceWorkspace, nil
	case skillSourceName(SourceAgentLocal):
		return SourceAgentLocal, nil
	default:
		return 0, fmt.Errorf("skills: unsupported skill source %q", source)
	}
}

func cloneSkillHookDecls(src []hookspkg.HookDecl) []hookspkg.HookDecl {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]hookspkg.HookDecl, 0, len(src))
	for _, decl := range src {
		next := decl
		next.Args = append([]string(nil), decl.Args...)
		next.Env = cloneStringMap(decl.Env)
		next.SecretEnv = cloneStringMap(decl.SecretEnv)
		next.Metadata = cloneStringMap(decl.Metadata)
		if decl.Matcher.ToolReadOnly != nil {
			value := *decl.Matcher.ToolReadOnly
			next.Matcher.ToolReadOnly = &value
		}
		cloned = append(cloned, next)
	}
	return cloned
}
