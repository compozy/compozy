package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestDaemonCheckpointSummarizer(t *testing.T) {
	t.Parallel()

	t.Run("Should skip the hidden session when the role is disabled", func(t *testing.T) {
		t.Parallel()

		manager := &checkpointSummarySessionManagerStub{}
		summarizer := newDaemonCheckpointSummarizer(
			manager,
			resolvedRoleResolver(ResolvedRole{Enabled: false}),
		)
		summary, err := summarizer.Summarize(testutil.Context(t), checkpointSummaryRequestFixture())
		if !errors.Is(err, memory.ErrCheckpointSummaryDisabled) {
			t.Fatalf("Summarize() error = %v, want ErrCheckpointSummaryDisabled", err)
		}
		if summary != "" || manager.createCalls != 0 {
			t.Fatalf("Summarize() = %q with %d create calls, want skipped", summary, manager.createCalls)
		}
	})

	t.Run("Should collect agent output and stop the internal dream session", func(t *testing.T) {
		t.Parallel()

		manager := &checkpointSummarySessionManagerStub{events: []acp.AgentEvent{
			{Type: acp.EventTypeAgentMessage, Text: "## Historical Task Snapshot\n- cobalt\n"},
			{Type: acp.EventTypeAgentMessage, Text: "\n## Goal\n- preserve context"},
		}}
		summarizer := newDaemonCheckpointSummarizer(
			manager,
			resolvedRoleResolver(ResolvedRole{
				Enabled:   true,
				AgentName: aghconfig.BuiltinDreamingCuratorAgentName,
				Builtin:   true,
			}),
		)
		request := checkpointSummaryRequestFixture()
		got, err := summarizer.Summarize(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("Summarize() error = %v", err)
		}
		if !strings.Contains(got, "cobalt") || !strings.Contains(got, "preserve context") {
			t.Fatalf("Summarize() = %q, want collected agent chunks", got)
		}
		if manager.createOpts.Name != checkpointSummarySessionName ||
			manager.createOpts.Type != session.SessionTypeDream ||
			manager.createOpts.Workspace != request.WorkspaceRoot ||
			manager.createOpts.AgentName != aghconfig.BuiltinDreamingCuratorAgentName {
			t.Fatalf("Create() opts = %#v, want checkpoint dream session", manager.createOpts)
		}
		if manager.promptID != "checkpoint-session" ||
			!strings.Contains(manager.prompt, request.SessionID) {
			t.Fatalf(
				"Prompt() = (%q, %q), want checkpoint session and rendered source identity",
				manager.promptID,
				manager.prompt,
			)
		}
		if manager.stopID != "checkpoint-session" || manager.stopCause != session.CauseCompleted {
			t.Fatalf("StopWithCause() = (%q, %v), want completed checkpoint session", manager.stopID, manager.stopCause)
		}
	})

	t.Run("Should stop the internal session when prompting fails", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("prompt rejected")
		manager := &checkpointSummarySessionManagerStub{promptErr: wantErr}
		summarizer := newDaemonCheckpointSummarizer(
			manager,
			resolvedRoleResolver(ResolvedRole{
				Enabled:   true,
				AgentName: aghconfig.BuiltinDreamingCuratorAgentName,
				Builtin:   true,
			}),
		)
		_, err := summarizer.Summarize(testutil.Context(t), checkpointSummaryRequestFixture())
		if !errors.Is(err, wantErr) {
			t.Fatalf("Summarize() error = %v, want wrapped %v", err, wantErr)
		}
		if manager.stopID != "checkpoint-session" || manager.stopCause != session.CauseFailed {
			t.Fatalf(
				"StopWithCause() = (%q, %v), want failed cleanup after prompt failure",
				manager.stopID,
				manager.stopCause,
			)
		}
	})
}

func checkpointSummaryRequestFixture() memory.CheckpointSummaryRequest {
	return memory.CheckpointSummaryRequest{
		WorkspaceID:   "ws-alpha",
		WorkspaceRoot: "/workspace/alpha",
		SessionID:     "sess-alpha",
		AgentName:     "coder",
		EndedAt:       time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
		Snapshot: memcontract.TranscriptSnapshot{Messages: []memcontract.TranscriptMessage{{
			Sequence: 1,
			Role:     "user",
			Content:  "remember cobalt",
		}}},
	}
}

type checkpointSummarySessionManagerStub struct {
	createCalls int
	createOpts  session.CreateOpts
	promptID    string
	prompt      string
	stopID      string
	stopCause   session.StopCause
	events      []acp.AgentEvent
	promptErr   error
}

func (m *checkpointSummarySessionManagerStub) Create(
	_ context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	m.createCalls++
	m.createOpts = opts
	return &session.Session{ID: "checkpoint-session"}, nil
}

func (m *checkpointSummarySessionManagerStub) Prompt(
	_ context.Context,
	id string,
	message string,
) (<-chan acp.AgentEvent, error) {
	m.promptID = id
	m.prompt = message
	if m.promptErr != nil {
		return nil, m.promptErr
	}
	events := make(chan acp.AgentEvent, len(m.events))
	for _, event := range m.events {
		events <- event
	}
	close(events)
	return events, nil
}

func (m *checkpointSummarySessionManagerStub) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	_ string,
) error {
	m.stopID = id
	m.stopCause = cause
	return nil
}
