package doctor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
)

const (
	defaultRunTimeout   = 30 * time.Second
	defaultProbeTimeout = 5 * time.Second
)

// Probe executes one doctor diagnostic check.
type Probe interface {
	ID() string
	Category() string
	Run(context.Context, *ProbeEnv) ([]contract.DiagnosticItem, error)
}

// ProbeEnv carries shared probe dependencies.
type ProbeEnv struct {
	Timeout time.Duration
	Now     func() time.Time
}

// RunOptions controls doctor execution.
type RunOptions struct {
	Only         []string
	Exclude      []string
	Quiet        bool
	Timeout      time.Duration
	ProbeTimeout time.Duration
	Env          ProbeEnv
}

// Runner executes registered probes in deterministic order.
type Runner struct {
	probes []Probe
}

type probeResult struct {
	items []contract.DiagnosticItem
	err   error
}

// NewRunner creates a deterministic doctor runner.
func NewRunner(registry *Registry) (*Runner, error) {
	if registry == nil {
		registry = NewRegistry()
	}
	probes := registry.Probes()
	return &Runner{probes: probes}, nil
}

// Run executes every selected probe and sanitizes every returned DiagnosticItem.
func (r *Runner) Run(ctx context.Context, opts RunOptions) ([]contract.DiagnosticItem, error) {
	if ctx == nil {
		return nil, errors.New("doctor: context is required")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env := opts.Env
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Timeout <= 0 {
		env.Timeout = opts.ProbeTimeout
	}
	if env.Timeout <= 0 {
		env.Timeout = defaultProbeTimeout
	}

	filters := newRunFilters(opts.Only, opts.Exclude)
	var items []contract.DiagnosticItem
	for _, probe := range r.probes {
		if !filters.allows(probe) {
			continue
		}
		probeItems := r.runProbe(ctx, probe, env)
		for _, item := range probeItems {
			if opts.Quiet && item.Severity == contract.SeverityOK {
				continue
			}
			items = append(items, item)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return items, nil
}

func (r *Runner) runProbe(ctx context.Context, probe Probe, env ProbeEnv) []contract.DiagnosticItem {
	probeCtx, cancel := context.WithTimeout(ctx, env.Timeout)
	defer cancel()

	resultCh := make(chan probeResult, 1)
	go func() {
		result := probeResult{}
		defer func() {
			if recovered := recover(); recovered != nil {
				result = probeResult{err: fmt.Errorf("doctor: probe panic: %v", recovered)}
			}
			resultCh <- result
		}()
		result.items, result.err = probe.Run(probeCtx, &env)
	}()

	var result probeResult
	select {
	case result = <-resultCh:
	case <-probeCtx.Done():
		return []contract.DiagnosticItem{probeErrorItem(probe, probeCtx.Err())}
	}
	if result.err != nil {
		return []contract.DiagnosticItem{probeErrorItem(probe, result.err)}
	}
	if probeCtx.Err() != nil && len(result.items) == 0 {
		return []contract.DiagnosticItem{probeErrorItem(probe, probeCtx.Err())}
	}

	sanitized := make([]contract.DiagnosticItem, 0, len(result.items))
	for _, item := range result.items {
		item = diagnostics.RedactItem(item)
		if validateErr := contract.ValidateDiagnosticItem(item); validateErr != nil {
			sanitized = append(sanitized, probeErrorItem(probe, validateErr))
			continue
		}
		sanitized = append(sanitized, item)
	}
	return sanitized
}

func probeErrorItem(probe Probe, err error) contract.DiagnosticItem {
	code := contract.CodeProbeFailed
	title := "Doctor probe failed"
	if errors.Is(err, context.DeadlineExceeded) {
		code = contract.CodeProbeTimeout
		title = "Doctor probe timed out"
	}
	return diagnostics.NewItem(
		probe.ID(),
		code,
		probe.Category(),
		title,
		err.Error(),
		contract.SeverityError,
		contract.FreshnessLive,
		diagnostics.WithEvidence(map[string]any{
			"probe_id": probe.ID(),
			"error":    err,
		}),
	)
}

type runFilters struct {
	only    map[string]struct{}
	exclude map[string]struct{}
}

func newRunFilters(only []string, exclude []string) runFilters {
	return runFilters{
		only:    normalizeFilterSet(only),
		exclude: normalizeFilterSet(exclude),
	}
}

func normalizeFilterSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func (f runFilters) allows(probe Probe) bool {
	id := probe.ID()
	category := probe.Category()
	_, idSelected := f.only[id]
	_, categorySelected := f.only[category]
	if category == contract.CategoryBridge && !idSelected && !categorySelected {
		return false
	}
	if len(f.only) > 0 {
		if !idSelected && !categorySelected {
			return false
		}
	}
	if _, ok := f.exclude[id]; ok {
		return false
	}
	if _, ok := f.exclude[category]; ok {
		return false
	}
	return true
}

// Registry stores doctor probes and returns them sorted by ID.
type Registry struct {
	probes map[string]Probe
}

// NewRegistry creates an empty probe registry.
func NewRegistry() *Registry {
	return &Registry{probes: make(map[string]Probe)}
}

// Register adds one probe after validating its deterministic identity.
func (r *Registry) Register(probe Probe) error {
	if r == nil {
		return errors.New("doctor: registry is nil")
	}
	if probe == nil {
		return errors.New("doctor: probe is nil")
	}
	id := strings.TrimSpace(probe.ID())
	if id == "" {
		return errors.New("doctor: probe id is required")
	}
	category := strings.TrimSpace(probe.Category())
	if !contract.IsDiagnosticCategory(category) {
		return fmt.Errorf("doctor: probe %q has unknown category %q", id, category)
	}
	if _, exists := r.probes[id]; exists {
		return fmt.Errorf("doctor: duplicate probe id %q", id)
	}
	r.probes[id] = probe
	return nil
}

// Probes returns registered probes sorted by ID.
func (r *Registry) Probes() []Probe {
	if r == nil || len(r.probes) == 0 {
		return nil
	}
	ids := make([]string, 0, len(r.probes))
	for id := range r.probes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	probes := make([]Probe, 0, len(ids))
	for _, id := range ids {
		probes = append(probes, r.probes[id])
	}
	return probes
}

// ProbeFunc adapts a function into a Probe for extensions and tests.
type ProbeFunc struct {
	ProbeID       string
	ProbeCategory string
	RunFunc       func(context.Context, *ProbeEnv) ([]contract.DiagnosticItem, error)
}

var _ Probe = (*ProbeFunc)(nil)

func (p *ProbeFunc) ID() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.ProbeID)
}

func (p *ProbeFunc) Category() string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(p.ProbeCategory)
}

func (p *ProbeFunc) Run(ctx context.Context, env *ProbeEnv) ([]contract.DiagnosticItem, error) {
	if p == nil || p.RunFunc == nil {
		return nil, errors.New("doctor: probe function is not configured")
	}
	return p.RunFunc(ctx, env)
}
