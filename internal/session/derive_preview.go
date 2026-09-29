package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// DerivePreview reports what a continue or fork of the source would carry right now,
// from the same snapshot and bounding the derive uses. It never writes and never
// repairs the source.
func (m *Manager) DerivePreview(
	ctx context.Context,
	workspaceID string,
	sourceSessionID string,
	messageID string,
) (DerivePreview, error) {
	if m == nil {
		return DerivePreview{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return DerivePreview{}, errors.New("session: derive preview context is required")
	}
	spec := deriveSpec{
		kind:        deriveKindForMessage(messageID),
		workspaceID: strings.TrimSpace(workspaceID),
		sourceID:    strings.TrimSpace(sourceSessionID),
		messageID:   strings.TrimSpace(messageID),
	}
	if spec.sourceID == "" || spec.workspaceID == "" {
		return DerivePreview{}, fmt.Errorf("%w: workspace and source session id are required", ErrValidation)
	}
	snapshot, release, err := m.readDeriveSnapshot(ctx, spec.sourceID, spec.messageID)
	if err != nil {
		return DerivePreview{}, err
	}
	release()
	if err := m.validateDeriveSource(ctx, spec, snapshot); err != nil {
		return DerivePreview{}, err
	}
	workspace, err := m.resolveResumeWorkspace(ctx, snapshot.meta)
	if err != nil {
		return DerivePreview{}, err
	}
	imported, err := m.buildImportedContext(snapshot, spec, m.deriveBudget(&workspace))
	if err != nil {
		return DerivePreview{}, err
	}
	// imported counts inherited plus cut messages (carried + omitted); the whole source
	// adds whatever lies after the cut.
	cutTotal := imported.MessageCount + imported.OmittedCount
	preview := DerivePreview{
		MessageCount: imported.MessageCount, ReplayBytes: imported.Bytes,
		SourceMessageCount: cutTotal + max(snapshot.wholeMessageCount-len(snapshot.messages), 0),
		OmittedCount:       imported.OmittedCount, Truncated: imported.Truncated,
		SourceTurnInProgress: imported.SourceTurnInProgress,
		Epoch:                snapshot.epoch, Generation: snapshot.generation, MaxSequence: snapshot.maxSequence,
	}
	if spec.messageID != "" {
		cut := snapshot.cut
		preview.Cut = &cut
	} else {
		preview.NativeForkPossible = m.previewNativeForkPossible(ctx, &snapshot)
	}
	return preview, nil
}

func deriveKindForMessage(messageID string) store.LineageKind {
	if strings.TrimSpace(messageID) != "" {
		return store.LineageKindFork
	}
	return store.LineageKindContinue
}
