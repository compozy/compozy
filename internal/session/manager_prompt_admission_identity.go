package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
)

const sessionPromptFingerprintVersion = "session-prompt/v5"

type promptAdmissionLock struct {
	mu   sync.Mutex
	refs int
}

func (r promptRequest) hasPromptAdmissionIdentity() bool {
	return strings.TrimSpace(r.messageID) != "" && strings.TrimSpace(r.idempotencyKey) != ""
}

func (r promptRequest) validatePromptAdmissionIdentity() error {
	messageID := strings.TrimSpace(r.messageID)
	idempotencyKey := strings.TrimSpace(r.idempotencyKey)
	if (messageID == "") == (idempotencyKey == "") {
		return nil
	}
	return errors.New("session: message id and idempotency key must be provided together")
}

func (m *Manager) lockPromptAdmission(workspaceID string, sessionID string, idempotencyKey string) func() {
	key := strings.Join([]string{
		strings.TrimSpace(workspaceID),
		strings.TrimSpace(sessionID),
		strings.TrimSpace(idempotencyKey),
	}, "\x00")
	m.promptAdmissionMu.Lock()
	lock := m.promptAdmissionLocks[key]
	if lock == nil {
		lock = &promptAdmissionLock{}
		m.promptAdmissionLocks[key] = lock
	}
	lock.refs++
	m.promptAdmissionMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		m.promptAdmissionMu.Lock()
		lock.refs--
		if lock.refs == 0 && m.promptAdmissionLocks[key] == lock {
			delete(m.promptAdmissionLocks, key)
		}
		m.promptAdmissionMu.Unlock()
	}
}

func sessionPromptWorkspaceID(session *Session) (string, error) {
	if session == nil || session.Info() == nil {
		return "", errors.New("session: prompt workspace is unavailable")
	}
	workspaceID := strings.TrimSpace(session.Info().WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("session: prompt workspace is unavailable")
	}
	return workspaceID, nil
}

func (m *Manager) newPromptAdmissionRequest(
	workspaceID string,
	req promptRequest,
	operation string,
	mode BusyInputMode,
) (store.SessionPromptAdmissionRequest, error) {
	fingerprint, err := promptAdmissionFingerprint(operation, mode, req)
	if err != nil {
		return store.SessionPromptAdmissionRequest{}, err
	}
	admissionID, err := store.NewID("pad")
	if err != nil {
		return store.SessionPromptAdmissionRequest{}, fmt.Errorf("session: generate prompt admission id: %w", err)
	}
	eventID, err := store.NewID("ev")
	if err != nil {
		return store.SessionPromptAdmissionRequest{}, fmt.Errorf("session: generate prompt admission event id: %w", err)
	}
	return store.SessionPromptAdmissionRequest{
		ID:                 admissionID,
		WorkspaceID:        workspaceID,
		SessionID:          req.target,
		MessageID:          req.messageID,
		IdempotencyKey:     req.idempotencyKey,
		Operation:          operation,
		FingerprintVersion: sessionPromptFingerprintVersion,
		RequestFingerprint: fingerprint,
		Mode:               string(mode),
		AuthoredText:       req.authoredMessage,
		Runtime:            storeRuntimeSelection(req.runtime),
		SkillInvocations:   append([]commandpkg.Invocation(nil), req.skillInvocations...),
		Attachments:        storeAttachments(req.attachments),
		TurnID:             req.turnID,
		EventID:            eventID,
		Now:                m.now(),
	}, nil
}

