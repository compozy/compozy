package demoseed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	pickActorOperator = "operator"
	// initiatorDaemon marks a session the daemon started on an instance's behalf.
	initiatorDaemon = "daemon"
)

// registerMainSession records the main session the daemon starts when an agent is turned on. A
// daemon shutdown leaves it stopped with reason shutdown; the next prompt resumes it in place.
func registerMainSession(
	ctx context.Context,
	db *globaldb.GlobalDB,
	record workspaceRecord,
	story instanceStory,
	instanceID string,
	policy store.SessionPermissionPolicy,
) error {
	info := &store.SessionInfo{
		ID: story.MainID, ProfileID: store.DefaultProfileID, InstanceID: instanceID,
		Name: story.MainName, AgentName: story.Agent, Provider: story.Provider, Model: story.Model,
		WorkspaceID: record.ID, SessionType: string(session.SessionTypeUser),
		State: string(session.StateStopped), RuntimeStatus: store.SessionRuntimeUnbound,
		StopReason: store.StopShutdown, CreatedAt: story.CreatedAt, UpdatedAt: story.CreatedAt,
		InitiatorKind: initiatorDaemon, InitiatorID: instanceID,
		Lineage: &store.SessionLineage{RootSessionID: story.MainID, PermissionPolicy: policy},
	}
	if _, err := db.RegisterInstanceSession(ctx, info, instanceID, nil); err != nil {
		return fmt.Errorf("demo seed: register main session for %q: %w", story.Agent, err)
	}
	return nil
}

func writeMainSessionMeta(dir string, record workspaceRecord, run *instanceRun, updatedAt time.Time) error {
	story := run.story
	meta := store.SessionMeta{
		ID: story.MainID, Name: story.MainName, AgentName: story.Agent, Provider: story.Provider,
		ProfileID: store.DefaultProfileID, Model: story.Model, WorkspaceID: record.ID,
		SessionType: string(session.SessionTypeUser), State: string(session.StateStopped),
		RuntimeStatus: store.SessionRuntimeUnbound, StopReason: new(store.StopShutdown),
		CreatedAt: story.CreatedAt, UpdatedAt: updatedAt,
		InitiatorKind: initiatorDaemon, InitiatorID: run.instance.ID,
		Lineage: &store.SessionLineage{RootSessionID: story.MainID, PermissionPolicy: run.policy},
	}
	meta.SetCWD(record.RootDir)
	meta.SetEffectivePermissions(run.agent.Permissions)
	if err := store.WriteSessionMeta(store.SessionMetaFile(dir), &meta); err != nil {
		return fmt.Errorf("demo seed: write main session metadata for %q: %w", story.Agent, err)
	}
	return nil
}

func reportAt(
	ctx context.Context,
	db *globaldb.GlobalDB,
	instanceID string,
	at time.Time,
) (agentinstance.Report, error) {
	reports, err := db.InstanceReports().ListReports(ctx, agentinstance.ReportQuery{
		InstanceID: instanceID, AfterCreatedAt: at.Add(-time.Millisecond), Limit: 1,
	})
	if err != nil {
		return agentinstance.Report{}, err
	}
	if len(reports) == 0 || !reports[0].CreatedAt.Equal(at) {
		return agentinstance.Report{}, fmt.Errorf("no report written at %s", at)
	}
	return reports[0], nil
}

func actionSnapshots(story instanceStory) []agentinstance.ActionSnapshot {
	out := make([]agentinstance.ActionSnapshot, 0, len(story.Actions))
	for _, action := range story.Actions {
		out = append(out, agentinstance.ActionSnapshot{
			ID: action.ID, Title: action.Title, Instructions: action.Instructions, AutoAccept: action.AutoAccept,
		})
	}
	return out
}

