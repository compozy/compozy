package ui

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestRemoteSnapshotBootstrapHydratesUIStateBeforeLiveEvents(t *testing.T) {
	t.Parallel()

	summarySnapshot := buildSnapshotWithEntries(t, TranscriptEntry{
		ID:     "assistant-1",
		Kind:   transcriptEntryAssistantMessage,
		Title:  "Assistant",
		Blocks: []model.ContentBlock{mustContentBlockUITest(t, model.TextBlock{Text: "hello from snapshot"})},
	})
	snapshot := apicore.RunSnapshot{
		Run: apicore.Run{RunID: "run-remote-ui-001", Status: "completed"},
		Jobs: []apicore.RunJobState{{
			Index:  0,
			JobID:  "task_01",
			Status: "completed",
			Summary: &apicore.RunJobSummary{
				CodeFile:        "task_01.md",
				CodeFiles:       []string{"task_01.md"},
				TaskTitle:       "Remote Attach",
				TaskType:        "frontend",
				SafeName:        "task_01",
				IDE:             "codex",
				Model:           "gpt-5.4",
				ReasoningEffort: "high",
				Attempt:         1,
				MaxAttempts:     1,
				ExitCode:        0,
				Usage:           kinds.Usage{InputTokens: 7, OutputTokens: 3, TotalTokens: 10},
				Session:         summarySnapshot,
			},
		}},
	}

	jobs, msgs := remoteSnapshotBootstrap(snapshot)
	if len(jobs) != 1 {
		t.Fatalf("expected one job from snapshot, got %d", len(jobs))
	}

	mdl := newUIModel(len(jobs))
	applyRemoteQueuedJobs(mdl, jobs)
	applyUIMsgs(mdl, msgs...)

	if got := mdl.jobs[0].taskTitle; got != "Remote Attach" {
		t.Fatalf("expected restored task title, got %q", got)
	}
	if got := mdl.jobs[0].state; got != jobSuccess {
		t.Fatalf("expected restored completed job state, got %v", got)
	}
	if got := mdl.aggregateUsage.TotalTokens; got != 10 {
		t.Fatalf("expected restored aggregate usage total 10, got %d", got)
	}
	if len(mdl.jobs[0].snapshot.Entries) != 1 {
		t.Fatalf("expected restored transcript snapshot entry, got %#v", mdl.jobs[0].snapshot.Entries)
	}
}

func TestFollowRemoteRunReconnectsFromOverflowCursor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 23, 10, 0, 0, time.UTC)
	firstStream := newBufferedClientRunStream(
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobQueued,
			kinds.JobQueuedPayload{
				Index:     0,
				CodeFile:  "task_01.md",
				CodeFiles: []string{"task_01.md"},
				TaskTitle: "Reconnect",
			},
		), 1, now)},
		apiclient.RunStreamItem{Overflow: &apiclient.RunStreamOverflow{
			Cursor: apicore.StreamCursor{Timestamp: now, Sequence: 1},
			Reason: "subscriber_dropped_messages",
		}},
	)
	secondStream := newBufferedClientRunStream(
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindJobStarted,
			kinds.JobStartedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1}},
		), 2, now.Add(time.Second))},
		apiclient.RunStreamItem{Event: eventPointer(mustRuntimeEventUITest(
			t,
			eventspkg.EventKindRunCompleted,
			kinds.RunCompletedPayload{JobsTotal: 1, JobsSucceeded: 1},
		), 3, now.Add(2*time.Second))},
	)

	session := &recordingUISession{}
	var (
		mu           sync.Mutex
		afterCursors []apicore.StreamCursor
		openCalls    int
	)
	opts := RemoteAttachOptions{
		LoadSnapshot: func(_ context.Context) (apicore.RunSnapshot, error) {
			return apicore.RunSnapshot{Run: apicore.Run{RunID: "run-remote-ui-002", Status: "running"}}, nil
		},
		OpenStream: func(_ context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			mu.Lock()
			defer mu.Unlock()
			afterCursors = append(afterCursors, after)
			openCalls++
			if openCalls == 1 {
				return secondStream, nil
			}
			return nil, context.Canceled
		},
	}

	followRemoteRun(context.Background(), session, opts, firstStream, apicore.StreamCursor{})

	if got := session.messageTypes(); !reflect.DeepEqual(got, []string{
		"event:job.queued",
		"event:job.started",
		"event:run.completed",
	}) {
		t.Fatalf("unexpected streamed remote messages: %v", got)
	}
	if len(afterCursors) != 1 || afterCursors[0].Sequence != 1 {
		t.Fatalf("expected reconnect from cursor sequence 1, got %#v", afterCursors)
	}
}

