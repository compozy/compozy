package ui

import (
	"reflect"
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	apicore "github.com/compozy/compozy/internal/api/core"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestSpeedLifecycleEventsProjectCanonicalFields(t *testing.T) {
	t.Parallel()

	queued, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobQueued,
		kinds.JobQueuedPayload{Index: 2, Speed: kinds.SpeedFast},
	))
	if !ok {
		t.Fatal("job.queued did not translate")
	}
	if got := queued.(jobQueuedMsg).Speed; got != kinds.SpeedFast {
		t.Fatalf("queued speed = %q, want %q", got, kinds.SpeedFast)
	}

	started, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 2, Attempt: 3, MaxAttempts: 4},
			Speed:          kinds.SpeedNormal,
		},
	))
	if !ok {
		t.Fatal("job.started did not translate")
	}
	if got := started.(jobStartedMsg).Speed; got != kinds.SpeedNormal {
		t.Fatalf("started speed = %q, want %q", got, kinds.SpeedNormal)
	}

	applied := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusApplied,
	}
	session, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindSessionStarted,
		kinds.SessionStartedPayload{Index: 2, ACPSessionID: "session-2", SpeedResolution: &applied},
	))
	if !ok {
		t.Fatal("session.started did not translate")
	}
	assertSpeedMsg(t, session, 2, 0, "", &applied)

	rejected := kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusRejected,
		Reason:    kinds.SpeedResolutionReasonProviderRejected,
	}
	failedMsgs := newUIEventTranslator().translateMessages(mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobFailed,
		kinds.JobFailedPayload{
			JobAttemptInfo:  kinds.JobAttemptInfo{Index: 2, Attempt: 3, MaxAttempts: 4},
			Error:           "speed rejected",
			SpeedResolution: &rejected,
		},
	))
	speedMsg, found := findSpeedMsg(failedMsgs)
	if !found {
		t.Fatalf("job.failed messages = %#v, want canonical speed resolution", failedMsgs)
	}
	assertSpeedMsg(t, speedMsg, 2, 3, "", &rejected)
}

func TestMissingHistoricalSpeedFieldsDoNotTranslateOrInventState(t *testing.T) {
	t.Parallel()

	queued, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindJobQueued,
		kinds.JobQueuedPayload{Index: 0, SafeName: "historical"},
	))
	if !ok {
		t.Fatal("historical job.queued did not translate")
	}
	if got := queued.(jobQueuedMsg).Speed; got != "" {
		t.Fatalf("historical queued speed = %q, want empty", got)
	}

	if msg, ok := translateEventForTest(t, mustRuntimeEventUITest(
		t,
		eventspkg.EventKindSessionStarted,
		kinds.SessionStartedPayload{Index: 0, ACPSessionID: "historical-session"},
	)); ok {
		t.Fatalf("historical session.started translated invented state %T: %#v", msg, msg)
	}
}

