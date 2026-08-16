package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

type promptAdmissionTarget struct {
	session     *Session
	workspaceID string
	runtime     *RuntimeSelection
}

func (m *Manager) canonicalizeAdmissionPromptAttachments(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	req *promptRequest,
	admission *store.SessionPromptAdmissionRequest,
) error {
	if admission == nil {
		return errors.New("session: prompt admission request is required")
	}
	if err := m.canonicalizePromptRequestAttachments(ctx, workspaceID, sessionID, req); err != nil {
		return err
	}
	admission.Attachments = storeAttachments(req.attachments)
	return nil
}

func (m *Manager) resolvePromptAdmissionTarget(
	ctx context.Context,
	target string,
	requested *RuntimeSelection,
) (promptAdmissionTarget, error) {
	if active, ok := m.Get(target); ok {
		resolved, err := m.preparePromptRuntimeSelection(ctx, active, requested)
		if err != nil {
			return promptAdmissionTarget{}, err
		}
		workspaceID, err := sessionPromptWorkspaceID(active)
		if err != nil {
			return promptAdmissionTarget{}, err
		}
		return promptAdmissionTarget{session: active, workspaceID: workspaceID, runtime: resolved}, nil
	}

	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return promptAdmissionTarget{}, err
	}
	if err := m.rejectDeadSessionAttachment(ctx, target, meta); err != nil {
		return promptAdmissionTarget{}, err
	}
	resolved, err := normalizePromptRuntimeSelectionFromMeta(meta, requested)
	if err != nil {
		return promptAdmissionTarget{}, err
	}
	return promptAdmissionTarget{workspaceID: meta.WorkspaceID, runtime: resolved}, nil
}
