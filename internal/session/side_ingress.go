package session

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	commandpkg "github.com/compozy/compozy/internal/command"
)

type SideCommandResult struct {
	SessionID            string
	Seed                 string
	SourceTurnInProgress bool
}

type SidePromptError struct {
	Result DeriveResult
	Cause  error
}

func (e *SidePromptError) Error() string { return e.Cause.Error() }
func (e *SidePromptError) Unwrap() error { return e.Cause }

func ParseSideCommand(message string) (string, bool, error) {
	message = strings.TrimSpace(message)
	token, rest := message, ""
	if index := strings.IndexFunc(message, unicode.IsSpace); index >= 0 {
		token, rest = message[:index], message[index:]
	}
	if token != "/side" {
		return "", false, nil
	}
	question := strings.TrimSpace(rest)
	if question == "" {
		return "", true, ErrSideMessageRequired
	}
	return question, true, nil
}

func (m *Manager) sidePromptIngress(ctx context.Context, id string, opts SendPromptOpts) (*SendPromptResult, error) {
	if err := validateCommandIngress(opts.AllowCommands, opts.Caller); err != nil {
		return nil, err
	}
	if err := m.requireSideParentUnarchived(ctx, id); err != nil {
		return nil, err
	}
	if !opts.AllowCommands {
		return nil, nil
	}
	question, matched, parseErr := ParseSideCommand(opts.Message)
	if !matched {
		return nil, m.requireSideCommandSupported(ctx, id, opts.Message)
	}
	meta, err := m.readSessionMetaReadOnly(ctx, id)
	if err != nil {
		return nil, err
	}
	if isSideLineage(meta.Lineage) {
		return nil, ErrSideNested
	}
	if parseErr != nil {
		return nil, parseErr
	}
	if len(opts.Attachments) != 0 {
		return nil, ErrSideAttachmentsUnsupported
	}
	if strings.TrimSpace(opts.MessageID) == "" || strings.TrimSpace(opts.IdempotencyKey) == "" {
		return nil, fmt.Errorf("%w: message id and idempotency key are required", ErrValidation)
	}
	if err := opts.Caller.Validate(); err != nil {
		return nil, err
	}
	derived, err := m.OpenSide(ctx, OpenSideOpts{
		SourceSessionID: meta.ID, WorkspaceID: meta.WorkspaceID, ProfileID: meta.ProfileID,
		Message: question, IdempotencyKey: "side:" + meta.ID + ":" + strings.TrimSpace(opts.IdempotencyKey),
	})
	if err != nil {
		if derived.ChildSessionID != "" {
			return nil, &SidePromptError{Result: derived, Cause: err}
		}
		return nil, err
	}
	if derived.ChildDeleted {
		return nil, &SidePromptError{Result: derived, Cause: ErrSessionNotFound}
	}
	if derived.Child != nil && derived.Child.ArchivedAt != nil {
		return nil, &SidePromptError{Result: derived, Cause: ErrSessionArchived}
	}
	return &SendPromptResult{
		Status: "side_opened", Delivery: string(ChecksNone), TargetSessionID: meta.ID,
		IdempotencyKey: strings.TrimSpace(opts.IdempotencyKey), Replayed: derived.Replayed,
		Side: &SideCommandResult{
			SessionID:            derived.ChildSessionID,
			Seed:                 string(derived.Seed),
			SourceTurnInProgress: derived.SourceTurnInProgress,
		},
	}, nil
}

func (m *Manager) requireSideCommandSupported(ctx context.Context, id, message string) error {
	_, goal, err := ParseGoalCommand(message)
	if goal {
		return m.RequireGoalSupported(ctx, id)
	}
	if err != nil {
		return err
	}
	if token := strings.Fields(message); len(token) > 0 && token[0] == "/worktree" {
		return m.refuseSideOperation(ctx, id, ErrSideWorktreeFixed)
	}
	return nil
}

func applySideCommandAvailability(catalog commandpkg.Catalog, info *Info) commandpkg.Catalog {
	if info == nil {
		return catalog
	}
	if isSideLineage(info.Lineage) {
		for _, item := range []struct{ name, reason string }{
			{"side", SideErrorMessage(ErrSideNested)}, {managedInputOwnerGoal, SideErrorMessage(ErrSideGoalUnsupported)}, {"worktree", SideErrorMessage(ErrSideWorktreeFixed)},
		} {
			catalog = commandpkg.SetBuiltinAvailability(catalog, item.name, false, item.reason)
		}
		return catalog
	}
	if info.Type != SessionTypeUser || (info.Lineage != nil && info.Lineage.Kind == "spawn") {
		return commandpkg.WithoutBuiltin(catalog, "side")
	}
	return catalog
}

// preparePromptIngress handles side commands before ordinary prompt-slot admission.
func (m *Manager) preparePromptIngress(
	ctx context.Context,
	id string,
	opts SendPromptOpts,
) (sendPromptPreparation, *SendPromptResult, error) {
	if _, err := m.normalizeBusyInputMode(opts.Mode); err != nil {
		return sendPromptPreparation{}, nil, err
	}
	result, err := m.sidePromptIngress(ctx, id, opts)
	if result != nil || err != nil {
		return sendPromptPreparation{}, result, err
	}
	return m.prepareSendPrompt(ctx, id, opts)
}
