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
	RoleCoordinator       RoleName = "coordinator"
	RoleDream             RoleName = "dream"
	RoleCheckpointSummary RoleName = "checkpoint_summary"
	RoleMemoryExtractor   RoleName = "memory_extractor"
	RoleAutoTitle         RoleName = "auto_title"
	RoleMemoryController  RoleName = "memory_controller"
)

var allRoleNames = []RoleName{
	RoleCoordinator,
	RoleDream,
	RoleCheckpointSummary,
	RoleMemoryExtractor,
	RoleAutoTitle,
	RoleMemoryController,
}

// RoleNames returns the complete closed role roster.
func RoleNames() []RoleName {
	return append([]RoleName(nil), allRoleNames...)
}

// RolesConfig is the closed [roles] roster.
type RolesConfig struct {
	Coordinator       CoordinatorRoleConfig      `toml:"coordinator"`
	Dream             RoleConfig                 `toml:"dream"`
	CheckpointSummary RoleConfig                 `toml:"checkpoint_summary"`
	MemoryExtractor   RoleConfig                 `toml:"memory_extractor"`
	AutoTitle         RoleConfig                 `toml:"auto_title"`
	MemoryController  MemoryControllerRoleConfig `toml:"memory_controller"`
}

// CloneRolesConfig returns an ownership-safe copy of the roles configuration.
func CloneRolesConfig(source *RolesConfig) RolesConfig {
	if source == nil {
		return RolesConfig{}
	}
	cloned := *source
	cloned.Coordinator.ACPOptions = CloneACPOptionSelections(source.Coordinator.ACPOptions)
	cloned.Dream.ACPOptions = CloneACPOptionSelections(source.Dream.ACPOptions)
	cloned.CheckpointSummary.ACPOptions = CloneACPOptionSelections(source.CheckpointSummary.ACPOptions)
	cloned.MemoryExtractor.ACPOptions = CloneACPOptionSelections(source.MemoryExtractor.ACPOptions)
	cloned.AutoTitle.ACPOptions = CloneACPOptionSelections(source.AutoTitle.ACPOptions)
	cloned.MemoryController.ACPOptions = CloneACPOptionSelections(source.MemoryController.ACPOptions)
	cloned.Coordinator.FallbackChain = cloneRoleFallbacks(source.Coordinator.FallbackChain)
	cloned.Dream.FallbackChain = cloneRoleFallbacks(source.Dream.FallbackChain)
	cloned.CheckpointSummary.FallbackChain = cloneRoleFallbacks(source.CheckpointSummary.FallbackChain)
	cloned.MemoryExtractor.FallbackChain = cloneRoleFallbacks(source.MemoryExtractor.FallbackChain)
	cloned.AutoTitle.FallbackChain = cloneRoleFallbacks(source.AutoTitle.FallbackChain)
	cloned.MemoryController.FallbackChain = cloneRoleFallbacks(source.MemoryController.FallbackChain)
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
	Provider        string               `toml:"provider"`
	Model           string               `toml:"model"`
	ReasoningEffort string               `toml:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed       `toml:"speed,omitempty"`
	ACPOptions      []ACPOptionSelection `toml:"acp_options,omitempty"`
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

// MemoryControllerRoleConfig controls the in-process memory-controller model call.
type MemoryControllerRoleConfig struct {
	Enabled         bool                 `toml:"enabled"`
	Provider        string               `toml:"provider,omitempty"`
	Model           string               `toml:"model"`
	ReasoningEffort string               `toml:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed       `toml:"speed,omitempty"`
	ACPOptions      []ACPOptionSelection `toml:"acp_options,omitempty"`
	Timeout         time.Duration        `toml:"timeout"`
	TopK            int                  `toml:"top_k"`
	PromptVersion   string               `toml:"prompt_version"`
	MaxTokensOut    int                  `toml:"max_tokens_out"`
	FallbackChain   []RoleFallback       `toml:"fallback_chain,omitempty"`
}

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

// DefaultRolesConfig preserves the effective routing defaults from the pre-roles config.
func DefaultRolesConfig() RolesConfig {
	return RolesConfig{
		Coordinator: CoordinatorRoleConfig{
			RoleConfig:                    RoleConfig{Enabled: false},
			TTL:                           DefaultCoordinatorTTL,
			MaxChildren:                   DefaultCoordinatorMaxChildren,
			MaxActiveSessionsPerWorkspace: DefaultCoordinatorMaxActiveSessionsPerWorkspace,
		},
		Dream:             RoleConfig{Enabled: true},
		CheckpointSummary: RoleConfig{Enabled: true},
		MemoryExtractor:   RoleConfig{Enabled: true},
		AutoTitle:         RoleConfig{Enabled: true},
		MemoryController: MemoryControllerRoleConfig{
			Enabled:       true,
			Provider:      "pi",
			Model:         "anthropic/claude-haiku-4",
			Timeout:       250 * time.Millisecond,
			TopK:          5,
			PromptVersion: "v1",
			MaxTokensOut:  256,
		},
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
		{name: RoleDream, config: c.Dream},
		{name: RoleCheckpointSummary, config: c.CheckpointSummary},
		{name: RoleMemoryExtractor, config: c.MemoryExtractor},
		{name: RoleAutoTitle, config: c.AutoTitle},
	} {
		if err := role.config.validate(path+"."+string(role.name), resolver); err != nil {
			return err
		}
	}
	return c.MemoryController.validate(path+"."+string(RoleMemoryController), resolver)
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

func (c MemoryControllerRoleConfig) validate(path string, resolver providerResolver) error {
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
	if err := validateRoleFallbacks(path, c.FallbackChain, resolver); err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("%s.model is required", path)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("%s.timeout must be positive: %s", path, c.Timeout)
	}
	if c.TopK <= 0 {
		return fmt.Errorf("%s.top_k must be positive: %d", path, c.TopK)
	}
	if strings.TrimSpace(c.PromptVersion) == "" {
		return fmt.Errorf("%s.prompt_version is required", path)
	}
	if c.MaxTokensOut <= 0 {
		return fmt.Errorf("%s.max_tokens_out must be positive: %d", path, c.MaxTokensOut)
	}
	return nil
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
	for i, fallback := range fallbacks {
		fallbackPath := fmt.Sprintf("%s.fallback_chain[%d]", path, i)
		providerName := strings.TrimSpace(fallback.Provider)
		if providerName == "" {
			return fmt.Errorf("%s.provider is required", fallbackPath)
		}
		if strings.TrimSpace(fallback.Model) == "" {
			return fmt.Errorf("%s.model is required", fallbackPath)
		}
		if err := validateRoleReasoningEffort(fallbackPath+".reasoning_effort", fallback.ReasoningEffort); err != nil {
			return err
		}
		if err := validateAgentSpeed(fallback.Speed, fallbackPath+".speed"); err != nil {
			return err
		}
		if err := validateRoleACPOptions(fallbackPath+".acp_options", fallback.ACPOptions); err != nil {
			return err
		}
		if err := validateRoleACPOptionConflicts(
			fallbackPath+".acp_options",
			fallback.ACPOptions,
			fallback.Speed,
			fallback.ReasoningEffort,
		); err != nil {
			return err
		}
		if resolver == nil {
			return fmt.Errorf("%s.provider resolver is required", fallbackPath)
		}
		if _, err := resolver.ResolveProvider(providerName); err != nil {
			return fmt.Errorf("%s.provider: %w", fallbackPath, err)
		}
	}
	return nil
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
