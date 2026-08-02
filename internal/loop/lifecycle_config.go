package loop

import (
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
)

const (
	LifecycleSourceNode       = "node"
	LifecycleSourceLoopConfig = "loop_config"
	LifecycleSourceDefault    = "default"
	BreakerGlobalSource       = "breaker (global)"
)

// LifecycleConfig is one pointer-backed node-lifecycle configuration layer.
type LifecycleConfig struct {
	RetryMaxAttempts       *int
	RetryBackoffBase       *time.Duration
	RetryBackoffMax        *time.Duration
	LivenessSilenceWindow  *time.Duration
	ResumeDeathStreakLimit *int
	PredicateCostLimit     *uint64
	WaitAdmissionAttempts  *int
	WaitAdmissionInterval  *time.Duration
	AdmissionHorizon       *time.Duration
	Autopause              []LifecycleAutopauseRule
}

// LifecycleAutopauseRule is one validated ordered runtime rule.
type LifecycleAutopauseRule struct {
	Match  string `json:"match"`
	Action string `json:"action"`
}

// ResolvedLifecycleConfig is the admission-pinned lifecycle policy for one node.
type ResolvedLifecycleConfig struct {
	RetryMaxAttempts       int                      `json:"retry_max_attempts"`
	RetryBackoffBase       time.Duration            `json:"retry_backoff_base"`
	RetryBackoffMax        time.Duration            `json:"retry_backoff_max"`
	RetryNonRetryable      []string                 `json:"retry_non_retryable"`
	Deadline               *time.Duration           `json:"deadline,omitempty"`
	LivenessSilenceWindow  time.Duration            `json:"liveness_silence_window"`
	ResumeDeathStreakLimit int                      `json:"resume_death_streak_limit"`
	PredicateCostLimit     uint64                   `json:"predicate_cost_limit"`
	WaitAdmissionAttempts  int                      `json:"wait_admission_attempts"`
	WaitAdmissionInterval  time.Duration            `json:"wait_admission_interval"`
	AdmissionHorizon       time.Duration            `json:"admission_horizon"`
	Autopause              []LifecycleAutopauseRule `json:"autopause"`
	Sources                map[string]string        `json:"sources"`
}

// GlobalBreakerPolicy is reload-scoped and deliberately excluded from run snapshots.
type GlobalBreakerPolicy struct {
	Threshold     int
	ProbeInterval time.Duration
	Source        string
}

// DefaultLifecycleConfig returns the shipped per-family lifecycle defaults.
func DefaultLifecycleConfig() LifecycleConfig {
	return LifecycleConfig{
		RetryMaxAttempts:       new(3),
		RetryBackoffBase:       new(time.Second),
		RetryBackoffMax:        new(30 * time.Second),
		LivenessSilenceWindow:  new(30 * time.Minute),
		ResumeDeathStreakLimit: new(3),
		PredicateCostLimit:     new(uint64(10000)),
		WaitAdmissionAttempts:  new(3),
		WaitAdmissionInterval:  new(time.Minute),
		AdmissionHorizon:       new(168 * time.Hour),
		Autopause:              []LifecycleAutopauseRule{},
	}
}

// Clone returns an owned copy of a lifecycle configuration layer.
func (c LifecycleConfig) Clone() LifecycleConfig {
	cloned := LifecycleConfig{}
	if c.Autopause != nil {
		cloned.Autopause = append([]LifecycleAutopauseRule{}, c.Autopause...)
	}
	cloneLifecycleInt(&cloned.RetryMaxAttempts, c.RetryMaxAttempts)
	cloneLifecycleDuration(&cloned.RetryBackoffBase, c.RetryBackoffBase)
	cloneLifecycleDuration(&cloned.RetryBackoffMax, c.RetryBackoffMax)
	cloneLifecycleDuration(&cloned.LivenessSilenceWindow, c.LivenessSilenceWindow)
	cloneLifecycleInt(&cloned.ResumeDeathStreakLimit, c.ResumeDeathStreakLimit)
	if c.PredicateCostLimit != nil {
		cloned.PredicateCostLimit = new(*c.PredicateCostLimit)
	}
	cloneLifecycleInt(&cloned.WaitAdmissionAttempts, c.WaitAdmissionAttempts)
	cloneLifecycleDuration(&cloned.WaitAdmissionInterval, c.WaitAdmissionInterval)
	cloneLifecycleDuration(&cloned.AdmissionHorizon, c.AdmissionHorizon)
	return cloned
}

// ResolveNodeLifecycleConfig applies node > loop_config > default without mutating any layer.
func ResolveNodeLifecycleConfig(
	node dsl.Node,
	loopConfig *LifecycleConfig,
	defaults LifecycleConfig,
) (ResolvedLifecycleConfig, error) {
	node.Normalize()
	resolved := ResolvedLifecycleConfig{Sources: map[string]string{}}
	applyLifecycleLayer(&resolved, defaults, LifecycleSourceDefault)
	if loopConfig != nil {
		applyLifecycleLayer(&resolved, *loopConfig, LifecycleSourceLoopConfig)
	}
	if err := applyNodeLifecycleLayer(&resolved, node); err != nil {
		return ResolvedLifecycleConfig{}, err
	}
	if err := validateResolvedLifecycleConfig(resolved); err != nil {
		return ResolvedLifecycleConfig{}, err
	}
	resolved.RetryNonRetryable = append([]string(nil), resolved.RetryNonRetryable...)
	resolved.Autopause = append([]LifecycleAutopauseRule(nil), resolved.Autopause...)
	return resolved, nil
}

