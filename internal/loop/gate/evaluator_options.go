package gate

import "github.com/compozy/agh/internal/loop/dsl"

// Evaluator evaluates gate criteria using injected runtime boundaries.
type Evaluator struct {
	commands               CommandRunner
	judges                 JudgeRunner
	tools                  ToolCaller
	maxRevisionsCeiling    int
	brokenJudgeStreakLimit int
}

var _ GateEvaluator = (*Evaluator)(nil)

// Option configures an Evaluator.
type Option func(*Evaluator)

// WithCommandRunner injects command criterion execution.
func WithCommandRunner(runner CommandRunner) Option {
	return func(e *Evaluator) {
		e.commands = runner
	}
}

// WithJudgeRunner injects agent-judge criterion execution.
func WithJudgeRunner(runner JudgeRunner) Option {
	return func(e *Evaluator) {
		e.judges = runner
	}
}

// WithToolCaller injects tools/call dispatch for extension criteria.
func WithToolCaller(caller ToolCaller) Option {
	return func(e *Evaluator) {
		e.tools = caller
	}
}

// WithMaxRevisionsCeiling overrides the structural max_revisions ceiling.
func WithMaxRevisionsCeiling(ceiling int) Option {
	return func(e *Evaluator) {
		e.maxRevisionsCeiling = ceiling
	}
}

// WithBrokenJudgeStreakLimit overrides the ADR-012 broken judge streak limit.
func WithBrokenJudgeStreakLimit(limit int) Option {
	return func(e *Evaluator) {
		e.brokenJudgeStreakLimit = limit
	}
}

// NewEvaluator creates an evaluator with deterministic defaults.
func NewEvaluator(opts ...Option) *Evaluator {
	evaluator := &Evaluator{
		maxRevisionsCeiling:    dsl.GateMaxRevisionsCeiling,
		brokenJudgeStreakLimit: DefaultBrokenJudgeStreakLimit,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(evaluator)
		}
	}
	if evaluator.maxRevisionsCeiling <= 0 {
		evaluator.maxRevisionsCeiling = dsl.GateMaxRevisionsCeiling
	}
	if evaluator.brokenJudgeStreakLimit <= 0 {
		evaluator.brokenJudgeStreakLimit = DefaultBrokenJudgeStreakLimit
	}
	return evaluator
}
