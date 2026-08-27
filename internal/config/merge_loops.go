package config

import (
	"github.com/compozy/compozy/internal/loop/dsl"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

type loopsOverlay struct {
	ReconcileInterval *string              `toml:"reconcile_interval"`
	Defaults          loopsDefaultsOverlay `toml:"defaults"`
	Breaker           loopBreakerOverlay   `toml:"breaker"`
	Inputs            LoopInputDefaults    `toml:"inputs"`
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
	Retry           loopRetryDefaultOverlay      `toml:"retry"`
	Liveness        loopLivenessDefaultOverlay   `toml:"liveness"`
	Resume          loopResumeDefaultOverlay     `toml:"resume"`
	Predicates      loopPredicateDefaultOverlay  `toml:"predicates"`
	Waits           loopWaitDefaultOverlay       `toml:"waits"`
	Requests        loopRequestsDefaultOverlay   `toml:"requests"`
	Admission       loopAdmissionDefaultOverlay  `toml:"admission"`
	Autopause       []LoopAutopauseRule          `toml:"autopause"`
}

type loopRetryDefaultOverlay struct {
	MaxAttempts *int    `toml:"max_attempts"`
	BackoffBase *string `toml:"backoff_base"`
	BackoffMax  *string `toml:"backoff_max"`
}

type loopLivenessDefaultOverlay struct {
	SilenceWindow *string `toml:"silence_window"`
}

type loopResumeDefaultOverlay struct {
	DeathStreakLimit *int `toml:"death_streak_limit"`
}

type loopPredicateDefaultOverlay struct {
	CostLimit *uint64 `toml:"cost_limit"`
}

type loopWaitDefaultOverlay struct {
	AdmissionAttempts      *int    `toml:"admission_attempts"`
	AdmissionRetryInterval *string `toml:"admission_retry_interval"`
}

type loopRequestsDefaultOverlay struct {
	ExpireAfter *string `toml:"expire_after"`
}

type loopAdmissionDefaultOverlay struct {
	TombstoneHorizon *string `toml:"tombstone_horizon"`
}

type loopBreakerOverlay struct {
	Threshold     *int    `toml:"threshold"`
	ProbeInterval *string `toml:"probe_interval"`
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
	Provider   *string                   `toml:"provider"`
	Model      *string                   `toml:"model"`
	Reasoning  *string                   `toml:"reasoning"`
	Speed      *string                   `toml:"speed"`
	ACPOptions *[]dsl.ACPOptionSelection `toml:"acp_options"`
}

func (o *loopsOverlay) Apply(dst *LoopsConfig) {
	if o.ReconcileInterval != nil {
		dst.ReconcileInterval = *o.ReconcileInterval
	}
	o.Defaults.Apply(&dst.Defaults)
	o.Breaker.Apply(&dst.Breaker)
	if dst.Inputs == nil {
		dst.Inputs = LoopInputDefaults{}
	}
	for loopName, values := range o.Inputs {
		if dst.Inputs[loopName] == nil {
			dst.Inputs[loopName] = map[string]any{}
		}
		for key, value := range values {
			dst.Inputs[loopName][key] = cloneLoopInputDefaultValue(value)
		}
	}
}

func (o *loopsOverlay) recordInputSources(dst *LoopsConfig, source string) {
	if dst.inputSources == nil {
		dst.inputSources = LoopInputDefaultSources{}
	}
	for loopName, values := range o.Inputs {
		if dst.inputSources[loopName] == nil {
			dst.inputSources[loopName] = map[string]string{}
		}
		for key := range values {
			dst.inputSources[loopName][key] = source
		}
	}
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
	o.Retry.Apply(&dst.Retry)
	o.Liveness.Apply(&dst.Liveness)
	o.Resume.Apply(&dst.Resume)
	o.Predicates.Apply(&dst.Predicates)
	o.Waits.Apply(&dst.Waits)
	o.Requests.Apply(&dst.Requests)
	o.Admission.Apply(&dst.Admission)
	if len(o.Autopause) > 0 {
		dst.Autopause = append(dst.Autopause, o.Autopause...)
	}
}

func (o loopRetryDefaultOverlay) Apply(dst *LoopRetryDefaultConfig) {
	if o.MaxAttempts != nil {
		dst.MaxAttempts = *o.MaxAttempts
	}
	if o.BackoffBase != nil {
		dst.BackoffBase = *o.BackoffBase
	}
	if o.BackoffMax != nil {
		dst.BackoffMax = *o.BackoffMax
	}
}

func (o loopLivenessDefaultOverlay) Apply(dst *LoopLivenessDefaultConfig) {
	if o.SilenceWindow != nil {
		dst.SilenceWindow = *o.SilenceWindow
	}
}

func (o loopResumeDefaultOverlay) Apply(dst *LoopResumeDefaultConfig) {
	if o.DeathStreakLimit != nil {
		dst.DeathStreakLimit = *o.DeathStreakLimit
	}
}

func (o loopPredicateDefaultOverlay) Apply(dst *LoopPredicateDefaultConfig) {
	if o.CostLimit != nil {
		dst.CostLimit = *o.CostLimit
	}
}

func (o loopWaitDefaultOverlay) Apply(dst *LoopWaitDefaultConfig) {
	if o.AdmissionAttempts != nil {
		dst.AdmissionAttempts = *o.AdmissionAttempts
	}
	if o.AdmissionRetryInterval != nil {
		dst.AdmissionRetryInterval = *o.AdmissionRetryInterval
	}
}

func (o loopRequestsDefaultOverlay) Apply(dst *LoopRequestsDefaultConfig) {
	if o.ExpireAfter != nil {
		dst.ExpireAfter = *o.ExpireAfter
	}
}

func (o loopAdmissionDefaultOverlay) Apply(dst *LoopAdmissionDefaultConfig) {
	if o.TombstoneHorizon != nil {
		dst.TombstoneHorizon = *o.TombstoneHorizon
	}
}

func (o loopBreakerOverlay) Apply(dst *LoopBreakerConfig) {
	if o.Threshold != nil {
		dst.Threshold = *o.Threshold
	}
	if o.ProbeInterval != nil {
		dst.ProbeInterval = *o.ProbeInterval
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
	if o.Speed != nil {
		dst.Speed = speedpkg.Speed(*o.Speed)
	}
	if o.ACPOptions != nil {
		dst.ACPOptions = dsl.CloneACPOptionSelections(*o.ACPOptions)
	}
}