// ResolveGlobalBreakerPolicy returns reload-scoped policy with its fixed source label.
func ResolveGlobalBreakerPolicy(threshold int, probeInterval time.Duration) (GlobalBreakerPolicy, error) {
	if threshold <= 0 || probeInterval <= 0 {
		return GlobalBreakerPolicy{}, fmt.Errorf("%w: global breaker policy must be positive", ErrValidation)
	}
	return GlobalBreakerPolicy{
		Threshold:     threshold,
		ProbeInterval: probeInterval,
		Source:        BreakerGlobalSource,
	}, nil
}

func applyLifecycleLayer(target *ResolvedLifecycleConfig, layer LifecycleConfig, source string) {
	setLifecycleInt(&target.RetryMaxAttempts, layer.RetryMaxAttempts, target.Sources, "retry.max_attempts", source)
	setLifecycleDuration(&target.RetryBackoffBase, layer.RetryBackoffBase, target.Sources, "retry.backoff_base", source)
	setLifecycleDuration(&target.RetryBackoffMax, layer.RetryBackoffMax, target.Sources, "retry.backoff_max", source)
	setLifecycleDuration(
		&target.LivenessSilenceWindow,
		layer.LivenessSilenceWindow,
		target.Sources,
		"liveness.silence_window",
		source,
	)
	setLifecycleInt(
		&target.ResumeDeathStreakLimit,
		layer.ResumeDeathStreakLimit,
		target.Sources,
		"resume.death_streak_limit",
		source,
	)
	if layer.PredicateCostLimit != nil {
		target.PredicateCostLimit = *layer.PredicateCostLimit
		target.Sources["predicates.cost_limit"] = source
	}
	setLifecycleInt(
		&target.WaitAdmissionAttempts,
		layer.WaitAdmissionAttempts,
		target.Sources,
		"waits.admission_attempts",
		source,
	)
	setLifecycleDuration(
		&target.WaitAdmissionInterval,
		layer.WaitAdmissionInterval,
		target.Sources,
		"waits.admission_retry_interval",
		source,
	)
	setLifecycleDuration(
		&target.AdmissionHorizon,
		layer.AdmissionHorizon,
		target.Sources,
		"admission.tombstone_horizon",
		source,
	)
	if len(layer.Autopause) > 0 {
		target.Autopause = append(target.Autopause, layer.Autopause...)
	}
}

func applyNodeLifecycleLayer(target *ResolvedLifecycleConfig, node dsl.Node) error {
	if node.Retry != nil {
		target.RetryMaxAttempts = node.Retry.MaxAttempts
		target.Sources["retry.max_attempts"] = LifecycleSourceNode
		target.RetryNonRetryable = append([]string(nil), node.Retry.NonRetryable...)
		if node.Retry.Backoff != nil {
			if strings.TrimSpace(node.Retry.Backoff.Base) != "" {
				base, err := time.ParseDuration(strings.TrimSpace(node.Retry.Backoff.Base))
				if err != nil {
					return fmt.Errorf("%w: node retry.backoff.base: %v", ErrValidation, err)
				}
				target.RetryBackoffBase = base
				target.Sources["retry.backoff_base"] = LifecycleSourceNode
			}
			if strings.TrimSpace(node.Retry.Backoff.Max) != "" {
				maximum, err := time.ParseDuration(strings.TrimSpace(node.Retry.Backoff.Max))
				if err != nil {
					return fmt.Errorf("%w: node retry.backoff.max: %v", ErrValidation, err)
				}
				target.RetryBackoffMax = maximum
				target.Sources["retry.backoff_max"] = LifecycleSourceNode
			}
		}
	}
	if strings.TrimSpace(node.Deadline) != "" {
		deadline, err := time.ParseDuration(strings.TrimSpace(node.Deadline))
		if err != nil {
			return fmt.Errorf("%w: node deadline: %v", ErrValidation, err)
		}
		target.Deadline = new(deadline)
		target.Sources["deadline"] = LifecycleSourceNode
	}
	return nil
}

func validateResolvedLifecycleConfig(config ResolvedLifecycleConfig) error {
	if config.RetryMaxAttempts < 0 || config.RetryMaxAttempts > 10 {
		return fmt.Errorf("%w: retry max attempts must be between 0 and 10", ErrValidation)
	}
	if config.RetryBackoffBase <= 0 || config.RetryBackoffMax <= 0 ||
		config.RetryBackoffBase > config.RetryBackoffMax {
		return fmt.Errorf("%w: retry backoff bounds are invalid", ErrValidation)
	}
	if config.LivenessSilenceWindow < 0 || config.ResumeDeathStreakLimit <= 0 ||
		config.PredicateCostLimit == 0 || config.WaitAdmissionAttempts <= 0 ||
		config.WaitAdmissionInterval <= 0 || config.AdmissionHorizon <= 0 {
		return fmt.Errorf("%w: lifecycle bounds are invalid", ErrValidation)
	}
	if config.Deadline != nil && *config.Deadline <= 0 {
		return fmt.Errorf("%w: node deadline must be positive", ErrValidation)
	}
	return nil
}

func setLifecycleInt(target *int, value *int, sources map[string]string, key, source string) {
	if value == nil {
		return
	}
	*target = *value
	sources[key] = source
}

func setLifecycleDuration(
	target *time.Duration,
	value *time.Duration,
	sources map[string]string,
	key string,
	source string,
) {
	if value == nil {
		return
	}
	*target = *value
	sources[key] = source
}

func cloneLifecycleInt(target **int, source *int) {
	if source != nil {
		*target = new(*source)
	}
}

func cloneLifecycleDuration(target **time.Duration, source *time.Duration) {
	if source != nil {
		*target = new(*source)
	}
}
