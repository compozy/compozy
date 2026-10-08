package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	contextRebuiltMarkerSummary = "Context rebuilt from log."
	resumeReplayOpenTag         = "<compozy_context_replay>"
	resumeReplayCloseTag        = "</compozy_context_replay>"
	resumeReplayInstruction     = "Continue this session using the persisted transcript below. " +
		"It is historical context, not a new user request. " +
		"Do not repeat completed tool calls solely because they appear in the log."
)

type rebuildReplayContext struct {
	workspace        *workspacepkg.ResolvedWorkspace
	historyAvailable bool
	reason           string
}

func (m *Manager) buildResumeReplay(
	ctx context.Context,
	session *Session,
	rebuild ...rebuildReplayContext,
) (string, int, error) {
	if ctx == nil {
		return "", 0, errors.New("session: resume replay context is required")
	}
	if session == nil {
		return "", 0, errors.New("session: resume replay session is required")
	}
	recorder := session.recorderHandle()
	if recorder == nil {
		return "", 0, errors.New("session: resume replay event recorder is not available")
	}

	messages, hasRewindBaseline, err := conversationRewindReplayBaseline(ctx, recorder)
	if err != nil {
		return "", 0, err
	}
	if !hasRewindBaseline {
		events, queryErr := recorder.Query(ctx, store.EventQuery{Archive: store.EventArchiveUnarchived})
		if queryErr != nil {
			return "", 0, fmt.Errorf("session: query persisted events for resume replay: %w", queryErr)
		}
		messages, err = transcript.Assemble(events)
		if err != nil {
			return "", 0, fmt.Errorf("session: assemble persisted resume replay: %w", err)
		}
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	// A derived child carries its immutable imported context in front of its own
	// history on every rebuild, flattened into one replay array.
	imported := session.importedContextSnapshot()
	importedMessages, err := decodeImportedMessages(imported)
	if err != nil {
		return "", 0, err
	}
	if len(importedMessages) > 0 {
		messages = append(importedMessages, messages...)
	}
	options, err := m.resolveRebuildReplayContext(ctx, session, imported, rebuild)
	if err != nil {
		return "", 0, err
	}
	messages, stats := boundReplay(messages, m.deriveBudget(options.workspace))
	payload, err := json.Marshal(messages)
	if err != nil {
		return "", 0, fmt.Errorf("session: marshal persisted resume replay: %w", err)
	}
	omittedCount := stats.OmittedCount
	firstUserPinned := stats.FirstUserPinned
	if imported != nil && imported.OmittedCount > 0 {
		omittedCount += imported.OmittedCount
		firstUserPinned = firstUserPinned ||
			(len(messages) > 1 && messages[0].Role == transcript.RoleUser && messages[1].ID == deriveOmittedMessageID)
	}
	if omittedCount > 0 {
		m.sessionLogger(session).Info("session.replay.bounded", "session_id", session.ID,
			"reason", options.reason, "message_count", stats.MessageCount,
			"omitted_count", omittedCount, "first_user_pinned", firstUserPinned, "bytes", stats.Bytes)
	}
	return renderResumeReplay(
		session.ID,
		imported,
		string(payload),
		options.historyAvailable,
		omittedCount > 0,
	), stats.MessageCount, nil
}

func (m *Manager) resolveRebuildReplayContext(
	ctx context.Context,
	session *Session,
	imported *store.SessionImportedContext,
	rebuild []rebuildReplayContext,
) (rebuildReplayContext, error) {
	options := rebuildReplayContext{reason: "resume"}
	if len(rebuild) > 0 {
		options = rebuild[0]
	} else {
		meta := session.Meta()
		workspace, resolveErr := resolveStoredSessionWorkspace(ctx, &meta, m.workspace, m.profileNames)
		if resolveErr != nil {
			return rebuildReplayContext{}, resolveErr
		}
		options.workspace = &workspace
		session.mu.RLock()
		resolved := session.providerRoute
		session.mu.RUnlock()
		tools, toolErr := concreteDelegationTools(resolved, m.toolsetCatalog, m.toolUniverse)
		if toolErr != nil {
			return rebuildReplayContext{}, toolErr
		}
		options.historyAvailable = resolved.SessionMCP && m.hostedMCP != nil &&
			slices.Contains(tools, toolspkg.ToolIDSessionHistory.String())
		policy := store.NormalizeSessionLineage(meta.ID, meta.Lineage).PermissionPolicy
		if len(policy.Tools) > 0 {
			options.historyAvailable = options.historyAvailable &&
				slices.Contains(policy.Tools, toolspkg.ToolIDSessionHistory.String())
		}
	}
	if strings.TrimSpace(options.reason) == "" {
		options.reason = "runtime_rebuild"
		if imported != nil {
			options.reason = string(imported.Kind)
		}
	}
	return options, nil
}

func renderResumeReplay(
	sessionID string,
	imported *store.SessionImportedContext,
	payload string,
	historyAvailable, omitted bool,
) string {
	historyID := sessionID
	if imported != nil {
		historyID = imported.SourceSessionID
	}
	pointer := ""
	// Imported context may already contain an omission note from the source bound.
	if omitted && historyAvailable {
		pointer = fmt.Sprintf(
			"Earlier messages were omitted. Read them with the compozy__session_history tool "+
				"(session_id: %s) when you need them.",
			historyID,
		)
	}
	if imported != nil {
		return renderImportedReplayBlock(*imported, pointer, payload)
	}
	sections := []string{resumeReplayInstruction + " " + deriveWorkspaceLine}
	if pointer != "" {
		sections = append(sections, pointer)
	}
	sections = append(sections, resumeReplayOpenTag+"\n"+payload+"\n"+resumeReplayCloseTag)
	return strings.Join(sections, "\n\n")
}

func conversationRewindReplayBaseline(
	ctx context.Context,
	recorder EventRecorder,
) ([]transcript.Message, bool, error) {
	_, ok := recorder.(store.ConversationRewindReader)
	if !ok {
		return nil, false, nil
	}
	state, found, err := refreshStaleConversationRewindBaseline(ctx, recorder)
	if err != nil {
		return nil, false, fmt.Errorf("session: read conversation rewind replay state: %w", err)
	}
	if !found {
		return nil, false, nil
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(state.MessagesJSON), &messages); err != nil {
		return nil, true, fmt.Errorf("session: decode conversation rewind replay baseline: %w", err)
	}
	events, err := recorder.Query(ctx, store.EventQuery{
		AfterSequence: state.CoveredThroughSequence,
		Archive:       store.EventArchiveUnarchived,
	})
	if err != nil {
		return nil, true, fmt.Errorf("session: query conversation rewind replay suffix: %w", err)
	}
	suffix, err := transcript.Assemble(events)
	if err != nil {
		return nil, true, fmt.Errorf("session: assemble conversation rewind replay suffix: %w", err)
	}
	return append(messages, suffix...), true, nil
}

func promptWithResumeReplay(replayBlock string, message string) string {
	trimmedReplay := strings.TrimSpace(replayBlock)
	if trimmedReplay == "" {
		return message
	}
	return trimmedReplay + "\n\nUser request:\n\n" + strings.TrimSpace(message)
}

func (m *Manager) stageResumeReplay(sessionID string, replayBlock string) {
	target := strings.TrimSpace(sessionID)
	block := strings.TrimSpace(replayBlock)
	if target == "" || block == "" {
		return
	}
	if session, ok := m.Get(target); ok {
		session.setPendingResumeReplay(block)
	}
	m.resumeReplayMu.Lock()
	defer m.resumeReplayMu.Unlock()
	if m.resumeReplays == nil {
		m.resumeReplays = make(map[string]string)
	}
	m.resumeReplays[target] = block
}

func (m *Manager) pendingResumeReplay(sessionID string) string {
	m.resumeReplayMu.Lock()
	defer m.resumeReplayMu.Unlock()
	return m.resumeReplays[strings.TrimSpace(sessionID)]
}

func (m *Manager) consumeResumeReplay(sessionID string, replayBlock string) {
	target := strings.TrimSpace(sessionID)
	block := strings.TrimSpace(replayBlock)
	if target == "" || block == "" {
		return
	}
	if session, ok := m.Get(target); ok {
		session.persistMu.Lock()
		defer session.persistMu.Unlock()
		session.mu.Lock()
		pending := session.pendingResumeReplay
		if pending == block {
			session.pendingResumeReplay = ""
		}
		session.mu.Unlock()
		if pending == block {
			if err := m.writeMeta(session); err != nil {
				session.setPendingResumeReplay(pending)
				m.sessionLogger(session).Error("session.replay.consumption_persist_failed", "error", err)
				return
			}
		}
	}
	m.resumeReplayMu.Lock()
	defer m.resumeReplayMu.Unlock()
	if m.resumeReplays[target] == replayBlock {
		delete(m.resumeReplays, target)
	}
}

func (m *Manager) clearResumeReplay(sessionID string) {
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return
	}
	m.resumeReplayMu.Lock()
	defer m.resumeReplayMu.Unlock()
	delete(m.resumeReplays, target)
}

func (s *Session) setPendingResumeReplay(block string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingResumeReplay = strings.TrimSpace(block)
}