func TestSidebarSpeedLabelsAreTextVisibleAndTruthful(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		resolution *kinds.SpeedResolution
		wantStatus string
	}{
		{name: speedStatusPending, wantStatus: speedStatusPending},
		{
			name: "applied",
			resolution: &kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusApplied,
			},
			wantStatus: "applied",
		},
		{
			name: "unsupported",
			resolution: &kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusUnsupported,
				Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
			},
			wantStatus: "unsupported",
		},
		{
			name: "rejected",
			resolution: &kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusRejected,
				Reason:    kinds.SpeedResolutionReasonProviderRejected,
			},
			wantStatus: "rejected",
		},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mdl := newUIModel(1)
			mdl.handleJobQueued(&jobQueuedMsg{Index: 0, SafeName: "task_01", Speed: kinds.SpeedFast})
			mdl.handleJobStarted(jobStartedMsg{Index: 0, Attempt: 1, MaxAttempts: 1, Speed: kinds.SpeedFast})
			if testCase.resolution != nil {
				mdl.applyUIMsg(jobSpeedMsg{Index: 0, Attempt: 1, Resolution: testCase.resolution})
			}

			label := xansi.Strip(mdl.renderSidebarItem(0, &mdl.jobs[0], true))
			want := "speed fast · " + testCase.wantStatus
			if !strings.Contains(label, want) {
				t.Fatalf("sidebar label = %q, want text %q", label, want)
			}
			if testCase.wantStatus == "pending" && strings.Contains(label, "applied") {
				t.Fatalf("pending sidebar label invented applied status: %q", label)
			}
		})
	}

	t.Run("historical job omits speed outcome", func(t *testing.T) {
		t.Parallel()

		mdl := newUIModel(1)
		mdl.handleJobQueued(&jobQueuedMsg{Index: 0, SafeName: "historical"})
		if label := sidebarSpeedLabel(&mdl.jobs[0]); label != "" {
			t.Fatalf("historical speed label = %q, want empty", label)
		}
	})

	t.Run("resolution requested value remains canonical when queued value is absent", func(t *testing.T) {
		t.Parallel()

		job := &uiJob{speedResolution: &kinds.SpeedResolution{
			Requested: kinds.SpeedNormal,
			Status:    kinds.SpeedResolutionStatusApplied,
		}}
		if label := sidebarSpeedLabel(job); label != "speed normal · applied" {
			t.Fatalf("resolution-backed speed label = %q", label)
		}
		if key := newUIModel(1).sidebarRowKey(0, job, false); key.requestedSpeed != kinds.SpeedNormal {
			t.Fatalf("resolution-backed cache speed = %q, want normal", key.requestedSpeed)
		}
	})
}

func TestLaterAttemptReplacesSpeedResolutionAndRefreshesSidebar(t *testing.T) {
	t.Parallel()

	mdl := newUIModel(1)
	mdl.handleJobQueued(&jobQueuedMsg{Index: 0, SafeName: "task_01", Speed: kinds.SpeedFast})
	mdl.handleJobStarted(jobStartedMsg{Index: 0, Attempt: 1, MaxAttempts: 2, Speed: kinds.SpeedFast})
	rejected := &kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusRejected,
		Reason:    kinds.SpeedResolutionReasonProviderRejected,
	}
	mdl.applyUIMsg(jobSpeedMsg{Index: 0, Attempt: 1, Resolution: rejected})
	if got := sidebarSpeedLabel(&mdl.jobs[0]); got != "speed fast · rejected" {
		t.Fatalf("attempt 1 label = %q", got)
	}
	firstRow := mdl.jobs[0].sidebarCacheRow

	mdl.handleJobStarted(jobStartedMsg{Index: 0, Attempt: 2, MaxAttempts: 2, Speed: kinds.SpeedFast})
	if mdl.jobs[0].speedResolution != nil {
		t.Fatalf("attempt 2 retained stale resolution %#v", mdl.jobs[0].speedResolution)
	}
	if got := sidebarSpeedLabel(&mdl.jobs[0]); got != "speed fast · pending" {
		t.Fatalf("attempt 2 unresolved label = %q", got)
	}
	if firstRow == mdl.jobs[0].sidebarCacheRow {
		t.Fatal("later attempt did not refresh the cached sidebar row")
	}

	unsupported := &kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusUnsupported,
		Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
	}
	mdl.applyUIMsg(jobSpeedMsg{Index: 0, Attempt: 1, Resolution: unsupported})
	if mdl.jobs[0].speedResolution != nil {
		t.Fatalf("stale attempt replaced current state: %#v", mdl.jobs[0].speedResolution)
	}

	mdl.applyUIMsg(jobSpeedMsg{Index: 0, Attempt: 2, Resolution: unsupported})
	if got := sidebarSpeedLabel(&mdl.jobs[0]); got != "speed fast · unsupported" {
		t.Fatalf("attempt 2 resolved label = %q", got)
	}
}

