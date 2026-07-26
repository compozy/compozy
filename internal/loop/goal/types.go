package goal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/gate"
)

var (
	errStoreRequired    = errors.New("goal executor store is required")
	errBinderRequired   = errors.New("goal executor managed binder is required")
	errJudgeRequired    = errors.New("goal executor judge is required")
	errBudgetRequired   = errors.New("goal executor budget guard is required")
	errContextRequired  = errors.New("goal executor context health is required")
	errRecoveryRequired = errors.New("goal executor prompt recovery is required")
)

const (
	brokenJudgeStreakLimit   = 3
	defaultCompactionTimeout = 30 * time.Second
	defaultCompactionDrain   = 5 * time.Second
)

// Dependencies contains the child-package ports supplied by daemon composition.
type Dependencies struct {
	Store             ExecutorStore
	Binder            loop.ManagedActionSessionBinder
	Judge             JudgeEvaluator
	Budget            BudgetGuard
	Context           ContextHealth
	Recovery          PromptRecovery
	Now               func() time.Time
	CompactionTimeout time.Duration
	CompactionDrain   time.Duration
}

// Executor advances one serialized Goal worker segment until its first control boundary.
type Executor struct {
	store             ExecutorStore
	binder            loop.ManagedActionSessionBinder
	judge             JudgeEvaluator
	budget            BudgetGuard
	context           ContextHealth
	recovery          PromptRecovery
	now               func() time.Time
	compactionTimeout time.Duration
	compactionDrain   time.Duration
}

var _ loop.ActionExecutor = (*Executor)(nil)

// NewExecutor constructs the convergence engine from daemon-owned ports.
func NewExecutor(deps Dependencies) (*Executor, error) {
	missing := []struct {
		absent bool
		err    error
	}{
		{absent: deps.Store == nil, err: errStoreRequired},
		{absent: deps.Binder == nil, err: errBinderRequired},
		{absent: deps.Judge == nil, err: errJudgeRequired},
		{absent: deps.Budget == nil, err: errBudgetRequired},
		{absent: deps.Context == nil, err: errContextRequired},
		{absent: deps.Recovery == nil, err: errRecoveryRequired},
	}
	for _, dependency := range missing {
		if dependency.absent {
			return nil, fmt.Errorf("%w: %w", loop.ErrActionDependencyMissing, dependency.err)
		}
	}
	now := deps.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	compactionTimeout := deps.CompactionTimeout
	if compactionTimeout <= 0 {
		compactionTimeout = defaultCompactionTimeout
	}
	compactionDrain := deps.CompactionDrain
	if compactionDrain <= 0 {
		compactionDrain = defaultCompactionDrain
	}
	return &Executor{
		store:             deps.Store,
		binder:            deps.Binder,
		judge:             deps.Judge,
		budget:            deps.Budget,
		context:           deps.Context,
		recovery:          deps.Recovery,
		now:               now,
		compactionTimeout: compactionTimeout,
		compactionDrain:   compactionDrain,
	}, nil
}

// Harvest returns only a successfully settled Goal result.
func (e *Executor) Harvest(_ context.Context, raw loop.ActionRawResult, _ dsl.Node) (loop.ActionOutput, error) {
	if raw.Control == nil || raw.Control.Disposition != loop.ActionDispositionSucceeded {
		return loop.ActionOutput{}, fmt.Errorf("%w: Goal harvest requires succeeded control", loop.ErrValidation)
	}
	if err := raw.Control.Validate(); err != nil {
		return loop.ActionOutput{}, err
	}
	return loop.ActionOutput{
		Structured:    append([]byte(nil), raw.Structured...),
		Value:         raw.Value,
		Text:          raw.Text,
		SessionID:     raw.SessionID,
		EventStartSeq: raw.EventStartSeq,
		EventEndSeq:   raw.EventEndSeq,
		TokensUsed:    raw.TokensUsed,
		Status:        raw.Status,
	}, nil
}

type segmentState struct {
	input              loop.ActionExecutionInput
	key                TurnKey
	node               dsl.Node
	params             dsl.GoalParams
	checkpoint         Checkpoint
	binding            loop.ActionSessionBinding
	usage              *usageTracker
	lastResult         loop.ActionPromptResult
	lastVerdict        string
	lastBlockingIssues []gate.BlockingIssue
}

type turnBoundary struct {
	checkpoint Checkpoint
	result     loop.ActionPromptResult
	verdict    *gate.Verdict
	control    *loop.ActionControl
	completed  bool
}