func TestAttachRemoteSkipsLiveStreamForCompletedSnapshot(t *testing.T) {
	t.Parallel()

	snapshot := apicore.RunSnapshot{
		Run: apicore.Run{RunID: "run-remote-ui-003", Status: "completed"},
		Jobs: []apicore.RunJobState{{
			Index:  0,
			JobID:  "task_01",
			Status: "completed",
			Summary: &apicore.RunJobSummary{
				CodeFile:    "task_01.md",
				CodeFiles:   []string{"task_01.md"},
				TaskTitle:   "Completed Snapshot",
				TaskType:    "frontend",
				SafeName:    "task_01",
				Attempt:     1,
				MaxAttempts: 1,
				ExitCode:    0,
			},
		}},
	}

	openCalls := 0
	originalSetup := setupRemoteUISession
	setupRemoteUISession = func(context.Context, []job, *config, *eventspkg.Bus[eventspkg.Event], bool) Session {
		return &recordingUISession{}
	}
	defer func() {
		setupRemoteUISession = originalSetup
	}()
	session, err := AttachRemote(context.Background(), RemoteAttachOptions{
		Snapshot: snapshot,
		OpenStream: func(context.Context, apicore.StreamCursor) (apiclient.RunStream, error) {
			openCalls++
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("AttachRemote: %v", err)
	}
	if openCalls != 0 {
		t.Fatalf("expected terminal snapshot attach to avoid opening a live stream, got %d calls", openCalls)
	}
	if session == nil {
		t.Fatal("expected AttachRemote to return a session")
	}
}

func TestAttachRemoteKeepsOwnerSessionsCancelableFromLocalQuit(t *testing.T) {
	originalSetup := setupRemoteUISession
	defer func() {
		setupRemoteUISession = originalSetup
	}()

	detachOnly := true
	setupRemoteUISession = func(
		_ context.Context,
		_ []job,
		cfg *config,
		_ *eventspkg.Bus[eventspkg.Event],
		enabled bool,
	) Session {
		if !enabled {
			t.Fatal("expected remote attach to enable the ui session")
		}
		if cfg == nil {
			t.Fatal("expected remote attach config")
		}
		detachOnly = cfg.DetachOnly
		return &recordingUISession{}
	}

	session, err := AttachRemote(context.Background(), RemoteAttachOptions{
		Snapshot: apicore.RunSnapshot{
			Run: apicore.Run{RunID: "run-remote-owner-001", Status: "running"},
			Jobs: []apicore.RunJobState{{
				Index:  0,
				Status: "running",
			}},
		},
		OwnerSession: true,
	})
	if err != nil {
		t.Fatalf("AttachRemote(owner): %v", err)
	}
	if session == nil {
		t.Fatal("expected AttachRemote(owner) to return a session")
	}
	if detachOnly {
		t.Fatal("expected owner remote attach to preserve local quit handling")
	}
}

func TestAttachRemoteOpensStreamFromSnapshotCursorForRunningRun(t *testing.T) {
	t.Parallel()

	originalSetup := setupRemoteUISession
	setupRemoteUISession = func(context.Context, []job, *config, *eventspkg.Bus[eventspkg.Event], bool) Session {
		return &recordingUISession{}
	}
	defer func() {
		setupRemoteUISession = originalSetup
	}()

	cursor := apicore.StreamCursor{
		Timestamp: time.Date(2026, 4, 17, 23, 14, 0, 0, time.UTC),
		Sequence:  7,
	}
	openCalls := 0
	afterCursor := apicore.StreamCursor{}
	session, err := AttachRemote(context.Background(), RemoteAttachOptions{
		Snapshot: apicore.RunSnapshot{
			Run:        apicore.Run{RunID: "run-remote-ui-004", Status: "running"},
			NextCursor: &cursor,
		},
		LoadSnapshot: func(_ context.Context) (apicore.RunSnapshot, error) {
			return apicore.RunSnapshot{
				Run:        apicore.Run{RunID: "run-remote-ui-004", Status: "completed"},
				NextCursor: &cursor,
			}, nil
		},
		OpenStream: func(_ context.Context, after apicore.StreamCursor) (apiclient.RunStream, error) {
			openCalls++
			afterCursor = after
			return newBufferedClientRunStream(), nil
		},
	})
	if err != nil {
		t.Fatalf("AttachRemote: %v", err)
	}
	if session == nil {
		t.Fatal("expected AttachRemote to return a session")
	}
	if openCalls != 1 {
		t.Fatalf("expected one remote stream open, got %d", openCalls)
	}
	if afterCursor != cursor {
		t.Fatalf("expected stream open after snapshot cursor %#v, got %#v", cursor, afterCursor)
	}
}

func TestShouldStopAfterRemoteEOFUsesTerminalSnapshotCursor(t *testing.T) {
	t.Parallel()

	lastCursor := apicore.StreamCursor{
		Timestamp: time.Date(2026, 4, 17, 23, 12, 0, 0, time.UTC),
		Sequence:  4,
	}
	stop := shouldStopAfterRemoteEOF(context.Background(), RemoteAttachOptions{
		LoadSnapshot: func(context.Context) (apicore.RunSnapshot, error) {
			return apicore.RunSnapshot{
				Run: apicore.Run{RunID: "run-terminal", Status: "completed"},
				NextCursor: &apicore.StreamCursor{
					Timestamp: lastCursor.Timestamp,
					Sequence:  lastCursor.Sequence,
				},
			}, nil
		},
	}, lastCursor)
	if !stop {
		t.Fatal("expected terminal snapshot with no newer cursor to stop reconnecting")
	}
}

func TestRemoteSnapshotBootstrapIncludesShutdownState(t *testing.T) {
	t.Parallel()

	_, msgs := remoteSnapshotBootstrap(apicore.RunSnapshot{
		Run: apicore.Run{RunID: "run-shutdown", Status: "running"},
		Shutdown: &apicore.RunShutdownState{
			Phase:       string(shutdownPhaseDraining),
			Source:      string(shutdownSourceSignal),
			RequestedAt: time.Unix(10, 0).UTC(),
			DeadlineAt:  time.Unix(20, 0).UTC(),
		},
	})
	if len(msgs) != 1 {
		t.Fatalf("expected one shutdown bootstrap message, got %#v", msgs)
	}
	status, ok := msgs[0].(shutdownStatusMsg)
	if !ok {
		t.Fatalf("expected shutdownStatusMsg, got %T", msgs[0])
	}
	if status.State.Phase != shutdownPhaseDraining || status.State.Source != shutdownSourceSignal {
		t.Fatalf("unexpected shutdown bootstrap state: %#v", status.State)
	}
}

func applyRemoteQueuedJobs(mdl *uiModel, jobs []job) {
	for index := range jobs {
		jb := jobs[index]
		totalIssues := 0
		for _, items := range jb.Groups {
			totalIssues += len(items)
		}
		applyUIMsgs(mdl, jobQueuedMsg{
			Index:           index,
			CodeFile:        jb.CodeFileLabel(),
			CodeFiles:       append([]string(nil), jb.CodeFiles...),
			Issues:          totalIssues,
			TaskTitle:       jb.TaskTitle,
			TaskType:        jb.TaskType,
			SafeName:        jb.SafeName,
			IDE:             jb.IDE,
			Model:           jb.Model,
			ReasoningEffort: jb.ReasoningEffort,
			OutLog:          jb.OutLog,
			ErrLog:          jb.ErrLog,
			OutBuffer:       runshared.NewLineBuffer(0),
			ErrBuffer:       runshared.NewLineBuffer(0),
		})
	}
}

type recordingUISession struct {
	mu   sync.Mutex
	msgs []any
}

func (s *recordingUISession) Enqueue(msg any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.msgs = append(s.msgs, msg)
}

func (s *recordingUISession) SetQuitHandler(func(uiQuitRequest)) {}
func (s *recordingUISession) CloseEvents()                       {}
func (s *recordingUISession) Shutdown()                          {}
func (s *recordingUISession) Wait() error                        { return nil }

func (s *recordingUISession) messageTypes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.msgs))
	for _, msg := range s.msgs {
		switch msg := msg.(type) {
		case jobQueuedMsg:
			result = append(result, "jobQueuedMsg")
		case jobStartedMsg:
			result = append(result, "jobStartedMsg")
		case jobUpdateMsg:
			result = append(result, "jobUpdateMsg")
		case jobFinishedMsg:
			result = append(result, "jobFinishedMsg")
		case eventspkg.Event:
			result = append(result, "event:"+string(msg.Kind))
		default:
			result = append(result, "unknown")
		}
	}
	return result
}

type bufferedClientRunStream struct {
	items chan apiclient.RunStreamItem
	errs  chan error
}

func newBufferedClientRunStream(items ...apiclient.RunStreamItem) *bufferedClientRunStream {
	stream := &bufferedClientRunStream{
		items: make(chan apiclient.RunStreamItem, len(items)),
		errs:  make(chan error),
	}
	for _, item := range items {
		stream.items <- item
	}
	close(stream.items)
	close(stream.errs)
	return stream
}

func (s *bufferedClientRunStream) Items() <-chan apiclient.RunStreamItem {
	if s == nil {
		return nil
	}
	return s.items
}

func (s *bufferedClientRunStream) Errors() <-chan error {
	if s == nil {
		return nil
	}
	return s.errs
}

func (s *bufferedClientRunStream) Close() error {
	return nil
}

func eventPointer(event eventspkg.Event, seq uint64, ts time.Time) *eventspkg.Event {
	event.Seq = seq
	event.Timestamp = ts.UTC()
	return &event
}