func TestSidebarSpeedLabelTruncatesWithinNarrowCard(t *testing.T) {
	t.Parallel()

	mdl := newUIModel(1)
	mdl.sidebarViewport.SetWidth(18)
	mdl.handleJobQueued(&jobQueuedMsg{Index: 0, SafeName: "task_01", Speed: kinds.SpeedNormal})
	mdl.applyUIMsg(jobSpeedMsg{
		Index:   0,
		Attempt: 1,
		Resolution: &kinds.SpeedResolution{
			Requested: kinds.SpeedNormal,
			Status:    kinds.SpeedResolutionStatusUnsupported,
			Reason:    kinds.SpeedResolutionReasonCapabilityAmbiguous,
		},
	})

	rendered := mdl.renderSidebarItem(0, &mdl.jobs[0], false)
	lines := strings.Split(rendered, "\n")
	if len(lines) != sidebarRowLines {
		t.Fatalf("narrow card lines = %d, want %d: %q", len(lines), sidebarRowLines, xansi.Strip(rendered))
	}
	for index, line := range lines {
		if width := xansi.StringWidth(line); width != 18 {
			t.Fatalf("narrow card line %d width = %d, want 18: %q", index, width, xansi.Strip(line))
		}
	}
}

func TestRemoteSnapshotAndLiveSpeedProjectionMatchesLocalAndChildCards(t *testing.T) {
	t.Parallel()

	unsupported := &kinds.SpeedResolution{
		Requested: kinds.SpeedNormal,
		Status:    kinds.SpeedResolutionStatusUnsupported,
		Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
	}
	snapshot := apicore.RunSnapshot{
		Run: apicore.Run{RunID: "child-speed", WorkflowSlug: "child-speed", Status: remoteRunStatusRunning},
		Jobs: []apicore.RunJobState{{
			Index:  0,
			JobID:  "task_01",
			Status: remoteRunStatusRunning,
			Summary: &apicore.RunJobSummary{
				SafeName:        "task_01",
				TaskTitle:       "Remote speed",
				Attempt:         1,
				MaxAttempts:     2,
				Speed:           kinds.SpeedNormal,
				SpeedResolution: unsupported,
			},
		}},
	}

	remote := modelFromRemoteSpeedSnapshot(t, snapshot)
	local := newUIModel(1)
	replaySpeedEvents(t, local,
		mustRuntimeEventUITest(t, eventspkg.EventKindJobQueued, kinds.JobQueuedPayload{
			Index: 0, SafeName: "task_01", TaskTitle: "Remote speed", Speed: kinds.SpeedNormal,
		}),
		mustRuntimeEventUITest(t, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 2},
			Speed:          kinds.SpeedNormal,
		}),
		mustRuntimeEventUITest(t, eventspkg.EventKindSessionStarted, kinds.SessionStartedPayload{
			Index: 0, ACPSessionID: "session-1", SpeedResolution: unsupported,
		}),
	)

	if got, want := sidebarSpeedLabel(&remote.jobs[0]), sidebarSpeedLabel(&local.jobs[0]); got != want {
		t.Fatalf("remote label = %q, local replay label = %q", got, want)
	}
	if !reflect.DeepEqual(remote.jobs[0].speedResolution, local.jobs[0].speedResolution) {
		t.Fatalf("remote resolution = %#v, local = %#v", remote.jobs[0].speedResolution, local.jobs[0].speedResolution)
	}

	applied := &kinds.SpeedResolution{
		Requested: kinds.SpeedNormal,
		Status:    kinds.SpeedResolutionStatusApplied,
	}
	replaySpeedEvents(t, remote,
		mustRuntimeEventUITest(t, eventspkg.EventKindJobStarted, kinds.JobStartedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 2, MaxAttempts: 2},
			Speed:          kinds.SpeedNormal,
		}),
	)
	if got := sidebarSpeedLabel(&remote.jobs[0]); got != "speed normal · pending" {
		t.Fatalf("later live attempt did not clear snapshot resolution: %q", got)
	}
	replaySpeedEvents(t, remote,
		mustRuntimeEventUITest(t, eventspkg.EventKindSessionStarted, kinds.SessionStartedPayload{
			Index: 0, ACPSessionID: "session-2", SpeedResolution: applied,
		}),
	)
	if got := sidebarSpeedLabel(&remote.jobs[0]); got != "speed normal · applied" {
		t.Fatalf("later live resolution did not replace snapshot: %q", got)
	}

	child := childModelFromRunSnapshot(snapshot, &config{}, 120, 30)
	if got, want := sidebarSpeedLabel(&child.jobs[0]), "speed normal · unsupported"; got != want {
		t.Fatalf("child card label = %q, want %q", got, want)
	}
}