// pickProposal is the operator pressing Pick on a report card; the daemon then dispatches it.
func pickProposal(ctx context.Context, db *globaldb.GlobalDB, run *instanceRun, entry timelineEntry) error {
	report, err := reportAt(ctx, db, run.instance.ID, entry.beat.At)
	if err != nil {
		return err
	}
	pick, err := db.Instances().PickProposal(ctx, agentinstance.PickRequest{
		Actions: actionSnapshots(run.story), InstanceID: run.instance.ID, ReportID: report.ID,
		Actor: pickActorOperator, ProposalIndex: entry.beat.PickIndex, QueueCap: pickQueueCap,
	})
	if err != nil {
		return err
	}
	dispatchedAt := entry.at.Add(pickReplyDelay / 2)
	if err := dispatchPick(ctx, db, run, pick.QueueEntryID, dispatchedAt); err != nil {
		return err
	}
	text, err := pickText(run.story, *entry.beat)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s_pick_%d", run.story.MainID, entry.at.Unix())
	return admitOperatorTurn(ctx, db, run, key, text, dispatchedAt)
}

// dispatchPick claims the queued pick and records it sent, as the session input pump does.
func dispatchPick(ctx context.Context, db *globaldb.GlobalDB, run *instanceRun, entryID string, at time.Time) error {
	claimed, ok, err := db.ClaimNextSessionInput(ctx, run.story.MainID, at)
	if err != nil {
		return err
	}
	if !ok || claimed.ID != entryID {
		return fmt.Errorf("main session %q did not claim pick %q next", run.story.MainID, entryID)
	}
	return db.MarkSessionInputSent(ctx, run.story.MainID, entryID, at)
}

// pickText is the message a pick queues, as agentinstance.PickPromptText renders it.
func pickText(story instanceStory, beat checkInBeat) (string, error) {
	proposal := beat.Proposals[beat.PickIndex]
	payload := agentinstance.ProposalPayload{Title: proposal.Title, Body: proposal.Body, ItemKeys: proposal.ItemKeys}
	for _, action := range actionSnapshots(story) {
		if action.ID == proposal.Action {
			payload.Action = &action
		}
	}
	return agentinstance.PickPromptText(payload, payload.Action)
}

// settleInstance applies where the operator left the agent: paused, and when they last looked.
func settleInstance(ctx context.Context, db *globaldb.GlobalDB, state *scenario, run *instanceRun) error {
	if !run.story.PauseAt.IsZero() {
		state.writeClock.at(run.story.PauseAt)
		if _, err := db.Instances().Pause(ctx, run.instance.ID, pauseReasonOperator); err != nil {
			return fmt.Errorf("demo seed: pause %q: %w", run.story.Agent, err)
		}
	}
	if run.story.SeenAt.IsZero() {
		return nil
	}
	state.writeClock.at(run.story.SeenAt)
	if _, err := db.Instances().MarkSeen(
		ctx, run.instance.ID, agentinstance.LocalOperator, run.story.SeenAt,
	); err != nil {
		return fmt.Errorf("demo seed: mark %q seen: %w", run.story.Agent, err)
	}
	return nil
}

// writeAgentNotes writes the deliberate notes the agent kept, replacing any earlier seed's notes.
func writeAgentNotes(state *scenario, run *instanceRun) error {
	if len(run.story.Notes) == 0 {
		return nil
	}
	memory := agentmemory.NewStore(state.paths.HomeDir)
	key := agentmemory.Key{
		ProfileID: store.DefaultProfileID, Origin: "workspace-" + run.record.ID, Agent: run.story.Agent,
	}
	lines := make([]string, 0, len(run.story.Notes))
	for _, note := range run.story.Notes {
		line, err := agentmemory.FormatEntry(note.At, note.Scope, note.Text, "")
		if err != nil {
			return fmt.Errorf("demo seed: format note for %q: %w", run.story.Agent, err)
		}
		lines = append(lines, line)
	}
	current, err := memory.Read(key)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("demo seed: read notes for %q: %w", run.story.Agent, err)
	}
	if _, err := memory.Write(key, strings.Join(lines, "\n")+"\n", current.Digest); err != nil {
		return fmt.Errorf("demo seed: write notes for %q: %w", run.story.Agent, err)
	}
	return nil
}

