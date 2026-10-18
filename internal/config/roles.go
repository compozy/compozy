package config

import (
	"fmt"
	"strings"

	"time"

	"github.com/compozy/compozy/internal/runtimeoption"

	"github.com/compozy/compozy/internal/reasoning"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

type providerResolver interface {
	ResolveProvider(name string) (ProviderConfig, error)
}

var _ providerResolver = (*Config)(nil)

// RoleName identifies a background runtime role governed by [roles].
type RoleName string

const (
	RoleCoordinator RoleName = "coordinator"
	RoleAutoTitle   RoleName = "auto_title"
)

var allRoleNames = []RoleName{
	RoleCoordinator,
	RoleAutoTitle,
}

// RoleNames returns the complete closed role roster.
func RoleNames() []RoleName {
	return append([]RoleName(nil), allRoleNames...)
}

// RolesConfig is the closed [roles] roster.
type RolesConfig struct {
	Coordinator CoordinatorRoleConfig `toml:"coordinator"`
	AutoTitle   RoleConfig            `toml:"auto_title"`
}

// CloneRolesConfig returns an ownership-safe copy of the roles configuration.
func CloneRolesConfig(source *RolesConfig) RolesConfig {
	if source == nil {
		return RolesConfig{}
	}
	cloned := *source
	cloned.Coordinator.ACPOptions = CloneACPOptionSelections(source.Coordinator.ACPOptions)
	cloned.AutoTitle.ACPOptions = CloneACPOptionSelections(source.AutoTitle.ACPOptions)
	cloned.Coordinator.FallbackChain = cloneRoleFallbacks(source.Coordinator.FallbackChain)
	cloned.AutoTitle.FallbackChain = cloneRoleFallbacks(source.AutoTitle.FallbackChain)
	return cloned
}

func cloneRoleFallbacks(source []RoleFallback) []RoleFallback {
	if len(source) == 0 {
		return nil
	}
	cloned := make([]RoleFallback, len(source))
	for index, fallback := range source {
		cloned[index] = fallback
		cloned[index].ACPOptions = CloneACPOptionSelections(fallback.ACPOptions)
	}
	return cloned
}

// RoleConfig controls one session-backed role.
type RoleConfig struct {
	Enabled         bool   `toml:"enabled"`
	Agent           string `toml:"agent,omitempty"`
	Provider        string `toml:"provider,omitempty"`
	Model           string `toml:"model,omitempty"`
	ReasoningEffort string `toml:"reasoning_effort,omitempty"`
	// Speed selects normal or Fast execution when the provider advertises it.
	Speed speedpkg.Speed `toml:"speed,omitempty"`
	// ACPOptions selects provider-advertised typed session configuration values.
	ACPOptions    []ACPOptionSelection `toml:"acp_options,omitempty"`
	FallbackChain []RoleFallback       `toml:"fallback_chain,omitempty"`
}

// RoleFallback is one ordered invocation fallback route.
type RoleFallback struct {
	Provider        string               `toml:"provider"                   json:"provider"                   yaml:"provider"`
	Model           string               `toml:"model"                      json:"model"                      yaml:"model"`
	ReasoningEffort string               `toml:"reasoning_effort,omitempty" json:"reasoning_effort,omitempty" yaml:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed       `toml:"speed,omitempty"            json:"speed,omitempty"            yaml:"speed,omitempty"`
	ACPOptions      []ACPOptionSelection `toml:"acp_options,omitempty"      json:"acp_options,omitempty"      yaml:"acp_options,omitempty"`
	// Command selects the account for this route exactly like providers.<name>.command
	// and agent.command: shell-style quoting, leading NAME=value assignments are private
	// environment forwarded literally, and no shell is launched. Empty inherits.
	Command string `toml:"command,omitempty" json:"command,omitempty" yaml:"command,omitempty"`
}

// CoordinatorRoleConfig combines routing with coordinator safety policy.
type CoordinatorRoleConfig struct {
	RoleConfig
	TTL                           time.Duration `toml:"ttl"`
	MaxChildren                   int           `toml:"max_children"`
	MaxActiveSessionsPerWorkspace int           `toml:"max_active_sessions_per_workspace"`
}

const (
	// DefaultCoordinatorTTL is the default TTL for [roles.coordinator].
	DefaultCoordinatorTTL = 2 * time.Hour
	// MinCoordinatorTTL is the shortest coordinator TTL accepted by config validation.
	MinCoordinatorTTL = time.Minute
	// MaxCoordinatorTTL is the longest coordinator TTL accepted by config validation.
	MaxCoordinatorTTL = 24 * time.Hour
	// DefaultCoordinatorMaxChildren is the safe per-coordinator child-session cap.
	DefaultCoordinatorMaxChildren = 5
	// MaxCoordinatorChildren is the hard MVP cap for coordinator child sessions.
	MaxCoordinatorChildren = 5
	// DefaultCoordinatorMaxActiveSessionsPerWorkspace caps managed coordinator and worker sessions.
	DefaultCoordinatorMaxActiveSessionsPerWorkspace = 5
)

// ResolvedCoordinatorRole is the coordinator runtime policy assembled from a resolved role.
type ResolvedCoordinatorRole struct {
	Enabled                       bool
	AgentName                     string
	Provider                      string
	Model                         string
	ReasoningEffort               string
	Speed                         speedpkg.Speed
	ACPOptions                    []ACPOptionSelection
	Fallbacks                     []RoleFallback
	TTL                           time.Duration
	MaxChildren                   int
	MaxActiveSessionsPerWorkspace int
}

// DefaultResolvedCoordinatorRole returns the coordinator's resolved default identity and policy.
func DefaultResolvedCoordinatorRole() ResolvedCoordinatorRole {
	defaults := DefaultRolesConfig().Coordinator
	return ResolvedCoordinatorRole{
		Enabled:                       defaults.Enabled,
		AgentName:                     BuiltinCoordinatorAgentName,
		Speed:                         normalizeAgentSpeed(defaults.Speed),
		ACPOptions:                    CloneACPOptionSelections(defaults.ACPOptions),
		Fallbacks:                     cloneRoleFallbacks(defaults.FallbackChain),
		TTL:                           defaults.TTL,
		MaxChildren:                   defaults.MaxChildren,
		MaxActiveSessionsPerWorkspace: defaults.MaxActiveSessionsPerWorkspace,
	}
}

// DefaultRolesConfig returns routing defaults for the closed role roster.
func DefaultRolesConfig() RolesConfig {
	return RolesConfig{
		Coordinator: CoordinatorRoleConfig{
			Enabled:                       false,
			TTL:                           DefaultCoordinatorTTL,
			MaxChildren:                   DefaultCoordinatorMaxChildren,
			MaxActiveSessionsPerWorkspace: DefaultCoordinatorMaxActiveSessionsPerWorkspace,
		},
		AutoTitle: RoleConfig{Enabled: true},
	}
}

// Validate checks role shape, bounds, and configured provider references.
func (c *RolesConfig) Validate(path string, resolver providerResolver) error {
	if c == nil {
		return fmt.Errorf("%s is required", path)
	}
	if err := c.Coordinator.validate(path+".coordinator", resolver); err != nil {
		return err
	}
	for _, role := range []struct {
		name   RoleName
		config RoleConfig
	}{
		{name: RoleAutoTitle, config: c.AutoTitle},
	} {
		if err := role.config.validate(path+"."+string(role.name), resolver); err != nil {
			return err
		}
	}
	return nil
}

func (c CoordinatorRoleConfig) validate(path string, resolver providerResolver) error {
	if err := c.RoleConfig.validate(path, resolver); err != nil {
		return err
	}
	if c.TTL < MinCoordinatorTTL || c.TTL > MaxCoordinatorTTL {
		return fmt.Errorf("%s.ttl must be between %s and %s: %s", path, MinCoordinatorTTL, MaxCoordinatorTTL, c.TTL)
	}
	if c.MaxChildren <= 0 {
		return fmt.Errorf("%s.max_children must be positive: %d", path, c.MaxChildren)
	}
	if c.MaxChildren > MaxCoordinatorChildren {
		return fmt.Errorf("%s.max_children must be <= %d: %d", path, MaxCoordinatorChildren, c.MaxChildren)
	}
	if c.MaxActiveSessionsPerWorkspace <= 0 {
		return fmt.Errorf(
			"%s.max_active_sessions_per_workspace must be positive: %d",
			path,
			c.MaxActiveSessionsPerWorkspace,
		)
	}
	return nil
}

func (c RoleConfig) validate(path string, resolver providerResolver) error {
	if strings.TrimSpace(c.Agent) != "" {
		if err := validateAgentNameAtPath(c.Agent, path+".agent"); err != nil {
			return err
		}
	}
	if err := validateRoleReasoningEffort(path+".reasoning_effort", c.ReasoningEffort); err != nil {
		return err
	}
	if err := validateAgentSpeed(c.Speed, path+".speed"); err != nil {
		return err
	}
	if err := validateRoleACPOptions(path+".acp_options", c.ACPOptions); err != nil {
		return err
	}
	if err := validateRoleACPOptionConflicts(
		path+".acp_options",
		c.ACPOptions,
		c.Speed,
		c.ReasoningEffort,
	); err != nil {
		return err
	}
	if err := validateRoleProvider(path, c.Provider, resolver); err != nil {
		return err
	}
	return validateRoleFallbacks(path, c.FallbackChain, resolver)
}

func validateRoleProvider(path, providerName string, resolver providerResolver) error {
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		return nil
	}
	if resolver == nil {
		return fmt.Errorf("%s.provider resolver is required", path)
	}
	_, err := resolver.ResolveProvider(providerName)
	if err != nil {
		return fmt.Errorf("%s.provider: %w", path, err)
	}
	return nil
}

func validateRoleFallbacks(path string, fallbacks []RoleFallback, resolver providerResolver) error {
	return validateFallbackChain(path+".fallback_chain", fallbacks, resolver, true)
}

func validateRoleACPOptions(path string, options []ACPOptionSelection) error {
	return validateACPOptionSelections(path, options)
}

func validateRoleACPOptionConflicts(
	path string,
	options []ACPOptionSelection,
	speed speedpkg.Speed,
	reasoningEffort string,
) error {
	for _, option := range options {
		semanticID := runtimeoption.SemanticID(option.ID)
		switch semanticID {
		case "fast", "speed", "speedmode":
			if strings.TrimSpace(string(speed)) != "" {
				return fmt.Errorf("%s.%s duplicates speed", path, option.ID)
			}
		case "reasoning", "reasoningeffort", "effort":
			if strings.TrimSpace(reasoningEffort) != "" {
				return fmt.Errorf("%s.%s duplicates reasoning_effort", path, option.ID)
			}
		}
	}
	return nil
}

func validateRoleReasoningEffort(path, value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if value != strings.TrimSpace(value) || !reasoning.IsValid(value) {
		return &reasoning.InvalidEffortError{Path: path, Value: value}
	}
	return nil
}