func promptAdmissionFingerprint(
	operation string,
	mode BusyInputMode,
	req promptRequest,
) (string, error) {
	runtime := storeRuntimeSelection(req.runtime).Normalize()
	attachmentIdentities := attachmentFingerprintIdentities(req.attachments)
	slices.Sort(attachmentIdentities)
	canonical := struct {
		Version          string                            `json:"version"`
		Operation        string                            `json:"operation"`
		MessageID        string                            `json:"message_id"`
		AuthoredText     string                            `json:"authored_text"`
		Mode             string                            `json:"mode"`
		Provider         string                            `json:"provider"`
		Model            string                            `json:"model"`
		ReasoningEffort  string                            `json:"reasoning_effort"`
		Speed            string                            `json:"speed"`
		ACPOptions       []store.SessionACPOptionSelection `json:"acp_options,omitempty"`
		ExpectedTurnID   string                            `json:"expected_turn_id,omitempty"`
		SkillInvocations []commandpkg.Invocation           `json:"skill_invocations,omitempty"`
		AttachmentIDs    []string                          `json:"attachment_ids,omitempty"`
		PromptMeta       acp.PromptMeta                    `json:"prompt_meta,omitempty"`
	}{
		Version: sessionPromptFingerprintVersion, Operation: strings.TrimSpace(operation),
		MessageID: strings.TrimSpace(req.messageID), AuthoredText: strings.TrimSpace(req.authoredMessage),
		Mode: strings.TrimSpace(string(mode)), Provider: runtime.Provider, Model: runtime.Model,
		ReasoningEffort: runtime.ReasoningEffort, Speed: runtime.Speed,
		ACPOptions:       store.CloneSessionACPOptionSelections(runtime.ACPOptions),
		ExpectedTurnID:   strings.TrimSpace(req.expectedTurnID),
		SkillInvocations: append([]commandpkg.Invocation(nil), req.skillInvocations...),
		AttachmentIDs:    attachmentIdentities,
		PromptMeta:       req.meta.Normalize(),
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("session: encode prompt admission fingerprint: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func (m *Manager) claimPromptAdmission(
	ctx context.Context,
	req store.SessionPromptAdmissionRequest,
) (store.SessionPromptAdmission, *SendPromptResult, error) {
	if m.promptAdmissionStore == nil {
		return store.SessionPromptAdmission{}, nil, errors.New(
			"session: prompt admission store is not configured",
		)
	}
	admission, _, err := m.promptAdmissionStore.ClaimSessionPromptAdmission(ctx, req)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	if admission.State != store.SessionPromptAdmissionCompleted {
		return admission, nil, nil
	}
	replayed, err := sendPromptResultFromAdmission(admission)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	replayed.Replayed = true
	return admission, &replayed, nil
}

func bindPromptAdmissionRequest(req promptRequest, admission store.SessionPromptAdmission) promptRequest {
	req.turnID = admission.TurnID
	req.eventID = admission.EventID
	req.messageID = admission.MessageID
	req.idempotencyKey = admission.IdempotencyKey
	req.runtime = runtimeSelectionFromStore(admission.Runtime)
	req.skillInvocations = append([]commandpkg.Invocation(nil), admission.SkillInvocations...)
	req.attachments = attachmentMetaFromStore(admission.Attachments)
	return req
}

func sendPromptResultFromAdmission(admission store.SessionPromptAdmission) (SendPromptResult, error) {
	if admission.Result == nil {
		return SendPromptResult{}, errors.New("session: completed prompt admission has no result")
	}
	stored := admission.Result
	result := SendPromptResult{
		Status: stored.Status, Mode: BusyInputMode(stored.Mode),
		Delivery:  stored.Delivery,
		MessageID: admission.MessageID, IdempotencyKey: admission.IdempotencyKey,
		QueueEntryID: stored.QueueEntryID, QueuePosition: stored.QueuePosition,
		QueueGeneration: stored.QueueGeneration, PreviousTurnID: stored.PreviousTurnID,
		NewTurnID: stored.NewTurnID, CanceledQueuedEntries: stored.CanceledQueuedEntries,
	}
	if len(stored.Goal) > 0 {
		var goal GoalCommandResult
		if err := json.Unmarshal(stored.Goal, &goal); err != nil {
			return SendPromptResult{}, fmt.Errorf("session: decode replayed Goal result: %w", err)
		}
		result.Goal = &goal
	}
	return result, nil
}

func sessionPromptAdmissionResult(result SendPromptResult) (store.SessionPromptAdmissionResult, error) {
	stored := store.SessionPromptAdmissionResult{
		Status: result.Status, Mode: string(result.Mode), Delivery: result.Delivery,
		QueueEntryID:  result.QueueEntryID,
		QueuePosition: result.QueuePosition, QueueGeneration: result.QueueGeneration,
		PreviousTurnID: result.PreviousTurnID, NewTurnID: result.NewTurnID,
		CanceledQueuedEntries: result.CanceledQueuedEntries,
	}
	if result.Goal != nil {
		encoded, err := json.Marshal(result.Goal)
		if err != nil {
			return store.SessionPromptAdmissionResult{}, fmt.Errorf("session: encode Goal prompt result: %w", err)
		}
		stored.Goal = encoded
	}
	return stored, nil
}
