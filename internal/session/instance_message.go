package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// Named sends always enter the durable queue. Metadata is prepared optimistically;
// the store checks the current pointer in the enqueue transaction and the native
// caller re-resolves changed targets, including their message and workspace guards.
func (m *Manager) enqueueNamedInstancePrompt(
	ctx context.Context,
	prep sendPromptPreparation,
	target store.InstancePromptTarget,
) (SendPromptResult, error) {
	req := prep.request
	if !req.hasPromptAdmissionIdentity() {
		return SendPromptResult{}, errors.New("session: named message requires admission identity")
	}
	meta, err := m.readMetaWithContext(ctx, req.target)
	if err != nil {
		return SendPromptResult{}, err
	}
	if meta.ProfileID != target.ProfileID || meta.WorkspaceID != target.WorkspaceID ||
		meta.AgentName != target.AgentName {
		return SendPromptResult{}, errors.New("session: instance message scope mismatch")
	}
	req.runtime, err = normalizePromptRuntimeSelectionFromMeta(&meta, req.runtime)
	if err != nil {
		return SendPromptResult{}, err
	}
	if err := m.validateRuntimeModelAtAdmission(ctx, nil, *req.runtime, &meta); err != nil {
		return SendPromptResult{}, err
	}
	admission, err := m.newPromptAdmissionRequest(
		target.WorkspaceID,
		req,
		store.SessionPromptOperationPrompt,
		prep.mode,
	)
	if err != nil {
		return SendPromptResult{}, err
	}
	admission.InstanceTarget = &target
	admission.Mode = string(BusyInputModeQueue)
	if err := m.canonicalizeAdmissionPromptAttachments(
		ctx,
		target.WorkspaceID,
		req.target,
		&req,
		&admission,
	); err != nil {
		return SendPromptResult{}, err
	}
	return m.enqueueAdmittedOfflinePrompt(ctx, req.target, admission, false, req.supersede)
}
