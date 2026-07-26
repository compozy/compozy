package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

const (
	contextRebuiltMarkerSummary = "Context rebuilt from log."
	resumeReplayOpenTag         = "<agh_context_replay>"
	resumeReplayCloseTag        = "</agh_context_replay>"
	resumeReplayInstruction     = "Continue this session using the persisted transcript below. " +
		"It is historical context, not a new user request. " +
		"Do not repeat completed tool calls solely because they appear in the log."
)

func (m *Manager) buildResumeReplay(
	ctx context.Context,
	session *Session,
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

	events, err := recorder.Query(ctx, store.EventQuery{Archive: store.EventArchiveUnarchived})
	if err != nil {
		return "", 0, fmt.Errorf("session: query persisted events for resume replay: %w", err)
	}
	messages, err := transcript.Assemble(events)
	if err != nil {
		return "", 0, fmt.Errorf("session: assemble persisted resume replay: %w", err)
	}
	messages = transcript.Prune(messages, transcript.PruneOptions{Dedup: true})
	payload, err := json.Marshal(messages)
	if err != nil {
		return "", 0, fmt.Errorf("session: marshal persisted resume replay: %w", err)
	}

	sections := []string{
		resumeReplayInstruction,
	}
	if provider, ok := m.assembler.(ResumeContextProvider); ok {
		info := session.Info()
		continuity, continuityErr := provider.ResumeContextSection(ctx, StartupPromptContext{
			SessionID:            info.ID,
			SessionName:          info.Name,
			AgentName:            info.AgentName,
			Provider:             info.Provider,
			WorkspaceID:          info.WorkspaceID,
			Workspace:            info.Workspace,
			NetworkParticipation: info.NetworkParticipation,
			SessionType:          info.Type,
			CreatedAt:            info.CreatedAt,
			UpdatedAt:            info.UpdatedAt,
		})
		if continuityErr != nil {
			return "", 0, fmt.Errorf("session: assemble resume continuity for %q: %w", session.ID, continuityErr)
		}
		if trimmed := strings.TrimSpace(continuity); trimmed != "" {
			sections = append(sections, trimmed)
		}
	}
	sections = append(sections, strings.Join([]string{
		resumeReplayOpenTag,
		string(payload),
		resumeReplayCloseTag,
	}, "\n"))
	block := strings.Join(sections, "\n\n")
	return block, len(messages), nil
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