func TestHistoricalRemoteSnapshotDoesNotInventSpeed(t *testing.T) {
	t.Parallel()

	mdl := modelFromRemoteSpeedSnapshot(t, apicore.RunSnapshot{
		Run: apicore.Run{RunID: "historical", Status: remoteRunStatusCompleted},
		Jobs: []apicore.RunJobState{{
			Index:   0,
			JobID:   "task_01",
			Status:  remoteRunStatusCompleted,
			Summary: &apicore.RunJobSummary{SafeName: "task_01", Attempt: 1, MaxAttempts: 1},
		}},
	})
	if got := sidebarSpeedLabel(&mdl.jobs[0]); got != "" {
		t.Fatalf("historical remote label = %q, want empty", got)
	}
}

func TestParallelChildSpeedMessageReindexesCanonicalState(t *testing.T) {
	t.Parallel()

	resolution := &kinds.SpeedResolution{
		Requested: kinds.SpeedFast,
		Status:    kinds.SpeedResolutionStatusApplied,
	}
	mapped, ok := reindexParallelChildUIMsg(jobSpeedMsg{
		Index:      0,
		Attempt:    2,
		Requested:  kinds.SpeedFast,
		Resolution: resolution,
	}, 4)
	if !ok {
		t.Fatal("parallel child speed message was dropped")
	}
	assertSpeedMsg(t, mapped, 4, 2, kinds.SpeedFast, resolution)
}

func assertSpeedMsg(
	t *testing.T,
	msg uiMsg,
	index int,
	attempt int,
	requested kinds.Speed,
	resolution *kinds.SpeedResolution,
) {
	t.Helper()
	speed, ok := msg.(jobSpeedMsg)
	if !ok {
		t.Fatalf("speed message type = %T, want jobSpeedMsg", msg)
	}
	if speed.Index != index || speed.Attempt != attempt || speed.Requested != requested ||
		!reflect.DeepEqual(speed.Resolution, resolution) {
		t.Fatalf("speed message = %#v, want index=%d attempt=%d requested=%q resolution=%#v",
			speed, index, attempt, requested, resolution)
	}
}

func findSpeedMsg(msgs []uiMsg) (uiMsg, bool) {
	for _, msg := range msgs {
		if _, ok := msg.(jobSpeedMsg); ok {
			return msg, true
		}
	}
	return nil, false
}

func modelFromRemoteSpeedSnapshot(t *testing.T, snapshot apicore.RunSnapshot) *uiModel {
	t.Helper()
	jobs, msgs := remoteSnapshotBootstrap(snapshot)
	mdl := newUIModel(len(jobs))
	applyRemoteQueuedJobs(mdl, jobs)
	applyUIMsgs(mdl, msgs...)
	return mdl
}

func replaySpeedEvents(t *testing.T, mdl *uiModel, lifecycle ...eventspkg.Event) {
	t.Helper()
	translator := newUIEventTranslator()
	for _, event := range lifecycle {
		for _, msg := range translator.translateMessages(event) {
			mdl.applyUIMsg(msg)
		}
	}
}
