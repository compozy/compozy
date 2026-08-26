package session

import (
	"github.com/compozy/compozy/internal/acp"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

// CreationProfileInput is the resolved, secret-free creation policy material.
type CreationProfileInput struct {
	AgentName       string
	Provider        string
	Model           string
	ReasoningEffort string
	Speed           speedpkg.Speed
	ACPOptions      []acp.SessionConfigOptionSelection
	ProfileID       string
	Scope           store.SessionScope
	WorkspaceID     string
	CWD             string
	WorktreeRef     string
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

// BuildCreationProfile constructs the current canonical profile used by daemon and Manager.
func BuildCreationProfile(input CreationProfileInput) store.SessionCreationProfile {
	return store.NormalizeSessionCreationProfile(store.SessionCreationProfile{
		Version:         store.SessionCreationProfileVersion,
		AgentName:       input.AgentName,
		Provider:        input.Provider,
		Model:           input.Model,
		ReasoningEffort: input.ReasoningEffort,
		Speed:           input.Speed,
		ACPOptions:      storeOptionSelectionsFromACP(input.ACPOptions),
		ProfileID:       input.ProfileID,
		Scope:           input.Scope,
		WorkspaceID:     input.WorkspaceID,
		CWD:             input.CWD,
		WorktreeRef:     input.WorktreeRef,
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
