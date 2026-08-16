package config

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	// LoopsConfigKey is the top-level Loop configuration section.
	LoopsConfigKey = "loops"
	// LoopInputsConfigKey is the dynamic per-Loop input-default section.
	LoopInputsConfigKey = "inputs"

	loopDefaultsMaxNoProgressWindow = 30
	loopDefaultRetryBackoffBase     = "1s"
	loopDefaultRetryBackoffMax      = "30s"
	loopDefaultSilenceWindow        = "30m"
	loopDefaultAdmissionInterval    = "60s"
	loopDefaultTombstoneHorizon     = "168h"
)

// LoopsConfig holds global and workspace defaults used to seed new loop runs.
type LoopsConfig struct {
	Defaults LoopsDefaultsConfig `toml:"defaults"`
	Breaker  LoopBreakerConfig   `toml:"breaker"`
	Inputs   LoopInputDefaults   `toml:"inputs,omitempty"`

	inputSources LoopInputDefaultSources
}

// LoopInputDefaults stores author-owned defaults by Loop name and declared input key.
// Values remain untyped until the named Loop definition is resolved.
type LoopInputDefaults map[string]map[string]any

type LoopInputDefaultSources map[string]map[string]string

// InputDefaultLayers returns caller-owned global and workspace defaults for one Loop.
// The source map is recorded while overlays are applied; origin is never inferred from values.
func (c *LoopsConfig) InputDefaultLayers(loopName string) (global map[string]any, workspace map[string]any) {
	name := strings.TrimSpace(loopName)
	global = map[string]any{}
	workspace = map[string]any{}
	for key, value := range c.Inputs[name] {
		switch c.inputSources[name][key] {
		case RoleFieldSourceWorkspace:
			workspace[key] = cloneLoopInputDefaultValue(value)
		case RoleFieldSourceGlobal:
			global[key] = cloneLoopInputDefaultValue(value)
		}
	}
	return global, workspace
}

func cloneLoopInputDefaultValue(value any) any {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		return append([]any(nil), typed...)
	default:
		return typed
	}
}

// LoopsDefaultsConfig separates delivery and watch loop seed defaults.
type LoopsDefaultsConfig struct {
	Delivery LoopDefaultConfig `toml:"delivery"`
	Watch    LoopDefaultConfig `toml:"watch"`
}

// LoopDefaultConfig is one `[loops.defaults.<kind>]` seed layer.
type LoopDefaultConfig struct {
	IterationCap    int                         `toml:"iteration_cap"`
	NoProgress      LoopNoProgressDefaultConfig `toml:"no_progress"`
	Gates           LoopGatesDefaultConfig      `toml:"gates"`
	Budget          LoopBudgetDefaultConfig     `toml:"budget"`
	RuntimeDefaults dsl.RuntimeDefaults         `toml:"runtime_defaults"`
	RuntimeRules    []dsl.RuntimeRule           `toml:"runtime_rules"`
	FanOutWidth     int                         `toml:"fan_out_width"`
	Retry           LoopRetryDefaultConfig      `toml:"retry"`
	Liveness        LoopLivenessDefaultConfig   `toml:"liveness"`
	Resume          LoopResumeDefaultConfig     `toml:"resume"`
	Predicates      LoopPredicateDefaultConfig  `toml:"predicates"`
	Waits           LoopWaitDefaultConfig       `toml:"waits"`
	Requests        LoopRequestsDefaultConfig   `toml:"requests"`
	Admission       LoopAdmissionDefaultConfig  `toml:"admission"`
	Autopause       []LoopAutopauseRule         `toml:"autopause"`
}

// LoopNoProgressDefaultConfig controls generation-hash no-progress detection.
type LoopNoProgressDefaultConfig struct {
	Window int `toml:"window"`
}

// LoopGatesDefaultConfig controls gate retry defaults.
type LoopGatesDefaultConfig struct {
	MaxRevisions int `toml:"max_revisions"`
}

// LoopBudgetDefaultConfig controls loop budget defaults.
type LoopBudgetDefaultConfig struct {
	Tokens       int    `toml:"tokens"`
	WallClockSec int    `toml:"wall_clock_sec"`
	OnExceeded   string `toml:"on_exceeded"`
}