// reportEvent is the session.report row the daemon's report publisher appends to the main session.
func reportEvent(sessionID string, agentName string, report agentinstance.Report) (store.SessionEvent, error) {
	content, err := json.Marshal(session.InstanceReportEvent{
		ID: report.ID, InstanceID: report.InstanceID, Kind: report.Kind,
		Payload: report.Payload, CreatedAt: report.CreatedAt,
	})
	if err != nil {
		return store.SessionEvent{}, err
	}
	return store.SessionEvent{
		ID:        "report:" + report.ID,
		SessionID: sessionID,
		TurnID:    "report:" + report.ID,
		Type:      transcript.ReportEventType,
		AgentName: agentName,
		Content:   string(content),
		Timestamp: report.CreatedAt.UTC(),
	}, nil
}

func mainSessionDir(state *scenario, run *instanceRun) string {
	return filepath.Join(state.paths.SessionsDir, run.story.MainID)
}

func alwaysOnSessionIDs() []string {
	return []string{
		sessionIssueTriagerMainID, sessionReleaseWatchMainID, sessionLogTriageMainID, sessionRiskMonitorMainID,
		sessionOrchestratorMainID,
	}
}

// stopWorkspaceSessions stops the sessions a replace is about to delete: a daemon that ran on
// the seeded home can leave sessions active, and workspace deletion refuses live ones.
func stopWorkspaceSessions(ctx context.Context, db *globaldb.GlobalDB, workspaceID string) error {
	sessions, err := db.ListSessions(ctx, store.SessionListQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: workspaceID, Archive: store.SessionArchiveInclude,
	})
	if err != nil {
		return fmt.Errorf("demo seed: list sessions to replace: %w", err)
	}
	reason := string(store.StopShutdown)
	for index := range sessions {
		info := &sessions[index]
		if info.State == string(session.StateStopped) {
			continue
		}
		if err := db.UpdateSessionState(ctx, store.SessionStateUpdate{
			ID: info.ID, State: string(session.StateStopped), StopReasonSet: true, StopReason: &reason,
		}); err != nil {
			return fmt.Errorf("demo seed: stop session %q before replace: %w", info.ID, err)
		}
	}
	return nil
}

const promptFingerprintVersion = "session-prompt/v5"

// admitOperatorTurn records the operator's prompt admission the way the session manager does:
// admitting a human prompt reserves the agent's pending reports, and dispatch delivers them
// into that turn, so earlier reports stop waiting on the operator.
func admitOperatorTurn(
	ctx context.Context,
	db *globaldb.GlobalDB,
	run *instanceRun,
	key string,
	text string,
	at time.Time,
) error {
	id := "padm_" + key
	messageID := "msg_" + key
	if _, _, err := db.ClaimSessionPromptAdmission(ctx, store.SessionPromptAdmissionRequest{
		DirectDispatch: true, HumanIngress: true,
		ID: id, WorkspaceID: run.record.ID, SessionID: run.story.MainID, MessageID: messageID,
		IdempotencyKey: id, Operation: store.SessionPromptOperationPrompt,
		FingerprintVersion: promptFingerprintVersion, RequestFingerprint: "sha256:" + key,
		Mode: store.SessionInputQueueModeQueue, AuthoredText: text,
		TurnID: "turn_" + key, EventID: "evt_" + key, Now: at,
	}); err != nil {
		return fmt.Errorf("admit operator turn %q: %w", key, err)
	}
	if err := db.CommitSessionPromptDispatch(ctx, run.record.ID, run.story.MainID, id, at); err != nil {
		return fmt.Errorf("commit operator turn %q: %w", key, err)
	}
	if _, err := db.DispatchSessionReports(ctx, run.record.ID, run.story.MainID, []string{messageID}, at); err != nil {
		return fmt.Errorf("deliver reports into turn %q: %w", key, err)
	}
	if _, err := db.CompleteSessionPromptAdmission(ctx, run.record.ID, run.story.MainID, id,
		store.SessionPromptAdmissionResult{Status: "sent"}, at); err != nil {
		return fmt.Errorf("complete operator turn %q: %w", key, err)
	}
	return nil
}
