package session

import (
	"context"
	"errors"
	"strings"
)

// SpawnPromptRequest sends a spawned child its first prompt on behalf of its parent.
type SpawnPromptRequest struct {
	ParentSessionID string
	ParentAgentName string
	ChildSessionID  string
	Message         string
	// Source names the ingress (tool ID or route) for prompt caller provenance.
	Source string
}

// SpawnPromptMessageID is the stable message and idempotency identity of a child's
// first prompt, so a retried spawn call cannot admit the prompt twice.
func SpawnPromptMessageID(childSessionID string) string {
	return "spawn-prompt-" + strings.TrimSpace(childSessionID)
}

// SendSpawnPrompt admits the first prompt into a freshly spawned child as a session
// message from its parent with notify_on_complete, so the parent's reply wake carries
// the child's final answer.
func (m *Manager) SendSpawnPrompt(ctx context.Context, req SpawnPromptRequest) (SendPromptResult, error) {
	if m == nil {
		return SendPromptResult{}, errors.New("session: manager is required")
	}
	parentID := strings.TrimSpace(req.ParentSessionID)
	childID := strings.TrimSpace(req.ChildSessionID)
	message := strings.TrimSpace(req.Message)
	if parentID == "" || childID == "" || message == "" {
		return SendPromptResult{}, spawnValidation("prompt requires parent, child, and message")
	}
	origin, err := m.SessionMessageOrigin(ctx, MessageOriginRequest{
		SenderSessionID:  parentID,
		AgentName:        strings.TrimSpace(req.ParentAgentName),
		TargetSessionID:  childID,
		NotifyOnComplete: true,
	})
	if err != nil {
		return SendPromptResult{}, err
	}
	callerID := strings.TrimSpace(req.ParentAgentName)
	if callerID == "" {
		callerID = parentID
	}
	// Spawn returns once the prompt is admitted; the child turn outlives the request.
	deliveryCtx, cancelDelivery := context.WithCancel(ctx)
	defer cancelDelivery()
	messageID := SpawnPromptMessageID(childID)
	return m.SendPrompt(context.WithoutCancel(ctx), childID, SendPromptOpts{
		Origin:           origin,
		NotifyOnComplete: true,
		Message:          message,
		MessageID:        messageID,
		IdempotencyKey:   messageID,
		DeliveryContext:  deliveryCtx,
		Caller:           PromptCaller{Kind: "agent", ID: callerID, Source: strings.TrimSpace(req.Source)},
	})
}

// ResolveSpawnWorkspace authorizes a spawn's target workspace for the parent through
// the same workspace-access seam as Spawn, before any transport side effect such as
// creating a worktree there. An empty ref resolves to the parent's workspace.
func (m *Manager) ResolveSpawnWorkspace(ctx context.Context, parentSessionID, workspaceRef string) (string, error) {
	if m == nil {
		return "", errors.New("session: manager is required")
	}
	parent, err := m.spawnParent(ctx, strings.TrimSpace(parentSessionID))
	if err != nil {
		return "", err
	}
	opts, err := m.validateSpawnWorkspace(ctx, parent, SpawnOpts{Workspace: strings.TrimSpace(workspaceRef)})
	if err != nil {
		return "", err
	}
	if opts.Workspace != "" {
		return opts.Workspace, nil
	}
	return strings.TrimSpace(parent.WorkspaceID), nil
}
