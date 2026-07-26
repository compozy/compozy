package session

import "github.com/compozy/agh/internal/store"

// CreationProfileInput is the resolved, secret-free creation policy material.
type CreationProfileInput struct {
	AgentName       string
	Provider        string
	Model           string
	ReasoningEffort string
	WorkspaceID     string
	CWD             string
	SandboxMode     string
	SandboxRef      string
	Permissions     string
	AllowedTools    []string
	AgentTools      []string
	AgentToolsets   []string
	DeniedTools     []string
	RuntimeMode     string
	PromptOverlay   string
	ContractOverlay string
}

// BuildCreationProfile constructs the canonical v1 profile used by daemon and Manager.
func BuildCreationProfile(input CreationProfileInput) store.SessionCreationProfile {
	return store.NormalizeSessionCreationProfile(store.SessionCreationProfile{
		Version:         store.SessionCreationProfileVersion,
		AgentName:       input.AgentName,
		Provider:        input.Provider,
		Model:           input.Model,
		ReasoningEffort: input.ReasoningEffort,
		WorkspaceID:     input.WorkspaceID,
		CWD:             input.CWD,
		SandboxMode:     input.SandboxMode,
		SandboxRef:      input.SandboxRef,
		Permissions:     input.Permissions,
		AllowedTools:    input.AllowedTools,
		AgentTools:      input.AgentTools,
		AgentToolsets:   input.AgentToolsets,
		DeniedTools:     input.DeniedTools,
		RuntimeMode:     input.RuntimeMode,
		PromptOverlay:   input.PromptOverlay,
		ContractOverlay: input.ContractOverlay,
	})
}
