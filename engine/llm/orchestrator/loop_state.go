package orchestrator

import (
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
)

// loopState tracks runtime and serializable aspects of the orchestrator loop.
// JSON-annotated sub-structures allow future persistence without relying on
// the transient runtime fields captured in runtimeState.
type loopState struct {
	Iteration iterationState `json:"iteration"`
	Budgets   budgetState    `json:"budgets"`
	Progress  progressState  `json:"progress"`
	Memory    memoryState    `json:"memory"`
	runtime   runtimeState
}

// iterationState captures loop counters suitable for snapshotting.
type iterationState struct {
	Current       int `json:"current"`
	MaxIterations int `json:"max_iterations"`
	Restarts      int `json:"restarts"`
}

// budgetState tracks tool/error budgets and recent tool fingerprints.
type budgetState struct {
	ErrorBudget         int               `json:"error_budget"`
	FinalizeRetryBudget int               `json:"finalize_retry_budget"`
	FinalizeAttempts    int               `json:"finalize_attempts"`
	ToolErrors          map[string]int    `json:"tool_errors,omitempty"`
	ToolSuccess         map[string]int    `json:"tool_success,omitempty"`
	ToolUsage           map[string]int    `json:"tool_usage,omitempty"`
	LastToolResults     map[string]string `json:"last_tool_results,omitempty"`
}

// progressState aggregates fingerprint-based progress tracking metadata.
type progressState struct {
	NoProgressCount int    `json:"no_progress_count"`
	LastFingerprint string `json:"last_fingerprint,omitempty"`
}

// memoryState surfaces memory references and compaction bookkeeping.
type memoryState struct {
	References               []core.MemoryReference `json:"references,omitempty"`
	CompactionSuggested      bool                   `json:"compaction_suggested"`
	LastCompactionIteration  int                    `json:"last_compaction_iteration"`
	LastCompactionThreshold  float64                `json:"last_compaction_threshold"`
	PendingCompactionPercent float64                `json:"pending_compaction_percent"`
	CompactionFailures       int                    `json:"compaction_failures"`
}

type runtimeState struct {
	memories             *MemoryContext
	action               *agent.ActionConfig
	caps                 toolCallCaps
	finalizeFeedbackBase int
}

func newLoopState(cfg *settings, memories *MemoryContext, action *agent.ActionConfig) *loopState {
	if cfg == nil {
		cfg = &settings{}
	}
	budget := initBudgetState(cfg)
	iteration := initIterationState(cfg)
	memState := memoryState{}
	if memories != nil {
		if refs := memories.References(); len(refs) > 0 {
			memState.References = refs
		}
	}
	return &loopState{
		Iteration: iteration,
		Budgets:   budget,
		Memory:    memState,
		runtime: runtimeState{
			memories:             memories,
			action:               action,
			caps:                 cfg.toolCaps,
			finalizeFeedbackBase: -1, // -1 indicates no finalize feedback has been recorded yet
		},
	}
}

func initBudgetState(cfg *settings) budgetState {
	budget := budgetState{
		ErrorBudget:         cfg.maxSequentialToolErrors,
		FinalizeRetryBudget: cfg.finalizeOutputRetries,
		ToolErrors:          make(map[string]int),
		ToolSuccess:         make(map[string]int),
		ToolUsage:           make(map[string]int),
		LastToolResults:     make(map[string]string),
	}
	if budget.ErrorBudget <= 0 {
		budget.ErrorBudget = defaultMaxSequentialToolErrors
	}
	if budget.FinalizeRetryBudget <= 0 {
		budget.FinalizeRetryBudget = defaultStructuredOutputRetries
	}
	return budget
}

func initIterationState(cfg *settings) iterationState {
	iteration := iterationState{MaxIterations: cfg.maxToolIterations}
	if iteration.MaxIterations <= 0 {
		iteration.MaxIterations = defaultMaxToolIterations
	}
	return iteration
}

func (s *loopState) budgetFor(string) int {
	if s == nil {
		return defaultMaxSequentialToolErrors
	}
	if s.Budgets.ErrorBudget <= 0 {
		return defaultMaxSequentialToolErrors
	}
	return s.Budgets.ErrorBudget
}

func (s *loopState) incrementUsage(tool string) int {
	if s == nil {
		return 0
	}
	if s.Budgets.ToolUsage == nil {
		s.Budgets.ToolUsage = make(map[string]int)
	}
	key := canonicalToolName(tool)
	if key == "" {
		key = tool
	}
	s.Budgets.ToolUsage[key]++
	return s.Budgets.ToolUsage[key]
}

