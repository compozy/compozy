package config

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

const (
	loopDefaultsMaxFanoutWidth      = 64
	loopDefaultsMaxNoProgressWindow = 30
)

// LoopsConfig holds global and workspace defaults used to seed new loop runs.
type LoopsConfig struct {
	Defaults LoopsDefaultsConfig `toml:"defaults"`
}

// LoopsDefaultsConfig separates delivery and watch loop seed defaults.
type LoopsDefaultsConfig struct {
	Delivery LoopDefaultConfig `toml:"delivery"`
	Watch    LoopDefaultConfig `toml:"watch"`
}

// LoopDefaultConfig is one `[loops.defaults.<kind>]` seed layer.
type LoopDefaultConfig struct {
	IterationCap  int                         `toml:"iteration_cap"`
	NoProgress    LoopNoProgressDefaultConfig `toml:"no_progress"`
	Gates         LoopGatesDefaultConfig      `toml:"gates"`
	Budget        LoopBudgetDefaultConfig     `toml:"budget"`
	ModelDefaults LoopModelDefaultsConfig     `toml:"model_defaults"`
	FanOutWidth   int                         `toml:"fan_out_width"`
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

// LoopModelDefaultsConfig controls loop-owned worker and judge session model defaults.
type LoopModelDefaultsConfig struct {
	Worker string `toml:"worker"`
	Judge  string `toml:"judge"`
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
			},
		},
	}
}

// Validate enforces write-time loop default bounds.
func (c LoopsConfig) Validate() error {
	return c.Defaults.Validate("loops.defaults")
}

// Validate enforces delivery and watch default bounds.
func (c LoopsDefaultsConfig) Validate(path string) error {
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
	return validateLoopDefaultMax(path+".fan_out_width", c.FanOutWidth, loopDefaultsMaxFanoutWidth)
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
