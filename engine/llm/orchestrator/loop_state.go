package orchestrator

import "github.com/compozy/compozy/engine/agent"

type loopState struct {
	errorBudget           int
	structuredRetryBudget int
	toolErrors            map[string]int
	toolSuccess           map[string]int
	lastToolResults       map[string]string
	noProgressCount       int
	lastFingerprint       string
	memories              *MemoryContext
	action                *agent.ActionConfig
}

func newLoopState(cfg *settings, memories *MemoryContext, action *agent.ActionConfig) *loopState {
	if cfg == nil {
		cfg = &settings{}
	}
	return &loopState{
		errorBudget:           cfg.maxSequentialToolErrors,
		structuredRetryBudget: cfg.structuredOutputRetries,
		toolErrors:            make(map[string]int),
		toolSuccess:           make(map[string]int),
		lastToolResults:       make(map[string]string),
		memories:              memories,
		action:                action,
	}
}

func (s *loopState) budgetFor(string) int {
	if s.errorBudget <= 0 {
		return defaultMaxSequentialToolErrors
	}
	return s.errorBudget
}
