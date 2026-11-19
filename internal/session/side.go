package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

var (
	ErrSideMessageRequired        = errors.New("Write a question for the side chat.")
	ErrSideAttachmentsUnsupported = errors.New("Side chat questions can't include files yet. Send the question alone.")
	ErrSideSourceEmpty            = errors.New("Send a message first, then open a side chat.")
	ErrSideNested                 = errors.New("Side chats can't open side chats.")
	ErrSideUnsupported            = errors.New("Side chats are available on your own sessions.")
	ErrSideParentArchived         = store.ErrSessionSideParentArchived
	ErrSideNotDerivable           = errors.New("Side chats can't be forked or continued. Use the main session.")
	ErrSideFixedParent            = errors.New("Side chats stay with their session.")
	ErrSideWorktreeFixed          = errors.New("Change worktrees in the main session.")
	ErrSideLaterUnsupported       = errors.New("Side chats can't be parked. Park the main session instead.")
	ErrSideGoalUnsupported        = errors.New("Set Goals in the main session.")
	ErrSideSpawnUnsupported       = errors.New("Start that from the main session.")
)

const sideBoundary = `<compozy_side_chat>
Everything before this boundary is history inherited from the main session. It is reference only, not your task. Only messages after this boundary are active.
Do not continue, execute, or complete plans, tool calls, approvals, or edits from the inherited history.
You are a side chat: answer questions and do read-only inspection without disrupting the main session.
Do not modify files, git state, permissions, configuration, or workspace state unless the user explicitly asks in this side chat. Keep any requested change minimal.
Do not start subagents, Loops, Goals, monitors, or scheduled work, and do not change worktrees. Point the user to the main session for those.
</compozy_side_chat>`

type OpenSideOpts struct {
	SourceSessionID string
	WorkspaceID     string
	ProfileID       string
	Message         string
	Name            string
	IdempotencyKey  string
}

func (m *Manager) OpenSide(ctx context.Context, opts OpenSideOpts) (DeriveResult, error) {
	if ctx == nil || m == nil {
		return DeriveResult{}, fmt.Errorf("%w: manager and context are required", ErrValidation)
	}
	if strings.TrimSpace(opts.Message) == "" {
		return DeriveResult{}, ErrSideMessageRequired
	}
	return m.deriveSession(ctx, deriveSpec{
		kind: store.LineageKindSide, sourceID: strings.TrimSpace(opts.SourceSessionID),
		workspaceID: strings.TrimSpace(opts.WorkspaceID), profileID: strings.TrimSpace(opts.ProfileID),
		sourceProfileID: strings.TrimSpace(opts.ProfileID), message: strings.TrimSpace(opts.Message),
		name: strings.TrimSpace(opts.Name), key: strings.TrimSpace(opts.IdempotencyKey),
	})
}

func isSideLineage(lineage *store.SessionLineage) bool {
	return lineage != nil && lineage.Kind == store.LineageKindSide
}

func (m *Manager) snapshotSideRuntime(snapshot *deriveSnapshot) {
	meta := &snapshot.meta
	if live, ok := m.Get(meta.ID); ok {
		live.mu.RLock()
		*meta = live.metaLocked()
		info := live.infoLocked()
		live.mu.RUnlock()
		runtime := info.EffectiveRuntime()
		meta.Provider, meta.Model, meta.ReasoningEffort = runtime.Provider, runtime.Model, runtime.ReasoningEffort
		meta.Speed = runtime.Speed
		meta.SetACPOptions(storeOptionSelectionsFromACP(runtime.ACPOptions))
	} else if selected, _ := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection); selected != nil {
		meta.Provider, meta.Model, meta.ReasoningEffort = selected.Provider, selected.Model, selected.ReasoningEffort
		meta.Speed = selected.Speed
		meta.SetACPOptions(selected.ACPOptions)
	}
}

func applySideCreate(opts *CreateOpts, meta *store.SessionMeta, name string) {
	opts.Name = strings.TrimSpace(name)
	opts.Permissions = compozyconfig.PermissionMode(meta.EffectivePermissionsValue())
	if opts.Permissions == "" && meta.CreationProfile != nil {
		opts.Permissions = compozyconfig.PermissionMode(meta.CreationProfile.Permissions)
	}
	opts.PromptOverlay = sideBoundary
	lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
	opts.Lineage.PermissionPolicy = store.NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
}

func validateUserParentLineageShape(lineage *store.SessionLineage) error {
	if !isSideLineage(lineage) {
		return validateProvenanceLineageShape(lineage)
	}
	copy := *lineage
	copy.PermissionPolicy = store.SessionPermissionPolicy{}
	return validateProvenanceLineageShape(&copy)
}

func sideInProgressNote(source string, at time.Time) transcript.Message {
	note := abortedReplayNote(at)
	note.ID = "sys_turn_in_progress"
	note.Content = "<compozy_turn_in_progress>The source session was still working on this turn " +
		"when this side chat was opened. The messages above are a snapshot of it; the source continues after this point. " +
		"Use compozy__session_history on " + source + " if the user asks what happened since.</compozy_turn_in_progress>"
	return note
}

func sideDispatchMessage(s *Session, req *promptRequest, replay, question string) string {
	meta := s.Meta()
	if isSideLineage(meta.Lineage) && meta.ImportedContext != nil &&
		meta.ImportedContext.Consumed == nil && req.delivery == PromptDeliveryNormal {
		question = sideBoundary + "\n\n" + question
	}
	return promptWithResumeReplay(replay, question)
}

func (m *Manager) ListSides(ctx context.Context, workspaceID, sourceID string, includeArchived bool) ([]*Info, error) {
	source, err := m.readSessionMetaReadOnly(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	if source.WorkspaceID != strings.TrimSpace(workspaceID) {
		return nil, ErrSessionNotFound
	}
	if isSideLineage(source.Lineage) {
		return nil, ErrSideNested
	}
	if normalizeSessionType(Type(source.SessionType)) != SessionTypeUser ||
		(source.Lineage != nil && source.Lineage.Kind == store.LineageKindSpawn) {
		return nil, ErrSideUnsupported
	}
	sides, ok := m.sessionCatalog.(store.SessionSideStore)
	if !ok {
		return nil, ErrSessionArchiveUnavailable
	}
	rows, err := sides.ListSessionSides(ctx, workspaceID, source.ID, includeArchived)
	if err != nil {
		return nil, err
	}
	infos := sessionInfosFromCatalog(rows)
	for i, info := range infos {
		if live, ok := m.Get(info.ID); ok {
			infos[i] = live.Info()
			infos[i].ArchivedAt = info.ArchivedAt
		}
	}
	return infos, nil
}

func (m *Manager) refuseSideOperation(ctx context.Context, id string, refusal error) error {
	meta, err := m.readSessionMetaReadOnly(ctx, id)
	if err != nil {
		return err
	}
	if isSideLineage(meta.Lineage) {
		return refusal
	}
	return nil
}

func (m *Manager) requireSideParentUnarchived(ctx context.Context, workspaceID, id string) error {
	meta, err := m.readSessionMetaReadOnly(ctx, id)
	if err != nil {
		return err
	}
	if !isSideLineage(meta.Lineage) {
		return nil
	}
	workspaceID = meta.WorkspaceID
	if err := m.requireSessionUnarchived(ctx, workspaceID, meta.Lineage.ParentSessionID); err != nil {
		if errors.Is(err, ErrSessionArchived) {
			return ErrSideParentArchived
		}
		return err
	}
	return nil
}
