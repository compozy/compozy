package config

import "github.com/compozy/compozy/internal/loop/dsl"

type loopsOverlay struct {
	Defaults loopsDefaultsOverlay `toml:"defaults"`
}

type loopsDefaultsOverlay struct {
	Delivery loopDefaultOverlay `toml:"delivery"`
	Watch    loopDefaultOverlay `toml:"watch"`
}

type loopDefaultOverlay struct {
	IterationCap    *int                         `toml:"iteration_cap"`
	NoProgress      loopNoProgressDefaultOverlay `toml:"no_progress"`
	Gates           loopGatesDefaultOverlay      `toml:"gates"`
	Budget          loopBudgetDefaultOverlay     `toml:"budget"`
	RuntimeDefaults loopRuntimeDefaultsOverlay   `toml:"runtime_defaults"`
	RuntimeRules    []dsl.RuntimeRule            `toml:"runtime_rules"`
	FanOutWidth     *int                         `toml:"fan_out_width"`
}

type loopNoProgressDefaultOverlay struct {
	Window *int `toml:"window"`
}

type loopGatesDefaultOverlay struct {
	MaxRevisions *int `toml:"max_revisions"`
}

type loopBudgetDefaultOverlay struct {
	Tokens       *int    `toml:"tokens"`
	WallClockSec *int    `toml:"wall_clock_sec"`
	OnExceeded   *string `toml:"on_exceeded"`
}

type loopRuntimeDefaultsOverlay struct {
	Worker loopRuntimeSpecOverlay `toml:"worker"`
	Judge  loopRuntimeSpecOverlay `toml:"judge"`
}

type loopRuntimeSpecOverlay struct {
	Provider  *string `toml:"provider"`
	Model     *string `toml:"model"`
	Reasoning *string `toml:"reasoning"`
}

func (o loopsOverlay) Apply(dst *LoopsConfig) {
	o.Defaults.Apply(&dst.Defaults)
}

func (o loopsDefaultsOverlay) Apply(dst *LoopsDefaultsConfig) {
	o.Delivery.Apply(&dst.Delivery)
	o.Watch.Apply(&dst.Watch)
}

func (o loopDefaultOverlay) Apply(dst *LoopDefaultConfig) {
	if o.IterationCap != nil {
		dst.IterationCap = *o.IterationCap
	}
	o.NoProgress.Apply(&dst.NoProgress)
	o.Gates.Apply(&dst.Gates)
	o.Budget.Apply(&dst.Budget)
	o.RuntimeDefaults.Apply(&dst.RuntimeDefaults)
	if len(o.RuntimeRules) > 0 {
		dst.RuntimeRules = append(dst.RuntimeRules, o.RuntimeRules...)
	}
	if o.FanOutWidth != nil {
		dst.FanOutWidth = *o.FanOutWidth
	}
}

func (o loopNoProgressDefaultOverlay) Apply(dst *LoopNoProgressDefaultConfig) {
	if o.Window != nil {
		dst.Window = *o.Window
	}
}

func (o loopGatesDefaultOverlay) Apply(dst *LoopGatesDefaultConfig) {
	if o.MaxRevisions != nil {
		dst.MaxRevisions = *o.MaxRevisions
	}
}

func (o loopBudgetDefaultOverlay) Apply(dst *LoopBudgetDefaultConfig) {
	if o.Tokens != nil {
		dst.Tokens = *o.Tokens
	}
	if o.WallClockSec != nil {
		dst.WallClockSec = *o.WallClockSec
	}
	if o.OnExceeded != nil {
		dst.OnExceeded = *o.OnExceeded
	}
}

func (o loopRuntimeDefaultsOverlay) Apply(dst *dsl.RuntimeDefaults) {
	o.Worker.Apply(&dst.Worker)
	o.Judge.Apply(&dst.Judge)
}

func (o loopRuntimeSpecOverlay) Apply(dst *dsl.RuntimeSpec) {
	if o.Provider != nil {
		dst.Provider = *o.Provider
	}
	if o.Model != nil {
		dst.Model = *o.Model
	}
	if o.Reasoning != nil {
		dst.Reasoning = *o.Reasoning
	}
}