func (s *loopState) limitFor(tool string) int {
	return s.runtime.caps.limitFor(tool)
}

func (s *loopState) allowFinalizeRetry() bool {
	if s == nil {
		return false
	}
	budget := s.Budgets.FinalizeRetryBudget
	if budget <= 0 {
		return false
	}
	if s.Budgets.FinalizeAttempts >= budget {
		return false
	}
	s.Budgets.FinalizeAttempts++
	return true
}

func (s *loopState) remainingFinalizeRetries() int {
	if s == nil {
		return 0
	}
	remaining := s.Budgets.FinalizeRetryBudget - s.Budgets.FinalizeAttempts
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s *loopState) finalizeAttemptNumber() int {
	if s == nil {
		return 0
	}
	return s.Budgets.FinalizeAttempts
}

func (s *loopState) finalizeBudget() int {
	if s == nil {
		return 0
	}
	return s.Budgets.FinalizeRetryBudget
}

func (s *loopState) memories() *MemoryContext {
	if s == nil {
		return nil
	}
	return s.runtime.memories
}

func (s *loopState) actionConfig() *agent.ActionConfig {
	if s == nil {
		return nil
	}
	return s.runtime.action
}

func (s *loopState) setMemories(mem *MemoryContext) {
	if s == nil {
		return
	}
	s.runtime.memories = mem
	if mem == nil {
		s.Memory.References = nil
		return
	}
	if refs := mem.References(); len(refs) > 0 {
		s.Memory.References = refs
		return
	}
	s.Memory.References = nil
}

func (s *loopState) resetBudgets(cfg *settings) {
	if s == nil {
		return
	}
	s.Budgets.ToolErrors = make(map[string]int)
	s.Budgets.ToolSuccess = make(map[string]int)
	s.Budgets.ToolUsage = make(map[string]int)
	s.Budgets.LastToolResults = make(map[string]string)
	s.Budgets.FinalizeAttempts = 0
	if cfg != nil {
		if cfg.maxSequentialToolErrors > 0 {
			s.Budgets.ErrorBudget = cfg.maxSequentialToolErrors
		}
		if cfg.finalizeOutputRetries > 0 {
			s.Budgets.FinalizeRetryBudget = cfg.finalizeOutputRetries
		}
	}
}

func (s *loopState) resetProgress() {
	if s == nil {
		return
	}
	s.Progress.NoProgressCount = 0
	s.Progress.LastFingerprint = ""
}

func (s *loopState) recordFingerprint(fingerprint string) int {
	if s == nil {
		return 0
	}
	if fingerprint == "" {
		s.resetProgress()
		return 0
	}
	if fingerprint == s.Progress.LastFingerprint {
		s.Progress.NoProgressCount++
		return s.Progress.NoProgressCount
	}
	s.Progress.LastFingerprint = fingerprint
	s.Progress.NoProgressCount = 0
	return s.Progress.NoProgressCount
}

func (s *loopState) incrementRestart() int {
	if s == nil {
		return 0
	}
	s.Iteration.Restarts++
	return s.Iteration.Restarts
}

func (s *loopState) markCompaction(threshold float64, percent float64) {
	if s == nil {
		return
	}
	s.Memory.CompactionSuggested = true
	s.Memory.LastCompactionThreshold = threshold
	s.Memory.PendingCompactionPercent = percent
}

func (s *loopState) completeCompaction(iteration int) {
	if s == nil {
		return
	}
	s.Memory.CompactionSuggested = false
	s.Memory.LastCompactionIteration = iteration
	s.Memory.PendingCompactionPercent = 0
	s.Memory.CompactionFailures = 0
	s.Memory.LastCompactionThreshold = 0
}

func (s *loopState) compactionPending(iteration int, cooldown int) bool {
	if s == nil {
		return false
	}
	if !s.Memory.CompactionSuggested {
		return false
	}
	if cooldown <= 0 {
		return true
	}
	if s.Memory.LastCompactionIteration == 0 {
		return true
	}
	return iteration-s.Memory.LastCompactionIteration >= cooldown
}

func (s *loopState) recordCompactionFailure(iteration int) int {
	if s == nil {
		return 0
	}
	s.Memory.CompactionSuggested = true
	s.Memory.LastCompactionIteration = iteration
	s.Memory.CompactionFailures++
	return s.Memory.CompactionFailures
}

func (s *loopState) resetCompaction() {
	if s == nil {
		return
	}
	refs := s.Memory.References
	s.Memory = memoryState{References: refs}
}