// DefaultLoopsConfig returns the TechSpec `[loops.defaults.*]` layer.
func DefaultLoopsConfig() LoopsConfig {
	return LoopsConfig{
		Defaults: LoopsDefaultsConfig{
			Delivery: LoopDefaultConfig{
				IterationCap: 50,
				NoProgress:   LoopNoProgressDefaultConfig{Window: 3},
				Gates:        LoopGatesDefaultConfig{MaxRevisions: 10},
				Budget: LoopBudgetDefaultConfig{
					Tokens:       0,
					WallClockSec: 0,
					OnExceeded:   string(dsl.BudgetExceededHalt),
				},
				FanOutWidth: 4,
				Retry: LoopRetryDefaultConfig{
					MaxAttempts: 3,
					BackoffBase: loopDefaultRetryBackoffBase,
					BackoffMax:  loopDefaultRetryBackoffMax,
				},
				Liveness:   LoopLivenessDefaultConfig{SilenceWindow: loopDefaultSilenceWindow},
				Resume:     LoopResumeDefaultConfig{DeathStreakLimit: 3},
				Predicates: LoopPredicateDefaultConfig{CostLimit: 10000},
				Waits: LoopWaitDefaultConfig{
					AdmissionAttempts:      3,
					AdmissionRetryInterval: loopDefaultAdmissionInterval,
				},
				Admission: LoopAdmissionDefaultConfig{TombstoneHorizon: loopDefaultTombstoneHorizon},
				Autopause: []LoopAutopauseRule{},
			},
			Watch: LoopDefaultConfig{
				IterationCap: 0,
				NoProgress:   LoopNoProgressDefaultConfig{Window: 2},
				Budget: LoopBudgetDefaultConfig{
					Tokens:       0,
					WallClockSec: 0,
					OnExceeded:   string(dsl.BudgetExceededHalt),
				},
				FanOutWidth: 2,
				Retry: LoopRetryDefaultConfig{
					MaxAttempts: 3,
					BackoffBase: loopDefaultRetryBackoffBase,
					BackoffMax:  loopDefaultRetryBackoffMax,
				},
				Liveness:   LoopLivenessDefaultConfig{SilenceWindow: loopDefaultSilenceWindow},
				Resume:     LoopResumeDefaultConfig{DeathStreakLimit: 3},
				Predicates: LoopPredicateDefaultConfig{CostLimit: 10000},
				Waits: LoopWaitDefaultConfig{
					AdmissionAttempts:      3,
					AdmissionRetryInterval: loopDefaultAdmissionInterval,
				},
				Admission: LoopAdmissionDefaultConfig{TombstoneHorizon: loopDefaultTombstoneHorizon},
				Autopause: []LoopAutopauseRule{},
			},
		},
		Breaker: LoopBreakerConfig{Threshold: 5, ProbeInterval: loopDefaultAdmissionInterval},
	}
}

// Validate enforces write-time loop default bounds.
func (c *LoopsConfig) Validate() error {
	if err := c.Defaults.Validate("loops.defaults"); err != nil {
		return err
	}
	return c.Breaker.validate("loops.breaker")
}

// Validate enforces delivery and watch default bounds.
func (c *LoopsDefaultsConfig) Validate(path string) error {
	if err := c.Delivery.Validate(path + ".delivery"); err != nil {
		return err
	}
	if err := c.Watch.Validate(path + ".watch"); err != nil {
		return err
	}
	return nil
}

// Validate enforces one loop default section without clamping operator input.
func (c LoopDefaultConfig) Validate(path string) error {
	if err := validateLoopDefaultNonNegative(path+".iteration_cap", c.IterationCap); err != nil {
		return err
	}
	if err := validateLoopDefaultMax(
		path+".no_progress.window",
		c.NoProgress.Window,
		loopDefaultsMaxNoProgressWindow,
	); err != nil {
		return err
	}
	if err := validateLoopDefaultMax(
		path+".gates.max_revisions",
		c.Gates.MaxRevisions,
		dsl.GateMaxRevisionsCeiling,
	); err != nil {
		return err
	}
	if err := c.Budget.Validate(path + ".budget"); err != nil {
		return err
	}
	if err := validateLoopDefaultNonNegative(path+".fan_out_width", c.FanOutWidth); err != nil {
		return err
	}
	if err := c.Retry.validate(path + ".retry"); err != nil {
		return err
	}
	if err := c.Liveness.validate(path + ".liveness"); err != nil {
		return err
	}
	if err := c.Resume.validate(path + ".resume"); err != nil {
		return err
	}
	if err := c.Predicates.validate(path + ".predicates"); err != nil {
		return err
	}
	if err := c.Waits.validate(path + ".waits"); err != nil {
		return err
	}
	if err := c.Requests.validate(path + ".requests"); err != nil {
		return err
	}
	if err := c.Admission.validate(path + ".admission"); err != nil {
		return err
	}
	return validateLoopAutopauseRules(path+".autopause", c.Autopause)
}

// Validate enforces SET budget semantics before config writes are persisted.
func (c LoopBudgetDefaultConfig) Validate(path string) error {
	if err := validateLoopDefaultNonNegative(path+".tokens", c.Tokens); err != nil {
		return err
	}
	if err := validateLoopDefaultNonNegative(path+".wall_clock_sec", c.WallClockSec); err != nil {
		return err
	}
	onExceeded := strings.TrimSpace(c.OnExceeded)
	switch onExceeded {
	case string(dsl.BudgetExceededHalt), string(dsl.BudgetExceededEscalate):
		return nil
	default:
		return ValidationError{
			Path: path + ".on_exceeded",
			Message: fmt.Sprintf(
				"must be one of %q or %q: %q",
				dsl.BudgetExceededHalt,
				dsl.BudgetExceededEscalate,
				c.OnExceeded,
			),
		}
	}
}

func validateLoopDefaultMax(path string, value int, maxValue int) error {
	if err := validateLoopDefaultNonNegative(path, value); err != nil {
		return err
	}
	if value > maxValue {
		return ValidationError{
			Path:    path,
			Message: fmt.Sprintf("must be <= %d: %d", maxValue, value),
		}
	}
	return nil
}

func validateLoopDefaultNonNegative(path string, value int) error {
	if value < 0 {
		return ValidationError{Path: path, Message: fmt.Sprintf("must be >= 0: %d", value)}
	}
	return nil
}
