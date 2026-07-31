package acp

import speedpkg "github.com/compozy/compozy/internal/speed"

// PromptRuntime is the immutable effective runtime attached to every event in
// one prompt turn.
type PromptRuntime struct {
	Provider        string         `json:"provider"`
	Model           string         `json:"model,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed `json:"speed,omitempty"`
}

// ClonePromptRuntime returns an isolated prompt-runtime snapshot.
func ClonePromptRuntime(runtime *PromptRuntime) *PromptRuntime {
	if runtime == nil {
		return nil
	}
	cloned := *runtime
	return &cloned
}

// WithPromptRuntime returns an event carrying an isolated effective runtime snapshot.
func (e AgentEvent) WithPromptRuntime(runtime *PromptRuntime) AgentEvent {
	payload := e.clonePayload()
	payload.promptRuntime = ClonePromptRuntime(runtime)
	e.payload = normalizeAgentEventPayload(payload)
	return e
}

// PromptRuntimeSnapshot returns an isolated effective runtime snapshot.
func (e AgentEvent) PromptRuntimeSnapshot() *PromptRuntime {
	if e.payload == nil {
		return nil
	}
	return ClonePromptRuntime(e.payload.promptRuntime)
}
