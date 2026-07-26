package globaldb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBAcceptNetworkMessage(t *testing.T) {
	t.Parallel()

	t.Run("Should persist sender and delivered thread recipients as durable participants", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 1, 55, 0, 0, time.UTC)
		message := threadMessage(
			"msg-thread-participants",
			"thread_delivery_evidence",
			"coder.sess-abc",
			"Please review",
			now,
		)
		_, err := globalDB.AcceptNetworkMessage(testutil.Context(t), store.AcceptNetworkMessageRequest{
			Message: message,
			Dispositions: []store.NetworkMessageDisposition{
				{RecipientSessionID: "other.sess-peer", Decision: store.NetworkDispositionIgnore},
				{RecipientSessionID: "reviewer.sess-xyz", Decision: store.NetworkDispositionDeliver},
			},
		})
		if err != nil {
			t.Fatalf("AcceptNetworkMessage() error = %v", err)
		}
		participants, err := globalDB.ListThreadParticipants(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			message.ThreadID,
		)
		if err != nil {
			t.Fatalf("ListThreadParticipants() error = %v", err)
		}
		got := make([]string, 0, len(participants))
		for _, participant := range participants {
			got = append(got, participant.SessionID)
		}
		want := []string{"coder.sess-abc", "reviewer.sess-xyz"}
		if !sameStrings(got, want) {
			t.Fatalf("thread participants = %#v, want delivered evidence %#v", got, want)
		}
		thread, err := globalDB.GetThread(testutil.Context(t), store.NetworkChannelRef{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
		}, message.ThreadID)
		if err != nil {
			t.Fatalf("GetThread() error = %v", err)
		}
		if thread.ParticipantCount != 2 {
			t.Fatalf("thread participant_count = %d, want 2", thread.ParticipantCount)
		}
	})

	t.Run("Should atomically accept a direct wake and return only committed notifications", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 2, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		req := networkWakeAcceptanceRequest(t, "msg-accept-1", "wake-1", "run-wake-1", now)

		result, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage() error = %v", err)
		}
		if result.AcceptanceSeq <= 0 || result.Duplicate || result.AvailabilityEpoch != 1 {
			t.Fatalf("acceptance result = %#v, want new acceptance at availability epoch 1", result)
		}
		if len(result.Admitted) != 1 || result.Admitted[0].TaskRunID != "run-wake-1" ||
			result.Admitted[0].OwnerKey != "session:reviewer.sess-xyz" {
			t.Fatalf("admitted wakes = %#v, want one durable reservation", result.Admitted)
		}
		if len(result.Notify) != 1 || result.Notify[0].TaskRunID != "run-wake-1" ||
			result.Notify[0].AcceptanceSeq != result.AcceptanceSeq {
			t.Fatalf("committed notifications = %#v, want admitted task run", result.Notify)
		}
		run, err := globalDB.GetTaskRun(testutil.Context(t), "run-wake-1")
		if err != nil {
			t.Fatalf("GetTaskRun(network wake) error = %v", err)
		}
		if !run.IsNetworkWake() || run.TaskID != "" || run.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("network wake run = %#v, want queued taskless wake", run)
		}
		wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
		if wakeID != "wake-1" || targetSessionID != "reviewer.sess-xyz" ||
			ownerKey != "session:reviewer.sess-xyz" {
			t.Fatalf("network wake correlation = (%q, %q, %q)", wakeID, targetSessionID, ownerKey)
		}
		assertNetworkAcceptanceRowCounts(t, globalDB, "msg-accept-1", "session:reviewer.sess-xyz", 1, 1)

		duplicate, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(duplicate) error = %v", err)
		}
		if !duplicate.Duplicate || duplicate.AcceptanceSeq != result.AcceptanceSeq ||
			len(duplicate.Notify) != 0 || len(duplicate.Dispositions) != 1 {
			t.Fatalf("duplicate acceptance = %#v, want durable evidence without notification", duplicate)
		}
		assertNetworkAcceptanceRowCounts(t, globalDB, "msg-accept-1", "session:reviewer.sess-xyz", 1, 1)
	})

	t.Run("Should coalesce only while the open wake is queued and inside its window", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		base := time.Date(2026, 7, 14, 2, 10, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return base }
		first := networkWakeAcceptanceRequest(t, "msg-coalesce-1", "wake-coalesced", "run-coalesced", base)
		if _, err := globalDB.AcceptNetworkMessage(testutil.Context(t), first); err != nil {
			t.Fatalf("AcceptNetworkMessage(first) error = %v", err)
		}

		second := networkWakeAcceptanceRequest(
			t,
			"msg-coalesce-2",
			"wake-unused-2",
			"run-unused-2",
			base.Add(100*time.Millisecond),
		)
		second.Admissions[0].RootID = first.Admissions[0].RootID
		coalesced, err := globalDB.AcceptNetworkMessage(testutil.Context(t), second)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(coalesced) error = %v", err)
		}
		if len(coalesced.Admitted) != 0 || len(coalesced.Notify) != 0 || len(coalesced.Skipped) != 1 ||
			coalesced.Skipped[0].Reason != store.NetworkWakeSkipCoalesced ||
			coalesced.Skipped[0].WakeID != "wake-coalesced" {
			t.Fatalf("coalesced acceptance = %#v", coalesced)
		}
		assertNetworkAcceptanceRowCounts(t, globalDB, "msg-coalesce-2", "session:reviewer.sess-xyz", 1, 2)

		third := networkWakeAcceptanceRequest(
			t,
			"msg-coalesce-3",
			"wake-unused-3",
			"run-unused-3",
			base.Add(time.Second),
		)
		third.Admissions[0].RootID = first.Admissions[0].RootID
		accumulated, err := globalDB.AcceptNetworkMessage(testutil.Context(t), third)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(after cutoff) error = %v", err)
		}
		if len(accumulated.Skipped) != 1 || accumulated.Skipped[0].Reason != store.NetworkWakeSkipInFlight {
			t.Fatalf("after-cutoff acceptance = %#v, want in_flight skip", accumulated)
		}
		assertNetworkAcceptanceRowCounts(t, globalDB, "msg-coalesce-3", "session:reviewer.sess-xyz", 1, 2)
	})

	t.Run("Should stop coalescing as soon as the canonical wake run is claimed", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		base := time.Date(2026, 7, 14, 2, 15, 0, 0, time.UTC)
		first := networkWakeAcceptanceRequest(t, "msg-claimed-1", "wake-claimed", "run-claimed", base)
		if _, err := globalDB.AcceptNetworkMessage(testutil.Context(t), first); err != nil {
			t.Fatalf("AcceptNetworkMessage(first) error = %v", err)
		}
		if _, err := globalDB.ClaimNextRun(testutil.Context(t), taskpkg.ClaimCriteria{
			RunID: "run-claimed", RunKind: taskpkg.RunKindNetworkWake,
			Scope: taskpkg.ScopeWorkspace, WorkspaceID: networkStoreTestWorkspaceID,
			TargetSessionID: "reviewer.sess-xyz", ClaimerSessionID: "reviewer.sess-xyz", Now: base,
		}); err != nil {
			t.Fatalf("ClaimNextRun(network wake) error = %v", err)
		}
		second := networkWakeAcceptanceRequest(
			t,
			"msg-claimed-2",
			"wake-unused-claimed",
			"run-unused-claimed",
			base.Add(100*time.Millisecond),
		)
		second.Admissions[0].RootID = first.Admissions[0].RootID
		result, err := globalDB.AcceptNetworkMessage(testutil.Context(t), second)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(after claim) error = %v", err)
		}
		if len(result.Skipped) != 1 || result.Skipped[0].Reason != store.NetworkWakeSkipInFlight {
			t.Fatalf("after-claim acceptance = %#v, want in_flight", result)
		}
		got := networkRowCount(
			t,
			globalDB,
			"network_wake_sources",
			"owner_key",
			first.Admissions[0].OwnerKey,
		)
		if got != 1 {
			t.Fatalf("after-claim wake source count = %d, want 1", got)
		}
	})

	t.Run("Should roll back every acceptance row when a recipient is outside the workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"network-foreign-recipient",
			filepath.Join(t.TempDir(), "foreign-workspace"),
		)
		now := time.Date(2026, 7, 14, 2, 20, 0, 0, time.UTC)
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ID: "sess-foreign-recipient", AgentName: "coder", Provider: "claude",
			WorkspaceID: foreignWorkspaceID, State: "active", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession(foreign) error = %v", err)
		}
		req := networkWakeAcceptanceRequest(t, "msg-foreign-recipient", "wake-foreign", "run-foreign", now)
		req.Dispositions[0].RecipientSessionID = "sess-foreign-recipient"
		req.Admissions[0].RecipientSessionID = "sess-foreign-recipient"

		if _, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req); err == nil {
			t.Fatal("AcceptNetworkMessage(foreign recipient) error = nil, want non-nil")
		}
		assertNoTimelineOrAuditRows(t, globalDB, req.Message.MessageID)
		if got := networkRowCount(t, globalDB, "task_runs", "id", "run-foreign"); got != 0 {
			t.Fatalf("rolled-back wake run count = %d, want 0", got)
		}
	})

	t.Run("Should reject raw claim tokens at the accept boundary before durable writes", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*store.AcceptNetworkMessageRequest, string)
		}{
			{
				name: "nested body metadata",
				mutate: func(req *store.AcceptNetworkMessageRequest, token string) {
					req.Message.Body = json.RawMessage(`{"metadata":{"claim_token":"` + token + `"}}`)
				},
			},
			{
				name: "extension metadata",
				mutate: func(req *store.AcceptNetworkMessageRequest, token string) {
					req.Message.ExtJSON = json.RawMessage(`{"agh.metadata":{"claim_token":"` + token + `"}}`)
				},
			},
		}
		for index, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				t.Parallel()

				globalDB := openNetworkConversationRepositoryTestDB(t)
				token := "agh_claim_ACCEPT_SECRET_" + strconv.Itoa(index)
				req := networkWakeAcceptanceRequest(
					t,
					"msg-unsafe-accept-"+strconv.Itoa(index),
					"wake-unsafe-accept-"+strconv.Itoa(index),
					"run-unsafe-accept-"+strconv.Itoa(index),
					time.Date(2026, 7, 14, 2, 25+index, 0, 0, time.UTC),
				)
				test.mutate(&req, token)
				_, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
				assertRawClaimTokenRejected(t, err, token)
				assertNoTimelineOrAuditRows(t, globalDB, req.Message.MessageID)
				if got := networkRowCount(t, globalDB, "task_runs", "id", req.Admissions[0].TaskRunID); got != 0 {
					t.Fatalf("unsafe accept wake run count = %d, want 0", got)
				}
			})
		}
	})

	t.Run("Should apply structural eligibility, depth, and address gates before admission", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			mutate func(*store.AcceptNetworkMessageRequest)
			reason string
		}{
			{
				name: "control message",
				mutate: func(req *store.AcceptNetworkMessageRequest) {
					req.Admissions[0].Eligible = false
					req.Admissions[0].Addressed = false
					req.Admissions[0].Trigger = store.NetworkWakeTriggerNone
					req.Admissions[0].WakeID = ""
					req.Admissions[0].TaskRunID = ""
					req.Admissions[0].RootID = ""
				},
				reason: store.NetworkWakeSkipNotEligible,
			},
			{
				name: "unaddressed say",
				mutate: func(req *store.AcceptNetworkMessageRequest) {
					req.Admissions[0].Addressed = false
					req.Admissions[0].Trigger = store.NetworkWakeTriggerNone
					req.Admissions[0].WakeID = ""
					req.Admissions[0].TaskRunID = ""
					req.Admissions[0].RootID = ""
				},
				reason: store.NetworkWakeSkipNotAddressed,
			},
			{
				name: "depth cap",
				mutate: func(req *store.AcceptNetworkMessageRequest) {
					req.Admissions[0].Depth = req.Admissions[0].Spec.Bounds.MaxWakeDepth
				},
				reason: store.NetworkWakeSkipDepthExceeded,
			},
		}
		for index, test := range tests {
			t.Run("Should skip "+test.name, func(t *testing.T) {
				t.Parallel()

				globalDB := openNetworkConversationRepositoryTestDB(t)
				now := time.Date(2026, 7, 14, 2, 50+index, 0, 0, time.UTC)
				req := networkWakeAcceptanceRequest(
					t,
					"msg-gate-"+strconv.Itoa(index),
					"wake-gate-"+strconv.Itoa(index),
					"run-gate-"+strconv.Itoa(index),
					now,
				)
				test.mutate(&req)
				result, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
				if err != nil {
					t.Fatalf("AcceptNetworkMessage() error = %v", err)
				}
				if len(result.Skipped) != 1 || result.Skipped[0].Reason != test.reason ||
					len(result.Admitted) != 0 || len(result.Notify) != 0 {
					t.Fatalf("gated acceptance = %#v, want %q skip", result, test.reason)
				}
				got := networkRowCount(
					t,
					globalDB,
					"network_live_wakes",
					"owner_key",
					req.Admissions[0].OwnerKey,
				)
				if got != 0 {
					t.Fatalf("gated wake count = %d, want 0", got)
				}
			})
		}
	})

	t.Run("Should linearize disable and never readmit an accumulated envelope after re-enable", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
		disabled, err := globalDB.SetNetworkAvailability(testutil.Context(t), false, "test:disable")
		if err != nil {
			t.Fatalf("SetNetworkAvailability(false) error = %v", err)
		}
		req := networkWakeAcceptanceRequest(t, "msg-disabled", "wake-disabled", "run-disabled", now)
		result, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(disabled) error = %v", err)
		}
		if result.AvailabilityEpoch != disabled.Epoch || len(result.Skipped) != 1 ||
			result.Skipped[0].Reason != store.NetworkWakeSkipNetworkDisabled {
			t.Fatalf("disabled acceptance = %#v, want epoch %d network_disabled", result, disabled.Epoch)
		}
		if _, err := globalDB.SetNetworkAvailability(testutil.Context(t), true, "test:enable"); err != nil {
			t.Fatalf("SetNetworkAvailability(true) error = %v", err)
		}
		replayed, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(re-enabled replay) error = %v", err)
		}
		if !replayed.Duplicate || len(replayed.Admitted) != 0 || len(replayed.Notify) != 0 {
			t.Fatalf("re-enabled replay = %#v, want duplicate evidence only", replayed)
		}
		if got := networkRowCount(t, globalDB, "task_runs", "id", "run-disabled"); got != 0 {
			t.Fatalf("disabled replay wake run count = %d, want 0", got)
		}
	})

	t.Run("Should linearize concurrent disable and admissions by persisted epoch", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 3, 5, 0, 0, time.UTC)
		const workers = 8
		requests := make([]store.AcceptNetworkMessageRequest, workers)
		connections := make([]*GlobalDB, workers+1)
		for index := range connections {
			connection, err := OpenGlobalDB(testutil.Context(t), globalDB.Path())
			if err != nil {
				t.Fatalf("OpenGlobalDB(concurrent %d) error = %v", index, err)
			}
			connections[index] = connection
			t.Cleanup(func() {
				if closeErr := connection.Close(context.Background()); closeErr != nil {
					t.Errorf("Close(concurrent %d) error = %v", index, closeErr)
				}
			})
		}
		for index := range workers {
			requests[index] = networkWakeAcceptanceRequest(
				t,
				"msg-disable-race-"+strconv.Itoa(index),
				"wake-disable-race-"+strconv.Itoa(index),
				"run-disable-race-"+strconv.Itoa(index),
				now,
			)
			requests[index].Admissions[0].RootID = "root-disable-race-" + strconv.Itoa(index)
		}

		type admissionOutcome struct {
			index  int
			result store.AcceptNetworkMessageResult
			err    error
		}
		start := make(chan struct{})
		admissions := make(chan admissionOutcome, workers)
		disableResult := make(chan struct {
			availability store.NetworkAvailability
			err          error
		}, 1)
		var waitGroup sync.WaitGroup
		waitGroup.Add(workers + 1)
		go func() {
			defer waitGroup.Done()
			<-start
			availability, err := connections[workers].SetNetworkAvailability(
				context.Background(),
				false,
				"test:concurrent-disable",
			)
			disableResult <- struct {
				availability store.NetworkAvailability
				err          error
			}{availability: availability, err: err}
		}()
		for index := range workers {
			go func(index int) {
				defer waitGroup.Done()
				<-start
				result, err := connections[index].AcceptNetworkMessage(
					context.Background(),
					requests[index],
				)
				admissions <- admissionOutcome{index: index, result: result, err: err}
			}(index)
		}
		close(start)
		waitGroup.Wait()
		close(admissions)
		disabled := <-disableResult
		if disabled.err != nil {
			t.Fatalf("SetNetworkAvailability(concurrent disable) error = %v", disabled.err)
		}
		if disabled.availability.Enabled || disabled.availability.Epoch != 2 {
			t.Fatalf("concurrent disabled availability = %#v, want disabled epoch 2", disabled.availability)
		}

		for outcome := range admissions {
			if outcome.err != nil {
				t.Fatalf("AcceptNetworkMessage(concurrent %d) error = %v", outcome.index, outcome.err)
			}
			switch outcome.result.AvailabilityEpoch {
			case 1:
				input := requests[outcome.index].Admissions[0]
				switch {
				case len(outcome.result.Admitted) == 1:
					reservation := outcome.result.Admitted[0]
					if outcome.result.Duplicate || outcome.result.AcceptanceSeq <= 0 ||
						len(outcome.result.Skipped) != 0 || len(outcome.result.Notify) != 1 ||
						reservation.WakeID != input.WakeID || reservation.TaskRunID != input.TaskRunID ||
						reservation.WorkspaceID != input.WorkspaceID || reservation.OwnerKey != input.OwnerKey {
						t.Fatalf(
							"epoch-1 admission %d = %#v, want one exact durable reservation",
							outcome.index,
							outcome.result,
						)
					}
					notification := outcome.result.Notify[0]
					if notification.WorkspaceID != input.WorkspaceID ||
						notification.RecipientSessionID != input.RecipientSessionID ||
						notification.TaskRunID != input.TaskRunID ||
						notification.AcceptanceSeq != outcome.result.AcceptanceSeq {
						t.Fatalf(
							"epoch-1 notification %d = %#v, want exact committed reservation notification",
							outcome.index,
							notification,
						)
					}
				case len(outcome.result.Skipped) == 1:
					skip := outcome.result.Skipped[0]
					if outcome.result.Duplicate || outcome.result.AcceptanceSeq <= 0 ||
						len(outcome.result.Admitted) != 0 || len(outcome.result.Notify) != 0 ||
						skip.Reason != store.NetworkWakeSkipBudgetExhausted ||
						skip.RecipientSessionID != input.RecipientSessionID || skip.OwnerKey != input.OwnerKey ||
						skip.WakeID != "" {
						t.Fatalf(
							"epoch-1 admission %d = %#v, want exact budget-exhausted accumulation",
							outcome.index,
							outcome.result,
						)
					}
				default:
					t.Fatalf(
						"epoch-1 admission %d = %#v, want one reservation or budget-exhausted skip",
						outcome.index,
						outcome.result,
					)
				}
			case disabled.availability.Epoch:
				if len(outcome.result.Admitted) != 0 || len(outcome.result.Skipped) != 1 ||
					outcome.result.Skipped[0].Reason != store.NetworkWakeSkipNetworkDisabled {
					t.Fatalf("epoch-%d admission %d = %#v, want network_disabled accumulation",
						disabled.availability.Epoch,
						outcome.index,
						outcome.result,
					)
				}
			default:
				t.Fatalf("admission %d availability epoch = %d, want 1 or %d",
					outcome.index,
					outcome.result.AvailabilityEpoch,
					disabled.availability.Epoch,
				)
			}
		}

		enabled, err := globalDB.SetNetworkAvailability(testutil.Context(t), true, "test:concurrent-enable")
		if err != nil {
			t.Fatalf("SetNetworkAvailability(re-enable) error = %v", err)
		}
		if !enabled.Enabled || enabled.Epoch != 3 {
			t.Fatalf("re-enabled availability = %#v, want enabled epoch 3", enabled)
		}
		wakeCount := networkRowCount(
			t,
			globalDB,
			"network_live_wakes",
			"owner_key",
			"session:reviewer.sess-xyz",
		)
		for index, req := range requests {
			replayed, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
			if err != nil {
				t.Fatalf("AcceptNetworkMessage(replay %d) error = %v", index, err)
			}
			if !replayed.Duplicate || len(replayed.Admitted) != 0 || len(replayed.Notify) != 0 {
				t.Fatalf("re-enabled replay %d = %#v, want durable duplicate only", index, replayed)
			}
		}
		if got := networkRowCount(
			t,
			globalDB,
			"network_live_wakes",
			"owner_key",
			"session:reviewer.sess-xyz",
		); got != wakeCount {
			t.Fatalf("wake count after replays = %d, want unchanged %d", got, wakeCount)
		}
	})

	t.Run("Should serialize a burst into one wake and durable coalesced sources", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 3, 10, 0, 0, time.UTC)
		const workers = 10
		requests := make([]store.AcceptNetworkMessageRequest, workers)
		for index := range workers {
			requests[index] = networkWakeAcceptanceRequest(
				t,
				"msg-burst-"+strconv.Itoa(index),
				"wake-burst-"+strconv.Itoa(index),
				"run-burst-"+strconv.Itoa(index),
				now,
			)
			requests[index].Admissions[0].RootID = "root-burst"
		}
		results := make(chan store.AcceptNetworkMessageResult, workers)
		errs := make(chan error, workers)
		var waitGroup sync.WaitGroup
		var ready sync.WaitGroup
		start := make(chan struct{})
		ready.Add(workers)
		for index := range workers {
			waitGroup.Add(1)
			go func(index int) {
				defer waitGroup.Done()
				ready.Done()
				<-start
				result, err := globalDB.AcceptNetworkMessage(context.Background(), requests[index])
				if err != nil {
					errs <- err
					return
				}
				results <- result
			}(index)
		}
		ready.Wait()
		close(start)
		waitGroup.Wait()
		close(results)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("AcceptNetworkMessage(concurrent) error = %v", err)
			}
		}
		admitted := 0
		coalesced := 0
		for result := range results {
			admitted += len(result.Admitted)
			if len(result.Skipped) == 1 && result.Skipped[0].Reason == store.NetworkWakeSkipCoalesced {
				coalesced++
			}
		}
		if admitted != 1 || coalesced != workers-1 {
			t.Fatalf("burst outcomes admitted=%d coalesced=%d, want 1 and %d", admitted, coalesced, workers-1)
		}
		ownerKey := "session:reviewer.sess-xyz"
		if got := networkRowCount(t, globalDB, "network_live_wakes", "owner_key", ownerKey); got != 1 {
			t.Fatalf("burst wake count = %d, want 1", got)
		}
		if got := networkRowCount(t, globalDB, "network_wake_sources", "owner_key", ownerKey); got != workers {
			t.Fatalf("burst source count = %d, want %d", got, workers)
		}
	})

	t.Run("Should linearize a coalesce attempt racing the canonical run claim", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		claimDB, err := OpenGlobalDB(testutil.Context(t), globalDB.Path())
		if err != nil {
			t.Fatalf("OpenGlobalDB(claim connection) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := claimDB.Close(context.Background()); closeErr != nil {
				t.Errorf("Close(claim connection) error = %v", closeErr)
			}
		})
		base := time.Date(2026, 7, 14, 3, 15, 0, 0, time.UTC)
		first := networkWakeAcceptanceRequest(t, "msg-claim-race-1", "wake-claim-race", "run-claim-race", base)
		if _, err := globalDB.AcceptNetworkMessage(testutil.Context(t), first); err != nil {
			t.Fatalf("AcceptNetworkMessage(first) error = %v", err)
		}
		second := networkWakeAcceptanceRequest(
			t,
			"msg-claim-race-2",
			"wake-claim-race-unused",
			"run-claim-race-unused",
			base.Add(50*time.Millisecond),
		)
		second.Admissions[0].RootID = first.Admissions[0].RootID

		start := make(chan struct{})
		acceptResult := make(chan store.AcceptNetworkMessageResult, 1)
		claimResult := make(chan taskpkg.ClaimResult, 1)
		errs := make(chan error, 2)
		var waitGroup sync.WaitGroup
		waitGroup.Add(2)
		go func() {
			defer waitGroup.Done()
			<-start
			result, err := globalDB.AcceptNetworkMessage(context.Background(), second)
			if err != nil {
				errs <- err
				return
			}
			acceptResult <- result
		}()
		go func() {
			defer waitGroup.Done()
			<-start
			result, err := claimDB.ClaimNextRun(context.Background(), taskpkg.ClaimCriteria{
				RunID: "run-claim-race", RunKind: taskpkg.RunKindNetworkWake,
				Scope: taskpkg.ScopeWorkspace, WorkspaceID: networkStoreTestWorkspaceID,
				TargetSessionID: "reviewer.sess-xyz", ClaimerSessionID: "reviewer.sess-xyz",
				LeaseDuration: time.Minute, Now: base.Add(50 * time.Millisecond),
			})
			if err != nil {
				errs <- err
				return
			}
			claimResult <- result
		}()
		close(start)
		waitGroup.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("claim/coalesce race error = %v", err)
			}
		}
		accepted := <-acceptResult
		claimed := <-claimResult
		if claimed.Run.ID != "run-claim-race" || claimed.Run.Status != taskpkg.TaskRunStatusClaimed {
			t.Fatalf("claimed wake = %#v, want run-claim-race claimed", claimed.Run)
		}
		if len(accepted.Admitted) != 0 || len(accepted.Skipped) != 1 {
			t.Fatalf("racing acceptance = %#v, want one accumulation outcome", accepted)
		}
		sourceCount := networkRowCount(
			t,
			globalDB,
			"network_wake_sources",
			"owner_key",
			"session:reviewer.sess-xyz",
		)
		switch accepted.Skipped[0].Reason {
		case store.NetworkWakeSkipCoalesced:
			if sourceCount != 2 {
				t.Fatalf("coalesced race source count = %d, want 2", sourceCount)
			}
		case store.NetworkWakeSkipInFlight:
			if sourceCount != 1 {
				t.Fatalf("claimed-first race source count = %d, want 1", sourceCount)
			}
		default:
			t.Fatalf("racing acceptance skip = %q, want coalesced or in_flight", accepted.Skipped[0].Reason)
		}
		if got := networkRowCount(
			t,
			globalDB,
			"network_live_wakes",
			"owner_key",
			"session:reviewer.sess-xyz",
		); got != 1 {
			t.Fatalf("claim/coalesce race wake count = %d, want 1", got)
		}
	})

	t.Run("Should give one concurrent last-wake winner across distinct roots", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 3, 20, 0, 0, time.UTC)
		const workers = 10
		requests := make([]store.AcceptNetworkMessageRequest, workers)
		for index := range workers {
			requests[index] = networkWakeAcceptanceRequest(
				t,
				"msg-last-wake-"+strconv.Itoa(index),
				"wake-last-wake-"+strconv.Itoa(index),
				"run-last-wake-"+strconv.Itoa(index),
				now,
			)
			requests[index].Admissions[0].Spec.Bounds.MaxWakes = 1
			requests[index].Admissions[0].RootID = "root-last-wake-" + strconv.Itoa(index)
		}
		results := make(chan store.AcceptNetworkMessageResult, workers)
		errs := make(chan error, workers)
		var waitGroup sync.WaitGroup
		var ready sync.WaitGroup
		start := make(chan struct{})
		ready.Add(workers)
		for index := range workers {
			waitGroup.Add(1)
			go func(index int) {
				defer waitGroup.Done()
				ready.Done()
				<-start
				result, err := globalDB.AcceptNetworkMessage(context.Background(), requests[index])
				if err != nil {
					errs <- err
					return
				}
				results <- result
			}(index)
		}
		ready.Wait()
		close(start)
		waitGroup.Wait()
		close(results)
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("AcceptNetworkMessage(concurrent last wake) error = %v", err)
			}
		}
		admitted := 0
		exhausted := 0
		for result := range results {
			admitted += len(result.Admitted)
			if len(result.Skipped) == 1 && result.Skipped[0].Reason == store.NetworkWakeSkipBudgetExhausted {
				exhausted++
			}
		}
		if admitted != 1 || exhausted != workers-1 {
			t.Fatalf("last-wake outcomes admitted=%d exhausted=%d, want 1 and %d", admitted, exhausted, workers-1)
		}
		ownerKey := "session:reviewer.sess-xyz"
		if got := networkRowCount(t, globalDB, "network_live_wakes", "owner_key", ownerKey); got != 1 {
			t.Fatalf("last-wake ledger count = %d, want 1", got)
		}
		if got := networkRowCount(t, globalDB, "network_wake_sources", "owner_key", ownerKey); got != 1 {
			t.Fatalf("last-wake source count = %d, want one winning source", got)
		}
	})
}

func TestGlobalDBSettleNetworkWake(t *testing.T) {
	t.Parallel()

	t.Run("Should replace reservations with actual usage exactly once", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 2, 30, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		accepted, err := globalDB.AcceptNetworkMessage(
			testutil.Context(t),
			networkWakeAcceptanceRequest(t, "msg-settle", "wake-settle", "run-settle", now),
		)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage() error = %v", err)
		}
		reservation := accepted.Admitted[0]
		outcome := store.NetworkWakeOutcome{
			State: store.NetworkWakeStateSucceeded, ActualWallTime: 40 * time.Millisecond,
			ActualInputTokens: 100, ActualOutputTokens: 50, UsageState: store.NetworkWakeUsageActual,
		}
		settlement := claimNetworkWakeSettlementForTest(t, globalDB, reservation)
		settlement.Outcome = outcome
		settled, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement)
		if err != nil {
			t.Fatalf("SettleNetworkWake() error = %v", err)
		}
		if settled.Duplicate || settled.Run.Status != taskpkg.TaskRunStatusCompleted {
			t.Fatalf("SettleNetworkWake() = %#v, want first completed settlement", settled)
		}
		assertNetworkWakeUsage(t, globalDB, "wake-settle", "succeeded", 40, 100, 50, "actual")
		assertNetworkWakeBudget(t, globalDB, reservation.OwnerKey, 1, 40, 100, 50)

		duplicate, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement)
		if err != nil {
			t.Fatalf("SettleNetworkWake(duplicate) error = %v", err)
		}
		if !duplicate.Duplicate || duplicate.Outcome != outcome {
			t.Fatalf("SettleNetworkWake(duplicate) = %#v, want stored identical result", duplicate)
		}
		settlement.Outcome.ActualInputTokens = 900
		settlement.Outcome.ActualOutputTokens = 900
		if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); !errors.Is(
			err,
			taskpkg.ErrNetworkWakeSettlementConflict,
		) {
			t.Fatalf("SettleNetworkWake(conflict) error = %v, want settlement conflict", err)
		}
		assertNetworkWakeUsage(t, globalDB, "wake-settle", "succeeded", 40, 100, 50, "actual")
		assertNetworkWakeBudget(t, globalDB, reservation.OwnerKey, 1, 40, 100, 50)
		if got := networkWakeEventCount(t, globalDB, "wake-settle", networkWakeEventSettled); got != 1 {
			t.Fatalf("settled event count = %d, want 1", got)
		}
	})

	t.Run("Should keep conservative reservations when usage is unavailable", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 2, 40, 0, 0, time.UTC)
		accepted, err := globalDB.AcceptNetworkMessage(
			testutil.Context(t),
			networkWakeAcceptanceRequest(t, "msg-unknown-usage", "wake-unknown", "run-unknown", now),
		)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage() error = %v", err)
		}
		settlement := claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
		settlement.Outcome = store.NetworkWakeOutcome{
			State: store.NetworkWakeStateFailed, UsageState: store.NetworkWakeUsageUnavailable,
			Reason: "provider usage unavailable",
		}
		if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
			t.Fatalf("SettleNetworkWake(unavailable) error = %v", err)
		}
		assertNetworkWakeUsage(t, globalDB, "wake-unknown", "failed", 500, 1000, 1000, "usage_unavailable")
		assertNetworkWakeBudget(t, globalDB, accepted.Admitted[0].OwnerKey, 1, 500, 1000, 1000)
	})

	t.Run("Should charge canceled usage once and never readmit its source envelope", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 2, 41, 0, 0, time.UTC)
		req := networkWakeAcceptanceRequest(t, "msg-canceled", "wake-canceled", "run-canceled", now)
		accepted, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage() error = %v", err)
		}
		settlement := claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
		settlement.Outcome = store.NetworkWakeOutcome{
			State: store.NetworkWakeStateCanceled, ActualWallTime: 25 * time.Millisecond,
			ActualInputTokens: 20, ActualOutputTokens: 10, UsageState: store.NetworkWakeUsageActual,
			Reason: "availability disabled",
		}
		if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
			t.Fatalf("SettleNetworkWake(canceled) error = %v", err)
		}
		assertNetworkWakeUsage(t, globalDB, "wake-canceled", "canceled", 25, 20, 10, "actual")
		assertNetworkWakeBudget(t, globalDB, accepted.Admitted[0].OwnerKey, 1, 25, 20, 10)

		replayed, err := globalDB.AcceptNetworkMessage(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(canceled replay) error = %v", err)
		}
		if !replayed.Duplicate || len(replayed.Admitted) != 0 || len(replayed.Notify) != 0 {
			t.Fatalf("canceled replay = %#v, want durable duplicate without re-admission", replayed)
		}
	})

	t.Run("Should aggregate usage by workspace channel and owner run from ledger detail", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 2, 42, 0, 0, time.UTC)
		const ownerRunID = "parent-run-usage"
		first := networkWakeAcceptanceRequest(t, "msg-usage-1", "wake-usage-1", "run-usage-1", now)
		first.Admissions[0].OwnerKey = "task_run:" + ownerRunID
		accepted, err := globalDB.AcceptNetworkMessage(testutil.Context(t), first)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(first usage) error = %v", err)
		}
		settlement := claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
		settlement.Outcome = store.NetworkWakeOutcome{
			State: store.NetworkWakeStateSucceeded, ActualWallTime: 40 * time.Millisecond,
			ActualInputTokens: 100, ActualOutputTokens: 50, UsageState: store.NetworkWakeUsageActual,
		}
		if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
			t.Fatalf("SettleNetworkWake(first usage) error = %v", err)
		}
		second := networkWakeAcceptanceRequest(
			t,
			"msg-usage-2",
			"wake-usage-2",
			"run-usage-2",
			now.Add(time.Second),
		)
		second.Admissions[0].OwnerKey = "task_run:" + ownerRunID
		accepted, err = globalDB.AcceptNetworkMessage(testutil.Context(t), second)
		if err != nil {
			t.Fatalf("AcceptNetworkMessage(second usage) error = %v", err)
		}
		settlement = claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
		settlement.Outcome = store.NetworkWakeOutcome{
			State: store.NetworkWakeStateFailed, UsageState: store.NetworkWakeUsageUnavailable,
		}
		if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
			t.Fatalf("SettleNetworkWake(second usage) error = %v", err)
		}

		report, err := globalDB.GetNetworkUsage(testutil.Context(t), store.NetworkUsageQuery{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			RunID:       ownerRunID,
		})
		if err != nil {
			t.Fatalf("GetNetworkUsage() error = %v", err)
		}
		if len(report.Details) != 2 {
			t.Fatalf("GetNetworkUsage().Details length = %d, want 2", len(report.Details))
		}
		if report.Details[0].WakeID != "wake-usage-2" || report.Details[1].WakeID != "wake-usage-1" {
			t.Fatalf("GetNetworkUsage().Details order = %#v, want newest first", report.Details)
		}
		if report.Total.WakeCount != 2 || report.Total.ActualWakeCount != 1 ||
			report.Total.UnavailableWakeCount != 1 || report.Total.ReservedWakeCount != 0 ||
			report.Total.ChargedWallTime != 540*time.Millisecond ||
			report.Total.InputTokens != 100 || report.Total.OutputTokens != 50 {
			t.Fatalf("GetNetworkUsage().Total = %#v", report.Total)
		}
		if report.Budget == nil || report.Budget.OwnerKey != "task_run:"+ownerRunID ||
			report.Budget.WakesUsed != 2 || report.Budget.WallTimeUsed != 540*time.Millisecond ||
			report.Budget.InputTokensUsed != 1000 || report.Budget.OutputTokensUsed != 1000 {
			t.Fatalf("GetNetworkUsage().Budget = %#v, want durable owner consumption", report.Budget)
		}
		var detailWall time.Duration
		var detailInput int64
		var detailOutput int64
		for _, detail := range report.Details {
			if detail.UsageState == store.NetworkWakeUsageUnavailable &&
				(detail.InputTokens != 0 || detail.OutputTokens != 0) {
				t.Fatalf("unavailable usage detail = %#v, want zero actual tokens", detail)
			}
			detailWall += detail.ChargedWallTime
			detailInput += detail.InputTokens
			detailOutput += detail.OutputTokens
		}
		if detailWall != report.Total.ChargedWallTime || detailInput != report.Total.InputTokens ||
			detailOutput != report.Total.OutputTokens {
			t.Fatalf(
				"usage aggregate = %#v, details = (%s, %d, %d)",
				report.Total,
				detailWall,
				detailInput,
				detailOutput,
			)
		}
		ownerReport, err := globalDB.GetNetworkUsage(testutil.Context(t), store.NetworkUsageQuery{
			WorkspaceID: networkStoreTestWorkspaceID,
			Owner: &participation.OwnerRef{
				WorkspaceID: networkStoreTestWorkspaceID,
				Kind:        participation.OwnerKindTaskRun,
				ID:          ownerRunID,
			},
		})
		if err != nil {
			t.Fatalf("GetNetworkUsage(owner) error = %v", err)
		}
		if ownerReport.Total != report.Total || len(ownerReport.Details) != len(report.Details) {
			t.Fatalf("GetNetworkUsage(owner) = %#v, want run-filter result %#v", ownerReport, report)
		}

		empty, err := globalDB.GetNetworkUsage(testutil.Context(t), store.NetworkUsageQuery{
			WorkspaceID: "workspace-with-zero-participation",
		})
		if err != nil {
			t.Fatalf("GetNetworkUsage(empty workspace) error = %v", err)
		}
		if len(empty.Details) != 0 || empty.Total != (store.NetworkUsageSummary{}) || empty.Budget != nil {
			t.Fatalf("GetNetworkUsage(empty workspace) = %#v, want zero", empty)
		}
	})

	t.Run("Should paginate 205 usage rows without gaps while totals remain complete", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		now := time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
		for index := range 205 {
			suffix := fmt.Sprintf("%03d", index)
			request := networkWakeAcceptanceRequest(
				t,
				"msg-page-"+suffix,
				"wake-page-"+suffix,
				"run-page-"+suffix,
				now.Add(time.Duration(index)*time.Nanosecond),
			)
			request.Admissions[0].Spec.Bounds.MaxWakes = 205
			accepted, err := globalDB.AcceptNetworkMessage(testutil.Context(t), request)
			if err != nil {
				t.Fatalf("AcceptNetworkMessage(page %d) error = %v", index, err)
			}
			if len(accepted.Admitted) != 1 {
				t.Fatalf("AcceptNetworkMessage(page %d).Admitted = %#v", index, accepted.Admitted)
			}
			settlement := claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
			settlement.Outcome = store.NetworkWakeOutcome{
				State:              store.NetworkWakeStateSucceeded,
				ActualWallTime:     time.Millisecond,
				ActualInputTokens:  1,
				ActualOutputTokens: 1,
				UsageState:         store.NetworkWakeUsageActual,
			}
			if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
				t.Fatalf("SettleNetworkWake(page %d) error = %v", index, err)
			}
		}

		seen := make(map[string]struct{}, 205)
		cursor := ""
		pageSizes := make([]int, 0, 5)
		for page := range 5 {
			report, err := globalDB.GetNetworkUsage(testutil.Context(t), store.NetworkUsageQuery{
				WorkspaceID: networkStoreTestWorkspaceID,
				Channel:     "builders",
				Cursor:      cursor,
				Limit:       50,
				LimitSet:    true,
			})
			if err != nil {
				t.Fatalf("GetNetworkUsage(page %d) error = %v", page, err)
			}
			pageSizes = append(pageSizes, len(report.Details))
			if report.Total.WakeCount != 205 || report.Total.ActualWakeCount != 205 ||
				report.Total.InputTokens != 205 || report.Total.OutputTokens != 205 {
				t.Fatalf("GetNetworkUsage(page %d).Total = %#v, want full 205-row scope", page, report.Total)
			}
			for _, detail := range report.Details {
				if _, duplicate := seen[detail.WakeID]; duplicate {
					t.Fatalf("GetNetworkUsage(page %d) duplicated %q", page, detail.WakeID)
				}
				seen[detail.WakeID] = struct{}{}
			}
			if page < 4 && report.NextCursor == "" {
				t.Fatalf("GetNetworkUsage(page %d).NextCursor is empty", page)
			}
			if page == 4 && report.NextCursor != "" {
				t.Fatalf("GetNetworkUsage(last page).NextCursor = %q, want empty", report.NextCursor)
			}
			cursor = report.NextCursor
		}
		if got := fmt.Sprint(pageSizes); got != "[50 50 50 50 5]" {
			t.Fatalf("usage page sizes = %s, want [50 50 50 50 5]", got)
		}
		if len(seen) != 205 {
			t.Fatalf("unique usage rows = %d, want 205", len(seen))
		}
	})

	t.Run("Should persist budget exhaustion and accumulate later eligible messages", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			reason    string
			configure func(*participation.Bounds, *store.NetworkWakeOutcome)
		}{
			{
				name:   "maximum wakes",
				reason: networkWakeExhaustionMaxWakes,
				configure: func(bounds *participation.Bounds, outcome *store.NetworkWakeOutcome) {
					bounds.MaxWakes = 1
					outcome.ActualWallTime = 10 * time.Millisecond
				},
			},
			{
				name:   "maximum total wall time",
				reason: "max_total_wall_time",
				configure: func(bounds *participation.Bounds, outcome *store.NetworkWakeOutcome) {
					bounds.MaxWakes = 3
					bounds.MaxWakeWallTime = "10ms"
					bounds.MaxTotalWallTime = "10ms"
					outcome.ActualWallTime = 10 * time.Millisecond
				},
			},
			{
				name:   "maximum input tokens",
				reason: "max_input_tokens",
				configure: func(bounds *participation.Bounds, outcome *store.NetworkWakeOutcome) {
					bounds.MaxWakes = 3
					bounds.MaxInputTokens = 1
					outcome.ActualWallTime = time.Millisecond
					outcome.ActualInputTokens = 1
				},
			},
			{
				name:   "maximum output tokens",
				reason: "max_output_tokens",
				configure: func(bounds *participation.Bounds, outcome *store.NetworkWakeOutcome) {
					bounds.MaxWakes = 3
					bounds.MaxOutputTokens = 1
					outcome.ActualWallTime = time.Millisecond
					outcome.ActualOutputTokens = 1
				},
			},
		}
		for index, test := range tests {
			t.Run("Should persist "+test.name+" exhaustion", func(t *testing.T) {
				t.Parallel()

				globalDB := openNetworkConversationRepositoryTestDB(t)
				now := time.Date(2026, 7, 14, 2, 45+index, 0, 0, time.UTC)
				suffix := strconv.Itoa(index)
				first := networkWakeAcceptanceRequest(
					t,
					"msg-budget-first-"+suffix,
					"wake-budget-first-"+suffix,
					"run-budget-first-"+suffix,
					now,
				)
				outcome := store.NetworkWakeOutcome{
					State:      store.NetworkWakeStateSucceeded,
					UsageState: store.NetworkWakeUsageActual,
				}
				test.configure(&first.Admissions[0].Spec.Bounds, &outcome)
				accepted, err := globalDB.AcceptNetworkMessage(testutil.Context(t), first)
				if err != nil {
					t.Fatalf("AcceptNetworkMessage(first) error = %v", err)
				}
				if len(accepted.Admitted) != 1 {
					t.Fatalf("AcceptNetworkMessage(first).Admitted = %#v, want one reservation", accepted.Admitted)
				}
				settlement := claimNetworkWakeSettlementForTest(t, globalDB, accepted.Admitted[0])
				settlement.Outcome = outcome
				if _, err := globalDB.SettleNetworkWake(testutil.Context(t), settlement); err != nil {
					t.Fatalf("SettleNetworkWake(first) error = %v", err)
				}

				second := networkWakeAcceptanceRequest(
					t,
					"msg-budget-second-"+suffix,
					"wake-budget-second-"+suffix,
					"run-budget-second-"+suffix,
					now.Add(time.Second),
				)
				second.Admissions[0].Spec.Bounds = first.Admissions[0].Spec.Bounds
				result, err := globalDB.AcceptNetworkMessage(testutil.Context(t), second)
				if err != nil {
					t.Fatalf("AcceptNetworkMessage(exhausted) error = %v", err)
				}
				if len(result.Admitted) != 0 || len(result.Notify) != 0 || len(result.Skipped) != 1 ||
					result.Skipped[0].Reason != store.NetworkWakeSkipBudgetExhausted {
					t.Fatalf("exhausted acceptance = %#v, want one budget_exhausted skip", result)
				}
				var exhaustedReason string
				if err := globalDB.db.QueryRowContext(
					testutil.Context(t),
					`SELECT exhausted_reason FROM network_participation_budgets
WHERE workspace_id = ? AND owner_key = ?`,
					first.Admissions[0].WorkspaceID,
					first.Admissions[0].OwnerKey,
				).Scan(&exhaustedReason); err != nil {
					t.Fatalf("query exhausted budget error = %v", err)
				}
				if exhaustedReason != test.reason {
					t.Fatalf("exhausted_reason = %q, want %q", exhaustedReason, test.reason)
				}
			})
		}
	})
}

func TestGlobalDBResolveDirectRoom(t *testing.T) {
	t.Parallel()

	t.Run("Should return one durable room for concurrent resolves of the same pair", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		expectedID, expectedSessionA, expectedSessionB, err := store.NetworkDirectRoomIdentity(
			networkStoreTestWorkspaceID,
			"builders",
			"reviewer.sess-xyz",
			"coder.sess-abc",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}

		const workers = 16
		var waitGroup sync.WaitGroup
		errs := make(chan error, workers)
		for index := range workers {
			waitGroup.Add(1)
			go func(index int) {
				defer waitGroup.Done()

				peerA := "coder.sess-abc"
				peerB := "reviewer.sess-xyz"
				if index%2 == 0 {
					peerA, peerB = peerB, peerA
				}
				summary, resolveErr := globalDB.ResolveDirectRoom(testutil.Context(t), store.NetworkDirectRoomEntry{
					WorkspaceID: networkStoreTestWorkspaceID,
					Channel:     "builders",
					SessionA:    peerA,
					SessionB:    peerB,
				})
				if resolveErr != nil {
					errs <- resolveErr
					return
				}
				if summary.DirectID != expectedID ||
					summary.SessionA != expectedSessionA ||
					summary.SessionB != expectedSessionB {
					errs <- errors.New("resolved direct room summary did not match deterministic identity")
				}
			}(index)
		}
		waitGroup.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("ResolveDirectRoom(concurrent) error = %v", err)
			}
		}

		rooms, err := globalDB.ListDirectRooms(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms() error = %v", err)
		}
		if got, want := len(rooms.Directs), 1; got != want {
			t.Fatalf("len(rooms) = %d, want %d", got, want)
		}
		if got, want := rooms.Directs[0].DirectID, expectedID; got != want {
			t.Fatalf("rooms[0].DirectID = %q, want %q", got, want)
		}
	})

	t.Run("Should fail closed when a deterministic direct id is already bound to another pair", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		directID, _, _, err := store.NetworkDirectRoomIdentity(
			networkStoreTestWorkspaceID,
			"builders",
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		insertDirectRoom(t, globalDB.db, networkStoreTestWorkspaceID, "builders", directID, "alpha.sess", "zulu.sess")

		_, err = globalDB.ResolveDirectRoom(testutil.Context(t), store.NetworkDirectRoomEntry{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			SessionA:    "coder.sess-abc",
			SessionB:    "reviewer.sess-xyz",
		})
		if !errors.Is(err, store.ErrNetworkDirectRoomCollision) {
			t.Fatalf("ResolveDirectRoom(collision) error = %v, want ErrNetworkDirectRoomCollision", err)
		}
	})

	t.Run("Should anchor the first message in a room resolved before its timeline exists", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		ctx := testutil.Context(t)
		openedAt := time.Date(2026, 7, 11, 19, 0, 0, 0, time.UTC)
		room, err := globalDB.ResolveDirectRoom(ctx, store.NetworkDirectRoomEntry{
			WorkspaceID:    networkStoreTestWorkspaceID,
			Channel:        "builders",
			SessionA:       "coder.sess-abc",
			SessionB:       "reviewer.sess-xyz",
			OpenedAt:       openedAt,
			LastActivityAt: openedAt,
		})
		if err != nil {
			t.Fatalf("ResolveDirectRoom() error = %v", err)
		}
		if room.OpenedSequence != 0 || room.LastActivitySequence != 0 {
			t.Fatalf(
				"resolved empty room sequences = (%d, %d), want (0, 0)",
				room.OpenedSequence,
				room.LastActivitySequence,
			)
		}
		if _, err := globalDB.ResolveDirectRoom(ctx, store.NetworkDirectRoomEntry{
			WorkspaceID:    networkStoreTestWorkspaceID,
			Channel:        "builders",
			SessionA:       "coder.sess-abc",
			SessionB:       "planner.sess-123",
			OpenedAt:       openedAt.Add(time.Minute),
			LastActivityAt: openedAt.Add(time.Minute),
		}); err != nil {
			t.Fatalf("ResolveDirectRoom(second empty room) error = %v", err)
		}
		firstPage, err := globalDB.ListDirectRooms(
			ctx,
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{Sort: store.NetworkConversationSortCreated, Limit: 1},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms(first empty-room page) error = %v", err)
		}
		if len(firstPage.Directs) != 1 || !firstPage.HasMore || firstPage.NextCursor == "" {
			t.Fatalf("first empty-room page = %#v, want one row and continuation", firstPage)
		}
		assertNetworkCursorHidesSequence(t, firstPage.NextCursor)
		secondPage, err := globalDB.ListDirectRooms(
			ctx,
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{
				Sort:  store.NetworkConversationSortCreated,
				Limit: 1,
				After: firstPage.NextCursor,
			},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms(second empty-room page) error = %v", err)
		}
		if len(secondPage.Directs) != 1 || secondPage.HasMore || secondPage.NextCursor != "" {
			t.Fatalf("second empty-room page = %#v, want one exhausted row", secondPage)
		}
		if firstPage.Directs[0].DirectID == secondPage.Directs[0].DirectID {
			t.Fatalf("empty-room pages repeated direct_id %q", firstPage.Directs[0].DirectID)
		}

		directID, firstWrite, err := writeDirectMessage(
			t,
			globalDB,
			"msg_resolved_room_first",
			"coder.sess-abc",
			"reviewer.sess-xyz",
			"first durable message",
			time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage() error = %v", err)
		}
		if !firstWrite.ConversationOpened {
			t.Fatal("WriteConversationMessage(first direct).ConversationOpened = false, want true")
		}
		continuedPage, err := globalDB.ListDirectRooms(
			ctx,
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{
				Sort:  store.NetworkConversationSortCreated,
				Limit: 1,
				After: firstPage.NextCursor,
			},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms(after first message with prior cursor) error = %v", err)
		}
		if len(continuedPage.Directs) != 1 || continuedPage.Directs[0].DirectID == directID {
			t.Fatalf("continued created page = %#v, want the other room without duplication", continuedPage)
		}
		_, secondWrite, err := writeDirectMessage(
			t,
			globalDB,
			"msg_resolved_room_second",
			"reviewer.sess-xyz",
			"coder.sess-abc",
			"second durable message",
			time.Date(2026, 7, 11, 20, 1, 0, 0, time.UTC),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage(second) error = %v", err)
		}
		if secondWrite.ConversationOpened {
			t.Fatal("WriteConversationMessage(second direct).ConversationOpened = true, want false")
		}
		persisted, err := globalDB.GetDirectRoom(
			ctx,
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			directID,
		)
		if err != nil {
			t.Fatalf("GetDirectRoom() error = %v", err)
		}
		if persisted.OpenedSequence <= 0 || persisted.LastActivitySequence <= persisted.OpenedSequence {
			t.Fatalf(
				"persisted direct sequences = (%d, %d), want immutable positive opening before later activity",
				persisted.OpenedSequence,
				persisted.LastActivitySequence,
			)
		}
	})
}

func TestGlobalDBWriteConversationMessageThreadSummaries(t *testing.T) {
	t.Parallel()

	t.Run("Should open a thread and derive message and participant counts from committed rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 14, 0, 0, 0, time.UTC)
		messages := []store.NetworkConversationMessage{
			threadMessage("msg_thread_root", "thread_store_counts", "coder.sess-abc", "hello builders", startedAt),
			threadMessage(
				"msg_thread_second",
				"thread_store_counts",
				"coder.sess-abc",
				"still here",
				startedAt.Add(time.Minute),
			),
			threadMessage(
				"msg_thread_third",
				"thread_store_counts",
				"reviewer.sess-xyz",
				"reviewing",
				startedAt.Add(2*time.Minute),
			),
		}
		messages[1].PeerTo = "planner.sess-plan"

		firstResult, err := globalDB.WriteConversationMessage(testutil.Context(t), messages[0])
		if err != nil {
			t.Fatalf("WriteConversationMessage(root) error = %v", err)
		}
		if !firstResult.ConversationOpened {
			t.Fatal("WriteConversationMessage(root).ConversationOpened = false, want true")
		}
		for _, message := range messages[1:] {
			result, writeErr := globalDB.WriteConversationMessage(testutil.Context(t), message)
			if writeErr != nil {
				t.Fatalf("WriteConversationMessage(%q) error = %v", message.MessageID, writeErr)
			}
			if result.ConversationOpened {
				t.Fatalf("WriteConversationMessage(%q).ConversationOpened = true, want false", message.MessageID)
			}
		}

		thread, err := globalDB.GetThread(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			"thread_store_counts",
		)
		if err != nil {
			t.Fatalf("GetThread() error = %v", err)
		}
		if got, want := thread.RootMessageID, "msg_thread_root"; got != want {
			t.Fatalf("thread.RootMessageID = %q, want %q", got, want)
		}
		if got, want := thread.MessageCount, 3; got != want {
			t.Fatalf("thread.MessageCount = %d, want %d", got, want)
		}
		if got, want := thread.ParticipantCount, 2; got != want {
			t.Fatalf("thread.ParticipantCount = %d, want %d", got, want)
		}
		if got, want := thread.LastMessagePreview, "reviewing"; got != want {
			t.Fatalf("thread.LastMessagePreview = %q, want %q", got, want)
		}

		threads, err := globalDB.ListThreads(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkThreadQuery{Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListThreads() error = %v", err)
		}
		if got, want := len(threads.Threads), 1; got != want {
			t.Fatalf("len(threads) = %d, want %d", got, want)
		}
		if got, want := threads.Threads[0].ThreadID, "thread_store_counts"; got != want {
			t.Fatalf("threads[0].ThreadID = %q, want %q", got, want)
		}
		if got, want := threads.Threads[0].ParticipantCount, 2; got != want {
			t.Fatalf("threads[0].ParticipantCount = %d, want %d", got, want)
		}

		participants, err := globalDB.ListThreadParticipants(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			"thread_store_counts",
		)
		if err != nil {
			t.Fatalf("ListThreadParticipants() error = %v", err)
		}
		participantSet := make(map[string]bool, len(participants))
		for _, participant := range participants {
			participantSet[participant.SessionID] = true
		}
		for _, want := range []string{"coder.sess-abc", "reviewer.sess-xyz"} {
			if !participantSet[want] {
				t.Fatalf("participants = %#v, want session %q", participantSet, want)
			}
		}

		entries, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			store.NetworkConversationRef{
				WorkspaceID: networkStoreTestWorkspaceID,
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_store_counts",
			},
			store.NetworkConversationMessageQuery{Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(thread) error = %v", err)
		}
		if got, want := len(entries), 3; got != want {
			t.Fatalf("len(entries) = %d, want %d", got, want)
		}

		auditRows, err := globalDB.ListNetworkAudit(testutil.Context(t), store.NetworkAuditQuery{
			WorkspaceID: networkStoreTestWorkspaceID,
			MessageID:   "msg_thread_root",
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListNetworkAudit(root) error = %v", err)
		}
		if got, want := len(auditRows), 1; got != want {
			t.Fatalf("len(auditRows) = %d, want %d", got, want)
		}
		if got, want := auditRows[0].ThreadID, "thread_store_counts"; got != want {
			t.Fatalf("auditRows[0].ThreadID = %q, want %q", got, want)
		}
	})

	t.Run("Should aggregate delivered prompt cost by thread session", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 14, 30, 0, 0, time.UTC)
		thread := threadMessage(
			"msg_thread_cost_root",
			"thread_store_cost",
			"coder.sess-abc",
			"hello cost accounting",
			startedAt,
		)
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), thread); err != nil {
			t.Fatalf("WriteConversationMessage(thread) error = %v", err)
		}

		updates := []store.NetworkThreadSessionTokenStatsUpdate{
			{
				WorkspaceID:           networkStoreTestWorkspaceID,
				Channel:               "builders",
				ThreadID:              "thread_store_cost",
				SessionID:             "reviewer.sess-xyz",
				DeliveredCount:        1,
				PromptSizeBytes:       400,
				EstimatedPromptTokens: 100,
				DeliveredAt:           startedAt.Add(time.Minute),
				UpdatedAt:             startedAt.Add(time.Minute),
			},
			{
				WorkspaceID:           networkStoreTestWorkspaceID,
				Channel:               "builders",
				ThreadID:              "thread_store_cost",
				SessionID:             "reviewer.sess-xyz",
				DeliveredCount:        1,
				PromptSizeBytes:       200,
				EstimatedPromptTokens: 50,
				DeliveredAt:           startedAt.Add(2 * time.Minute),
				UpdatedAt:             startedAt.Add(2 * time.Minute),
			},
		}
		for _, update := range updates {
			if err := globalDB.UpdateNetworkThreadSessionTokenStats(testutil.Context(t), update); err != nil {
				t.Fatalf("UpdateNetworkThreadSessionTokenStats() error = %v", err)
			}
		}

		stats, err := globalDB.ListNetworkThreadSessionTokenStats(
			testutil.Context(t),
			store.NetworkThreadSessionTokenStatsQuery{
				WorkspaceID: networkStoreTestWorkspaceID,
				Channel:     "builders",
				ThreadID:    "thread_store_cost",
				Limit:       10,
			},
		)
		if err != nil {
			t.Fatalf("ListNetworkThreadSessionTokenStats() error = %v", err)
		}
		if got, want := len(stats), 1; got != want {
			t.Fatalf("len(stats) = %d, want %d", got, want)
		}
		if got, want := stats[0].DeliveredCount, int64(2); got != want {
			t.Fatalf("stats[0].DeliveredCount = %d, want %d", got, want)
		}
		if got, want := stats[0].PromptSizeBytes, int64(600); got != want {
			t.Fatalf("stats[0].PromptSizeBytes = %d, want %d", got, want)
		}
		if got, want := stats[0].EstimatedPromptTokens, int64(150); got != want {
			t.Fatalf("stats[0].EstimatedPromptTokens = %d, want %d", got, want)
		}
		if got, want := stats[0].FirstDeliveredAt, startedAt.Add(time.Minute); !got.Equal(want) {
			t.Fatalf("stats[0].FirstDeliveredAt = %s, want %s", got, want)
		}
		if got, want := stats[0].LastDeliveredAt, startedAt.Add(2*time.Minute); !got.Equal(want) {
			t.Fatalf("stats[0].LastDeliveredAt = %s, want %s", got, want)
		}

		summary, err := globalDB.GetThread(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			"thread_store_cost",
		)
		if err != nil {
			t.Fatalf("GetThread() error = %v", err)
		}
		if got, want := summary.DeliveredCount, int64(2); got != want {
			t.Fatalf("summary.DeliveredCount = %d, want %d", got, want)
		}
		if got, want := summary.PromptSizeBytes, int64(600); got != want {
			t.Fatalf("summary.PromptSizeBytes = %d, want %d", got, want)
		}
		if got, want := summary.EstimatedPromptTokens, int64(150); got != want {
			t.Fatalf("summary.EstimatedPromptTokens = %d, want %d", got, want)
		}
	})
}

func TestGlobalDBWriteConversationMessageDirectIsolationAndWorkLookup(t *testing.T) {
	t.Parallel()

	t.Run("Should update only the matching direct room and isolate thread and direct queries", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 15, 0, 0, 0, time.UTC)
		directID, _, _, err := store.NetworkDirectRoomIdentity(
			networkStoreTestWorkspaceID,
			"builders",
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		direct := store.NetworkConversationMessage{
			MessageID:   "msg_direct_review",
			SessionID:   "sess-direct",
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceDirect,
			DirectID:    directID,
			Direction:   "sent",
			PeerFrom:    "coder.sess-abc",
			PeerTo:      "reviewer.sess-xyz",
			Kind:        store.NetworkKindSay,
			WorkID:      "work_direct_review",
			Text:        "please review privately",
			PreviewText: "please review privately",
			Body:        []byte(`{"text":"please review privately"}`),
			Timestamp:   startedAt,
		}
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), direct); err != nil {
			t.Fatalf("WriteConversationMessage(direct) error = %v", err)
		}
		if _, err := globalDB.WriteConversationMessage(
			testutil.Context(t),
			threadMessage(
				"msg_thread_public",
				"thread_store_isolation",
				"founder.sess-main",
				"public update",
				startedAt,
			),
		); err != nil {
			t.Fatalf("WriteConversationMessage(thread) error = %v", err)
		}

		directSummary, err := globalDB.GetDirectRoom(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			directID,
		)
		if err != nil {
			t.Fatalf("GetDirectRoom() error = %v", err)
		}
		if got, want := directSummary.MessageCount, 1; got != want {
			t.Fatalf("directSummary.MessageCount = %d, want %d", got, want)
		}
		if got, want := directSummary.OpenWorkCount, 1; got != want {
			t.Fatalf("directSummary.OpenWorkCount = %d, want %d", got, want)
		}

		directMessages, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			store.NetworkConversationRef{
				WorkspaceID: networkStoreTestWorkspaceID,
				Channel:     "builders",
				Surface:     store.NetworkSurfaceDirect,
				DirectID:    directID,
			},
			store.NetworkConversationMessageQuery{Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(direct) error = %v", err)
		}
		if got, want := len(directMessages), 1; got != want {
			t.Fatalf("len(directMessages) = %d, want %d", got, want)
		}
		if got, want := directMessages[0].MessageID, "msg_direct_review"; got != want {
			t.Fatalf("directMessages[0].MessageID = %q, want %q", got, want)
		}

		threadMessages, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			store.NetworkConversationRef{
				WorkspaceID: networkStoreTestWorkspaceID,
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_store_isolation",
			},
			store.NetworkConversationMessageQuery{Limit: 10},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(thread) error = %v", err)
		}
		if got, want := len(threadMessages), 1; got != want {
			t.Fatalf("len(threadMessages) = %d, want %d", got, want)
		}
		if got, want := threadMessages[0].MessageID, "msg_thread_public"; got != want {
			t.Fatalf("threadMessages[0].MessageID = %q, want %q", got, want)
		}

		work, err := globalDB.GetWork(testutil.Context(t), networkStoreTestWorkspaceID, "work_direct_review")
		if err != nil {
			t.Fatalf("GetWork() error = %v", err)
		}
		if got, want := work.Surface, store.NetworkSurfaceDirect; got != want {
			t.Fatalf("work.Surface = %q, want %q", got, want)
		}
		if got, want := work.DirectID, directID; got != want {
			t.Fatalf("work.DirectID = %q, want %q", got, want)
		}
	})

	t.Run("Should isolate identical conversation and work identities across workspaces", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		foreignWorkspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"network-store-foreign",
			filepath.Join(t.TempDir(), "foreign-workspace"),
		)
		startedAt := time.Date(2026, 7, 14, 15, 0, 0, 0, time.UTC)
		for _, sessionID := range []string{"foreign.coder", "foreign.reviewer"} {
			if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
				ID: sessionID, AgentName: "coder", Provider: "claude",
				WorkspaceID: foreignWorkspaceID, State: "active",
				CreatedAt: startedAt, UpdatedAt: startedAt,
			}); err != nil {
				t.Fatalf("RegisterSession(%q, foreign) error = %v", sessionID, err)
			}
		}

		const (
			channel   = "builders"
			threadID  = "thread_workspace_scope"
			threadMsg = "msg_workspace_thread"
			directMsg = "msg_workspace_direct"
			workID    = "work_workspace_scope"
		)
		mainThread := threadMessage(threadMsg, threadID, "coder.sess-abc", "main thread", startedAt)
		foreignThread := threadMessage(
			threadMsg,
			threadID,
			"foreign.coder",
			"foreign thread",
			startedAt.Add(time.Minute),
		)
		foreignThread.WorkspaceID = foreignWorkspaceID
		for name, message := range map[string]store.NetworkConversationMessage{
			"main thread":    mainThread,
			"foreign thread": foreignThread,
		} {
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%s) error = %v", name, err)
			}
		}

		mainDirectID, _, _, err := store.NetworkDirectRoomIdentity(
			networkStoreTestWorkspaceID,
			channel,
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity(main) error = %v", err)
		}
		foreignDirectID, _, _, err := store.NetworkDirectRoomIdentity(
			foreignWorkspaceID,
			channel,
			"foreign.coder",
			"foreign.reviewer",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity(foreign) error = %v", err)
		}
		if mainDirectID == foreignDirectID {
			t.Fatalf("workspace-qualified direct identities unexpectedly match: %q", mainDirectID)
		}
		for name, message := range map[string]store.NetworkConversationMessage{
			"main direct": {
				MessageID: directMsg, SessionID: "coder.sess-abc",
				WorkspaceID: networkStoreTestWorkspaceID, Channel: channel,
				Surface: store.NetworkSurfaceDirect, DirectID: mainDirectID,
				Direction: "sent", PeerFrom: "coder.sess-abc", PeerTo: "reviewer.sess-xyz",
				Kind: store.NetworkKindSay, WorkID: workID,
				Text: "main direct", PreviewText: "main direct",
				Body: []byte(`{"text":"main direct"}`), Timestamp: startedAt.Add(2 * time.Minute),
			},
			"foreign direct": {
				MessageID: directMsg, SessionID: "foreign.coder",
				WorkspaceID: foreignWorkspaceID, Channel: channel,
				Surface: store.NetworkSurfaceDirect, DirectID: foreignDirectID,
				Direction: "sent", PeerFrom: "foreign.coder", PeerTo: "foreign.reviewer",
				Kind: store.NetworkKindSay, WorkID: workID,
				Text: "foreign direct", PreviewText: "foreign direct",
				Body: []byte(`{"text":"foreign direct"}`), Timestamp: startedAt.Add(3 * time.Minute),
			},
		} {
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%s) error = %v", name, err)
			}
		}

		workspaceCases := []struct {
			name          string
			workspaceID   string
			directID      string
			sessionID     string
			threadPreview string
			directPreview string
		}{
			{
				name: "main", workspaceID: networkStoreTestWorkspaceID, directID: mainDirectID,
				sessionID:     "coder.sess-abc",
				threadPreview: "main thread", directPreview: "main direct",
			},
			{
				name: "foreign", workspaceID: foreignWorkspaceID, directID: foreignDirectID,
				sessionID:     "foreign.coder",
				threadPreview: "foreign thread", directPreview: "foreign direct",
			},
		}
		for _, test := range workspaceCases {
			ref := store.NetworkChannelRef{WorkspaceID: test.workspaceID, Channel: channel}
			thread, err := globalDB.GetThread(testutil.Context(t), ref, threadID)
			if err != nil {
				t.Fatalf("GetThread(%s) error = %v", test.name, err)
			}
			if thread.WorkspaceID != test.workspaceID || thread.LastMessagePreview != test.threadPreview {
				t.Fatalf(
					"GetThread(%s) = %#v, want workspace %q preview %q",
					test.name,
					thread,
					test.workspaceID,
					test.threadPreview,
				)
			}
			threads, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{Limit: 10})
			if err != nil {
				t.Fatalf("ListThreads(%s) error = %v", test.name, err)
			}
			if threads.Total != 1 || len(threads.Threads) != 1 || threads.Threads[0].ThreadID != threadID ||
				threads.Threads[0].LastMessagePreview != test.threadPreview {
				t.Fatalf("ListThreads(%s) = %#v, want only workspace-scoped thread", test.name, threads)
			}

			direct, err := globalDB.GetDirectRoom(testutil.Context(t), ref, test.directID)
			if err != nil {
				t.Fatalf("GetDirectRoom(%s) error = %v", test.name, err)
			}
			if direct.WorkspaceID != test.workspaceID || direct.LastMessagePreview != test.directPreview {
				t.Fatalf(
					"GetDirectRoom(%s) = %#v, want workspace %q preview %q",
					test.name,
					direct,
					test.workspaceID,
					test.directPreview,
				)
			}
			directs, err := globalDB.ListDirectRooms(
				testutil.Context(t),
				ref,
				store.NetworkDirectRoomQuery{SessionID: test.sessionID, Limit: 10},
			)
			if err != nil {
				t.Fatalf("ListDirectRooms(%s) error = %v", test.name, err)
			}
			if directs.Total != 1 || len(directs.Directs) != 1 || directs.Directs[0].DirectID != test.directID ||
				directs.Directs[0].LastMessagePreview != test.directPreview {
				t.Fatalf("ListDirectRooms(%s) = %#v, want only workspace-scoped direct room", test.name, directs)
			}

			for surface, conversationRef := range map[string]store.NetworkConversationRef{
				"thread": {
					WorkspaceID: test.workspaceID, Channel: channel,
					Surface: store.NetworkSurfaceThread, ThreadID: threadID,
				},
				"direct": {
					WorkspaceID: test.workspaceID, Channel: channel,
					Surface: store.NetworkSurfaceDirect, DirectID: test.directID,
				},
			} {
				messages, err := globalDB.ListConversationMessages(
					testutil.Context(t),
					conversationRef,
					store.NetworkConversationMessageQuery{Limit: 10},
				)
				if err != nil {
					t.Fatalf("ListConversationMessages(%s, %s) error = %v", test.name, surface, err)
				}
				wantPreview := test.threadPreview
				if surface == "direct" {
					wantPreview = test.directPreview
				}
				if len(messages) != 1 || messages[0].WorkspaceID != test.workspaceID ||
					messages[0].PreviewText != wantPreview {
					t.Fatalf(
						"ListConversationMessages(%s, %s) = %#v, want workspace-scoped payload",
						test.name,
						surface,
						messages,
					)
				}
			}

			work, err := globalDB.GetWork(testutil.Context(t), test.workspaceID, workID)
			if err != nil {
				t.Fatalf("GetWork(%s) error = %v", test.name, err)
			}
			if work.WorkspaceID != test.workspaceID || work.DirectID != test.directID {
				t.Fatalf(
					"GetWork(%s) = %#v, want workspace %q direct %q",
					test.name,
					work,
					test.workspaceID,
					test.directID,
				)
			}
		}
	})
}

func TestGlobalDBConversationQueriesSupportCursorsAndFilters(t *testing.T) {
	t.Parallel()

	t.Run("Should page thread and direct summaries and filter conversation messages", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 18, 0, 0, 0, time.UTC)
		threadMessages := []store.NetworkConversationMessage{
			threadMessage("msg_query_one", "thread_query_cursors", "coder.sess-abc", "first", startedAt),
			threadMessage(
				"msg_query_two",
				"thread_query_cursors",
				"coder.sess-abc",
				"second",
				startedAt.Add(time.Minute),
			),
			threadMessage(
				"msg_query_three",
				"thread_query_cursors",
				"reviewer.sess-xyz",
				"third",
				startedAt.Add(2*time.Minute),
			),
			threadMessage(
				"msg_query_other",
				"thread_query_other",
				"founder.sess-main",
				"other",
				startedAt.Add(3*time.Minute),
			),
		}
		threadMessages[1].PeerTo = "reviewer.sess-xyz"
		threadMessages[1].WorkID = "work_query_filter"
		for _, message := range threadMessages {
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%q) error = %v", message.MessageID, err)
			}
		}

		firstThreadPage, err := globalDB.ListThreads(
			testutil.Context(t), store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkThreadQuery{Limit: 1},
		)
		if err != nil {
			t.Fatalf("ListThreads(first page) error = %v", err)
		}
		if got, want := len(firstThreadPage.Threads), 1; got != want {
			t.Fatalf("len(firstThreadPage) = %d, want %d", got, want)
		}
		if got, want := firstThreadPage.Threads[0].ThreadID, "thread_query_other"; got != want {
			t.Fatalf("firstThreadPage[0].ThreadID = %q, want %q", got, want)
		}
		assertNetworkCursorHidesSequence(t, firstThreadPage.NextCursor)
		secondThreadPage, err := globalDB.ListThreads(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkThreadQuery{
				Limit: 1,
				After: firstThreadPage.NextCursor,
			},
		)
		if err != nil {
			t.Fatalf("ListThreads(second page) error = %v", err)
		}
		if got, want := len(secondThreadPage.Threads), 1; got != want {
			t.Fatalf("len(secondThreadPage) = %d, want %d", got, want)
		}
		if got, want := secondThreadPage.Threads[0].ThreadID, "thread_query_cursors"; got != want {
			t.Fatalf("secondThreadPage[0].ThreadID = %q, want %q", got, want)
		}

		ref := store.NetworkConversationRef{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_query_cursors",
		}
		before, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			ref,
			store.NetworkConversationMessageQuery{
				BeforeMessageID: "msg_query_three",
				Limit:           10,
			},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(before) error = %v", err)
		}
		if got, want := messageIDs(before), []string{"msg_query_one", "msg_query_two"}; !sameStrings(got, want) {
			t.Fatalf("before message IDs = %v, want %v", got, want)
		}
		after, err := globalDB.ListConversationMessages(testutil.Context(t), ref, store.NetworkConversationMessageQuery{
			AfterMessageID: "msg_query_one",
			Limit:          10,
		})
		if err != nil {
			t.Fatalf("ListConversationMessages(after) error = %v", err)
		}
		if got, want := messageIDs(after), []string{"msg_query_two", "msg_query_three"}; !sameStrings(got, want) {
			t.Fatalf("after message IDs = %v, want %v", got, want)
		}
		filtered, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			ref,
			store.NetworkConversationMessageQuery{
				WorkID: "work_query_filter",
				Limit:  10,
			},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(work filter) error = %v", err)
		}
		if got, want := messageIDs(filtered), []string{"msg_query_two"}; !sameStrings(got, want) {
			t.Fatalf("filtered message IDs = %v, want %v", got, want)
		}

		firstDirectID, _, err := writeDirectMessage(
			t,
			globalDB,
			"msg_direct_cursor_one",
			"coder.sess-abc",
			"reviewer.sess-xyz",
			"first direct",
			startedAt.Add(4*time.Minute),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage(first) error = %v", err)
		}
		secondDirectID, _, err := writeDirectMessage(
			t,
			globalDB,
			"msg_direct_cursor_two",
			"coder.sess-abc",
			"planner.sess-123",
			"second direct",
			startedAt.Add(5*time.Minute),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage(second) error = %v", err)
		}

		firstDirectPage, err := globalDB.ListDirectRooms(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{
				SessionID: "coder.sess-abc",
				Limit:     1,
			},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms(first page) error = %v", err)
		}
		if got, want := firstDirectPage.Total, 2; got != want || firstDirectPage.Limit != 1 ||
			!firstDirectPage.HasMore || firstDirectPage.NextCursor == "" {
			t.Fatalf("first direct page = %#v, want total=%d limit=1 has_more cursor", firstDirectPage, want)
		}
		if got, want := len(firstDirectPage.Directs), 1; got != want {
			t.Fatalf("len(firstDirectPage) = %d, want %d", got, want)
		}
		if got, want := firstDirectPage.Directs[0].DirectID, secondDirectID; got != want {
			t.Fatalf("firstDirectPage[0].DirectID = %q, want %q", got, want)
		}
		assertNetworkCursorHidesSequence(t, firstDirectPage.NextCursor)
		secondDirectPage, err := globalDB.ListDirectRooms(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{
				SessionID: "coder.sess-abc",
				Limit:     1,
				After:     firstDirectPage.NextCursor,
			},
		)
		if err != nil {
			t.Fatalf("ListDirectRooms(second page) error = %v", err)
		}
		if got, want := secondDirectPage.Total, 2; got != want || secondDirectPage.Limit != 1 ||
			secondDirectPage.HasMore || secondDirectPage.NextCursor != "" {
			t.Fatalf("second direct page = %#v, want total=%d limit=1 exhausted", secondDirectPage, want)
		}
		if got, want := len(secondDirectPage.Directs), 1; got != want {
			t.Fatalf("len(secondDirectPage) = %d, want %d", got, want)
		}
		if got, want := secondDirectPage.Directs[0].DirectID, firstDirectID; got != want {
			t.Fatalf("secondDirectPage[0].DirectID = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBConversationQueriesUseStableCompleteOrdering(t *testing.T) {
	t.Parallel()

	t.Run("Should filter before paging and walk alphabetical thread pages without gaps", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
		messages := []store.NetworkConversationMessage{
			threadMessage("msg_sort_zulu", "thread_sort_zulu", "peer.zulu", "Zulu", startedAt),
			threadMessage("msg_sort_alpha", "thread_sort_alpha", "peer.alpha", "alpha", startedAt),
			threadMessage("msg_sort_beta", "thread_sort_beta", "peer.beta", "Beta", startedAt),
			threadMessage("msg_sort_work", "thread_sort_work", "peer.work", "Work", startedAt),
		}
		messages[3].PeerTo = "peer.reviewer"
		messages[3].WorkID = "work_sort_open"
		for _, message := range messages {
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%q) error = %v", message.MessageID, err)
			}
		}

		ref := store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"}
		firstPage, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Sort:  store.NetworkConversationSortAlphabetical,
			Limit: 2,
		})
		if err != nil {
			t.Fatalf("ListThreads(first alphabetical page) error = %v", err)
		}
		if got, want := firstPage.Total, 4; got != want || firstPage.Limit != 2 || !firstPage.HasMore ||
			firstPage.NextCursor == "" {
			t.Fatalf("first alphabetical page = %#v, want total=%d limit=2 has_more cursor", firstPage, want)
		}
		secondPage, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Sort:  store.NetworkConversationSortAlphabetical,
			Limit: 2,
			After: firstPage.NextCursor,
		})
		if err != nil {
			t.Fatalf("ListThreads(second alphabetical page) error = %v", err)
		}
		if got, want := secondPage.Total, 4; got != want || secondPage.Limit != 2 || secondPage.HasMore ||
			secondPage.NextCursor != "" {
			t.Fatalf("second alphabetical page = %#v, want total=%d limit=2 exhausted", secondPage, want)
		}
		gotIDs := append(threadSummaryIDs(firstPage.Threads), threadSummaryIDs(secondPage.Threads)...)
		wantIDs := []string{
			"thread_sort_alpha",
			"thread_sort_beta",
			"thread_sort_work",
			"thread_sort_zulu",
		}
		if !sameStrings(gotIDs, wantIDs) {
			t.Fatalf("alphabetical thread IDs = %v, want %v", gotIDs, wantIDs)
		}

		hasWork := true
		work, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			HasWork: &hasWork,
			Limit:   10,
		})
		if err != nil {
			t.Fatalf("ListThreads(has_work) error = %v", err)
		}
		if got, want := work.Total, 1; got != want || work.Limit != 10 {
			t.Fatalf("has-work page = %#v, want total=%d limit=10", work, want)
		}
		if got, want := threadSummaryIDs(work.Threads), []string{"thread_sort_work"}; !sameStrings(got, want) {
			t.Fatalf("has-work thread IDs = %v, want %v", got, want)
		}

		search, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Search: "ALP",
			Limit:  10,
		})
		if err != nil {
			t.Fatalf("ListThreads(search) error = %v", err)
		}
		if got, want := search.Total, 1; got != want || search.Limit != 10 {
			t.Fatalf("search page = %#v, want total=%d limit=10", search, want)
		}
		if got, want := threadSummaryIDs(search.Threads), []string{"thread_sort_alpha"}; !sameStrings(got, want) {
			t.Fatalf("search thread IDs = %v, want %v", got, want)
		}

		_, err = globalDB.ListThreads(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: "ws-other", Channel: "builders"},
			store.NetworkThreadQuery{After: firstPage.NextCursor, Limit: 2},
		)
		if !errors.Is(err, store.ErrNetworkCursorInvalid) {
			t.Fatalf("ListThreads(cross-workspace cursor) error = %v, want ErrNetworkCursorInvalid", err)
		}

		_, err = globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Sort:   store.NetworkConversationSortAlphabetical,
			Search: "alpha",
			Limit:  2,
			After:  firstPage.NextCursor,
		})
		if !errors.Is(err, store.ErrNetworkCursorInvalid) {
			t.Fatalf("ListThreads(reused filtered cursor) error = %v, want ErrNetworkCursorInvalid", err)
		}

		empty, err := globalDB.ListThreads(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "empty"},
			store.NetworkThreadQuery{},
		)
		if err != nil {
			t.Fatalf("ListThreads(empty default page) error = %v", err)
		}
		if empty.Total != 0 || empty.Limit != store.NetworkConversationListDefaultLimit ||
			len(empty.Threads) != 0 || empty.HasMore || empty.NextCursor != "" {
			t.Fatalf("empty default page = %#v, want truthful zero total and normalized default limit", empty)
		}
	})

	t.Run("Should continue from the captured sort tuple when the anchor mutates", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)
		for index, threadID := range []string{"thread_mutation_oldest", "thread_mutation_anchor", "thread_mutation_newest"} {
			message := threadMessage(
				"msg_mutation_"+strconv.Itoa(index),
				threadID,
				"peer.mutation",
				threadID,
				startedAt.Add(time.Duration(index)*time.Minute),
			)
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%q) error = %v", message.MessageID, err)
			}
		}

		ref := store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"}
		first, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Search: "THREAD_MUTATION",
			Limit:  2,
		})
		if err != nil {
			t.Fatalf("ListThreads(first mutation page) error = %v", err)
		}
		if got, want := threadSummaryIDs(first.Threads), []string{
			"thread_mutation_newest",
			"thread_mutation_anchor",
		}; !sameStrings(got, want) {
			t.Fatalf("first mutation page IDs = %v, want %v", got, want)
		}

		mutation := threadMessage(
			"msg_mutation_anchor_new",
			"thread_mutation_anchor",
			"peer.mutation",
			"anchor moved",
			startedAt.Add(10*time.Minute),
		)
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), mutation); err != nil {
			t.Fatalf("WriteConversationMessage(anchor mutation) error = %v", err)
		}

		second, err := globalDB.ListThreads(testutil.Context(t), ref, store.NetworkThreadQuery{
			Search: "thread_mutation",
			Limit:  2,
			After:  first.NextCursor,
		})
		if err != nil {
			t.Fatalf("ListThreads(second mutation page) error = %v", err)
		}
		if got, want := threadSummaryIDs(second.Threads), []string{"thread_mutation_oldest"}; !sameStrings(got, want) {
			t.Fatalf("second mutation page IDs = %v, want %v", got, want)
		}
	})

	t.Run("Should return the latest message tail in chronological order", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
		for _, id := range []string{"zulu", "yankee", "xray", "whiskey", "victor"} {
			message := threadMessage(
				"msg_tail_"+id,
				"thread_tail",
				"peer.tail",
				id,
				startedAt,
			)
			if _, err := globalDB.WriteConversationMessage(testutil.Context(t), message); err != nil {
				t.Fatalf("WriteConversationMessage(%q) error = %v", message.MessageID, err)
			}
		}
		ref := store.NetworkConversationRef{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_tail",
		}
		latest, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			ref,
			store.NetworkConversationMessageQuery{Limit: 2},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(latest) error = %v", err)
		}
		if got, want := messageIDs(latest), []string{"msg_tail_whiskey", "msg_tail_victor"}; !sameStrings(got, want) {
			t.Fatalf("latest message IDs = %v, want %v", got, want)
		}

		older, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			ref,
			store.NetworkConversationMessageQuery{BeforeMessageID: "msg_tail_whiskey", Limit: 2},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(older) error = %v", err)
		}
		if got, want := messageIDs(older), []string{"msg_tail_yankee", "msg_tail_xray"}; !sameStrings(got, want) {
			t.Fatalf("older message IDs = %v, want %v", got, want)
		}

		newer, err := globalDB.ListConversationMessages(
			testutil.Context(t),
			ref,
			store.NetworkConversationMessageQuery{AfterMessageID: "msg_tail_xray", Limit: 2},
		)
		if err != nil {
			t.Fatalf("ListConversationMessages(newer) error = %v", err)
		}
		if got, want := messageIDs(newer), []string{"msg_tail_whiskey", "msg_tail_victor"}; !sameStrings(got, want) {
			t.Fatalf("newer message IDs = %v, want %v", got, want)
		}
	})
}

func TestGlobalDBNetworkChannelProjectionTracksTimelineWrites(t *testing.T) {
	t.Parallel()

	t.Run("Should aggregate channel timeline projections atomically", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 7, 10, 14, 0, 0, 0, time.UTC)
		first := threadMessage(
			"msg_projection_first",
			"thread_projection",
			"peer.author",
			"first public",
			startedAt,
		)
		first.Mentions = []string{"peer.mentioned"}
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), first); err != nil {
			t.Fatalf("WriteConversationMessage(first) error = %v", err)
		}
		presence := store.NetworkMessageEntry{
			MessageID:   "msg_projection_presence",
			SessionID:   "sess-projection-presence",
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Direction:   "received",
			PeerFrom:    "peer.presence",
			Kind:        store.NetworkKindGreet,
			Body:        []byte(`{"display_name":"Presence"}`),
			Timestamp:   startedAt.Add(time.Minute),
		}
		if err := globalDB.WriteNetworkMessage(testutil.Context(t), presence); err != nil {
			t.Fatalf("WriteNetworkMessage(presence) error = %v", err)
		}
		if err := globalDB.WriteNetworkMessage(testutil.Context(t), presence); err != nil {
			t.Fatalf("WriteNetworkMessage(duplicate presence) error = %v", err)
		}
		second := threadMessage(
			"msg_projection_second",
			"thread_projection",
			"peer.second",
			"latest public",
			startedAt.Add(2*time.Minute),
		)
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), second); err != nil {
			t.Fatalf("WriteConversationMessage(second) error = %v", err)
		}
		directID, _, err := writeDirectMessage(
			t,
			globalDB,
			"msg_projection_direct",
			"peer.second",
			"peer.direct",
			"private",
			startedAt.Add(3*time.Minute),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage() error = %v", err)
		}

		projections, err := globalDB.ListNetworkChannelProjections(
			testutil.Context(t),
			store.NetworkChannelProjectionQuery{WorkspaceID: networkStoreTestWorkspaceID},
		)
		if err != nil {
			t.Fatalf("ListNetworkChannelProjections() error = %v", err)
		}
		if got, want := len(projections), 1; got != want {
			t.Fatalf("len(projections) = %d, want %d", got, want)
		}
		projection := projections[0]
		if got, want := projection.MessageCount, 2; got != want {
			t.Fatalf("projection.MessageCount = %d, want %d", got, want)
		}
		if got, want := projection.PresenceCount, 1; got != want {
			t.Fatalf("projection.PresenceCount = %d, want %d", got, want)
		}
		if got, want := projection.HistoricalParticipantCount, 3; got != want {
			t.Fatalf("projection.HistoricalParticipantCount = %d, want %d", got, want)
		}
		if projection.LastActivityAt == nil || !projection.LastActivityAt.Equal(startedAt.Add(2*time.Minute)) {
			t.Fatalf("projection.LastActivityAt = %v, want %s", projection.LastActivityAt, startedAt.Add(2*time.Minute))
		}
		if got, want := projection.LastMessagePreview, "latest public"; got != want {
			t.Fatalf("projection.LastMessagePreview = %q, want %q", got, want)
		}

		counts, err := globalDB.ListNetworkChannelKindCounts(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
		)
		if err != nil {
			t.Fatalf("ListNetworkChannelKindCounts() error = %v", err)
		}
		if got, want := len(counts), 1; got != want {
			t.Fatalf("len(kind counts) = %d, want %d", got, want)
		}
		if got, want := counts[0].Kind, store.NetworkKindSay; got != want {
			t.Fatalf("kind count kind = %q, want %q", got, want)
		}
		if got, want := counts[0].Count, 2; got != want {
			t.Fatalf("kind count = %d, want %d", got, want)
		}

		recents, err := globalDB.ListNetworkRecents(
			testutil.Context(t),
			store.NetworkRecentQuery{WorkspaceID: networkStoreTestWorkspaceID, Limit: 2},
		)
		if err != nil {
			t.Fatalf("ListNetworkRecents() error = %v", err)
		}
		if got, want := len(recents), 2; got != want {
			t.Fatalf("len(recents) = %d, want %d", got, want)
		}
		if got, want := recents[0].ContainerID, directID; got != want {
			t.Fatalf("recents[0].ContainerID = %q, want %q", got, want)
		}
		if got, want := recents[1].ContainerID, "thread_projection"; got != want {
			t.Fatalf("recents[1].ContainerID = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBWriteConversationMessageWorkReceiptTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should apply receipt states and resume needs-input work from a say message", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 19, 0, 0, 0, time.UTC)
		opening := threadMessage(
			"msg_receipt_open",
			"thread_receipt_transitions",
			"coder.sess-abc",
			"please build",
			startedAt,
		)
		opening.PeerTo = "reviewer.sess-xyz"
		opening.WorkID = "work_receipt_failed"
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), opening); err != nil {
			t.Fatalf("WriteConversationMessage(opening) error = %v", err)
		}

		rejected := threadReceiptMessage(
			"msg_receipt_rejected",
			"thread_receipt_transitions",
			"reviewer.sess-xyz",
			"work_receipt_failed",
			"rejected",
			startedAt.Add(time.Minute),
		)
		result, err := globalDB.WriteConversationMessage(testutil.Context(t), rejected)
		if err != nil {
			t.Fatalf("WriteConversationMessage(rejected receipt) error = %v", err)
		}
		if !result.WorkTransitioned || result.WorkState != store.NetworkWorkStateFailed {
			t.Fatalf("rejected receipt result = %#v, want failed transition", result)
		}
		failedWork, err := globalDB.GetWork(testutil.Context(t), networkStoreTestWorkspaceID, "work_receipt_failed")
		if err != nil {
			t.Fatalf("GetWork(failed) error = %v", err)
		}
		if got, want := failedWork.State, store.NetworkWorkStateFailed; got != want {
			t.Fatalf("failedWork.State = %q, want %q", got, want)
		}
		if got, want := failedWork.OpenedBySessionID, "coder.sess-abc"; got != want {
			t.Fatalf("failedWork.OpenedBySessionID = %q, want %q", got, want)
		}
		if got, want := failedWork.TargetSessionID, "reviewer.sess-xyz"; got != want {
			t.Fatalf("failedWork.TargetSessionID = %q, want %q", got, want)
		}

		needsInput := threadMessage(
			"msg_needs_input_open",
			"thread_receipt_transitions",
			"coder.sess-abc",
			"needs input work",
			startedAt.Add(2*time.Minute),
		)
		needsInput.PeerTo = "reviewer.sess-xyz"
		needsInput.WorkID = "work_needs_input"
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), needsInput); err != nil {
			t.Fatalf("WriteConversationMessage(needs input opening) error = %v", err)
		}
		traceNeedsInput := threadTraceMessage(
			"msg_needs_input_trace",
			"thread_receipt_transitions",
			"reviewer.sess-xyz",
			"work_needs_input",
			store.NetworkWorkStateNeedsInput,
			startedAt.Add(3*time.Minute),
		)
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), traceNeedsInput); err != nil {
			t.Fatalf("WriteConversationMessage(needs input trace) error = %v", err)
		}
		resume := threadMessage(
			"msg_needs_input_resume",
			"thread_receipt_transitions",
			"coder.sess-abc",
			"resume work",
			startedAt.Add(4*time.Minute),
		)
		resume.PeerTo = "reviewer.sess-xyz"
		resume.WorkID = "work_needs_input"
		resumeResult, err := globalDB.WriteConversationMessage(testutil.Context(t), resume)
		if err != nil {
			t.Fatalf("WriteConversationMessage(resume) error = %v", err)
		}
		if !resumeResult.WorkTransitioned || resumeResult.WorkState != store.NetworkWorkStateWorking {
			t.Fatalf("resume result = %#v, want working transition", resumeResult)
		}
		resumedWork, err := globalDB.GetWork(testutil.Context(t), networkStoreTestWorkspaceID, "work_needs_input")
		if err != nil {
			t.Fatalf("GetWork(resumed) error = %v", err)
		}
		if got, want := resumedWork.State, store.NetworkWorkStateWorking; got != want {
			t.Fatalf("resumedWork.State = %q, want %q", got, want)
		}

		reopened := openGlobalDBForTest(t, globalDB.path)
		reopenedWork, err := reopened.GetWork(
			testutil.Context(t),
			networkStoreTestWorkspaceID,
			"work_needs_input",
		)
		if err != nil {
			t.Fatalf("GetWork(reopened) error = %v", err)
		}
		if got, want := reopenedWork.State, store.NetworkWorkStateWorking; got != want {
			t.Fatalf("reopened work state = %q, want durable %q", got, want)
		}
		if got, want := reopenedWork.OpenedBySessionID, "coder.sess-abc"; got != want {
			t.Fatalf("reopened work opener = %q, want %q", got, want)
		}
		if got, want := reopenedWork.TargetSessionID, "reviewer.sess-xyz"; got != want {
			t.Fatalf("reopened work target = %q, want %q", got, want)
		}
	})
}

func TestGlobalDBWriteConversationMessageRejectsInvalidLifecycleWrites(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid work mutations without keeping timeline or audit side effects", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 20, 0, 0, 0, time.UTC)
		receiptWithoutWork := threadReceiptMessage(
			"msg_receipt_missing_work",
			"thread_invalid_lifecycle",
			"reviewer.sess-xyz",
			"work_missing",
			"accepted",
			startedAt,
		)
		_, err := globalDB.WriteConversationMessage(testutil.Context(t), receiptWithoutWork)
		if err == nil {
			t.Fatal("WriteConversationMessage(receipt without work) error = nil, want non-nil")
		}
		assertNoTimelineOrAuditRows(t, globalDB, receiptWithoutWork.MessageID)

		missingPeerTo := threadMessage(
			"msg_work_missing_peer",
			"thread_invalid_lifecycle",
			"coder.sess-abc",
			"missing target",
			startedAt.Add(time.Minute),
		)
		missingPeerTo.WorkID = "work_missing_peer"
		_, err = globalDB.WriteConversationMessage(testutil.Context(t), missingPeerTo)
		if err == nil {
			t.Fatal("WriteConversationMessage(work without peer_to) error = nil, want non-nil")
		}
		assertNoTimelineOrAuditRows(t, globalDB, missingPeerTo.MessageID)

		opening := threadMessage(
			"msg_invalid_work_open",
			"thread_invalid_lifecycle",
			"coder.sess-abc",
			"open work",
			startedAt.Add(2*time.Minute),
		)
		opening.PeerTo = "reviewer.sess-xyz"
		opening.WorkID = "work_invalid_lifecycle"
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), opening); err != nil {
			t.Fatalf("WriteConversationMessage(opening) error = %v", err)
		}

		accepted := threadReceiptMessage(
			"msg_receipt_accepted",
			"thread_invalid_lifecycle",
			"reviewer.sess-xyz",
			"work_invalid_lifecycle",
			"accepted",
			startedAt.Add(3*time.Minute),
		)
		result, err := globalDB.WriteConversationMessage(testutil.Context(t), accepted)
		if err != nil {
			t.Fatalf("WriteConversationMessage(accepted receipt) error = %v", err)
		}
		if result.WorkTransitioned || result.WorkState != store.NetworkWorkStateSubmitted {
			t.Fatalf("accepted receipt result = %#v, want no submitted transition", result)
		}

		invalidTrace := threadTraceMessage(
			"msg_trace_invalid_state",
			"thread_invalid_lifecycle",
			"reviewer.sess-xyz",
			"work_invalid_lifecycle",
			store.NetworkWorkStateSubmitted,
			startedAt.Add(4*time.Minute),
		)
		_, err = globalDB.WriteConversationMessage(testutil.Context(t), invalidTrace)
		if err == nil {
			t.Fatal("WriteConversationMessage(invalid trace) error = nil, want non-nil")
		}
		assertNoTimelineOrAuditRows(t, globalDB, invalidTrace.MessageID)

		canceledOpening := threadMessage(
			"msg_canceled_work_open",
			"thread_invalid_lifecycle",
			"coder.sess-abc",
			"cancel me",
			startedAt.Add(5*time.Minute),
		)
		canceledOpening.PeerTo = "reviewer.sess-xyz"
		canceledOpening.WorkID = "work_receipt_canceled"
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), canceledOpening); err != nil {
			t.Fatalf("WriteConversationMessage(canceled opening) error = %v", err)
		}
		canceled := threadReceiptMessage(
			"msg_receipt_canceled",
			"thread_invalid_lifecycle",
			"reviewer.sess-xyz",
			"work_receipt_canceled",
			"canceled",
			startedAt.Add(6*time.Minute),
		)
		result, err = globalDB.WriteConversationMessage(testutil.Context(t), canceled)
		if err != nil {
			t.Fatalf("WriteConversationMessage(canceled receipt) error = %v", err)
		}
		if !result.WorkTransitioned || result.WorkState != store.NetworkWorkStateCanceled {
			t.Fatalf("canceled receipt result = %#v, want canceled transition", result)
		}

		unsupportedOpening := threadMessage(
			"msg_unsupported_work_open",
			"thread_invalid_lifecycle",
			"coder.sess-abc",
			"unsupported receipt",
			startedAt.Add(7*time.Minute),
		)
		unsupportedOpening.PeerTo = "reviewer.sess-xyz"
		unsupportedOpening.WorkID = "work_receipt_unsupported"
		if _, err := globalDB.WriteConversationMessage(testutil.Context(t), unsupportedOpening); err != nil {
			t.Fatalf("WriteConversationMessage(unsupported opening) error = %v", err)
		}
		unsupported := threadReceiptMessage(
			"msg_receipt_unsupported",
			"thread_invalid_lifecycle",
			"reviewer.sess-xyz",
			"work_receipt_unsupported",
			"ignored",
			startedAt.Add(8*time.Minute),
		)
		_, err = globalDB.WriteConversationMessage(testutil.Context(t), unsupported)
		if err == nil {
			t.Fatal("WriteConversationMessage(unsupported receipt) error = nil, want non-nil")
		}
		assertNoTimelineOrAuditRows(t, globalDB, unsupported.MessageID)

		mismatched := threadTraceMessage(
			"msg_work_container_mismatch",
			"thread_other_container",
			"reviewer.sess-xyz",
			"work_invalid_lifecycle",
			store.NetworkWorkStateWorking,
			startedAt.Add(9*time.Minute),
		)
		_, err = globalDB.WriteConversationMessage(testutil.Context(t), mismatched)
		if !errors.Is(err, store.ErrNetworkWorkContainerMismatch) {
			t.Fatalf("WriteConversationMessage(container mismatch) error = %v, want mismatch", err)
		}
		assertNoTimelineOrAuditRows(t, globalDB, mismatched.MessageID)
	})
}

func TestGlobalDBConversationQueryErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid cursors and missing conversation rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		startedAt := time.Date(2026, 5, 5, 21, 0, 0, 0, time.UTC)
		if _, err := globalDB.WriteConversationMessage(
			testutil.Context(t),
			threadMessage("msg_query_error", "thread_query_error", "coder.sess-abc", "query errors", startedAt),
		); err != nil {
			t.Fatalf("WriteConversationMessage(thread) error = %v", err)
		}
		directID, _, err := writeDirectMessage(
			t,
			globalDB,
			"msg_direct_query_error",
			"coder.sess-abc",
			"reviewer.sess-xyz",
			"direct query errors",
			startedAt.Add(time.Minute),
		)
		if err != nil {
			t.Fatalf("writeDirectMessage() error = %v", err)
		}

		_, err = globalDB.GetThread(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			"thread_missing",
		)
		if !errors.Is(err, store.ErrNetworkConversationNotFound) {
			t.Fatalf("GetThread(missing) error = %v, want ErrNetworkConversationNotFound", err)
		}
		_, err = globalDB.GetDirectRoom(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			"direct_missing",
		)
		if err == nil {
			t.Fatal("GetDirectRoom(invalid id) error = nil, want non-nil")
		}
		_, err = globalDB.GetWork(testutil.Context(t), networkStoreTestWorkspaceID, "")
		if err == nil {
			t.Fatal("GetWork(empty) error = nil, want non-nil")
		}
		_, err = globalDB.GetWork(testutil.Context(t), networkStoreTestWorkspaceID, "work_missing")
		if !errors.Is(err, store.ErrNetworkConversationNotFound) {
			t.Fatalf("GetWork(missing) error = %v, want ErrNetworkConversationNotFound", err)
		}

		_, err = globalDB.ListThreads(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkThreadQuery{
				Limit: 10,
				After: "thread_missing",
			},
		)
		if err == nil {
			t.Fatal("ListThreads(missing cursor) error = nil, want non-nil")
		}
		_, err = globalDB.ListDirectRooms(
			testutil.Context(t),
			store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
			store.NetworkDirectRoomQuery{
				SessionID: "other.sess-peer",
				Limit:     10,
				After:     directID,
			},
		)
		if err == nil {
			t.Fatal("ListDirectRooms(missing cursor for peer) error = nil, want non-nil")
		}
		ref := store.NetworkConversationRef{
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceThread,
			ThreadID:    "thread_query_error",
		}
		_, err = globalDB.ListConversationMessages(testutil.Context(t), ref, store.NetworkConversationMessageQuery{
			BeforeMessageID: "msg_missing",
			Limit:           10,
		})
		if !errors.Is(err, store.ErrNetworkCursorInvalid) {
			t.Fatalf("ListConversationMessages(missing cursor) error = %v, want ErrNetworkCursorInvalid", err)
		}
		_, err = globalDB.ListConversationMessages(testutil.Context(t), ref, store.NetworkConversationMessageQuery{
			BeforeMessageID: "msg_query_error",
			AfterMessageID:  "msg_query_error",
			Limit:           10,
		})
		if err == nil {
			t.Fatal("ListConversationMessages(conflicting cursors) error = nil, want non-nil")
		}
	})
}

func TestGlobalDBWriteConversationMessageIdempotencyAndRollback(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should process duplicates before lifecycle mutation and reject terminal continuations atomically",
		func(t *testing.T) {
			t.Parallel()

			globalDB := openNetworkConversationRepositoryTestDB(t)
			startedAt := time.Date(2026, 5, 5, 16, 0, 0, 0, time.UTC)
			opening := threadMessage("msg_work_open", "thread_store_work", "coder.sess-abc", "please review", startedAt)
			opening.PeerTo = "reviewer.sess-xyz"
			opening.WorkID = "work_thread_review"

			result, err := globalDB.WriteConversationMessage(testutil.Context(t), opening)
			if err != nil {
				t.Fatalf("WriteConversationMessage(opening) error = %v", err)
			}
			if !result.WorkOpened || result.WorkState != store.NetworkWorkStateSubmitted {
				t.Fatalf("opening work result = %#v, want submitted open work", result)
			}

			duplicateOpening := opening
			duplicateOpening.Timestamp = startedAt.Add(time.Hour)
			duplicate, err := globalDB.WriteConversationMessage(testutil.Context(t), duplicateOpening)
			if err != nil {
				t.Fatalf("WriteConversationMessage(duplicate opening) error = %v", err)
			}
			if !duplicate.Duplicate || duplicate.WorkOpened || duplicate.WorkTransitioned {
				t.Fatalf("duplicate opening result = %#v, want duplicate without lifecycle mutation", duplicate)
			}

			completed := threadTraceMessage(
				"msg_work_complete",
				"thread_store_work",
				"reviewer.sess-xyz",
				"work_thread_review",
				store.NetworkWorkStateCompleted,
				startedAt.Add(time.Minute),
			)
			completedResult, err := globalDB.WriteConversationMessage(testutil.Context(t), completed)
			if err != nil {
				t.Fatalf("WriteConversationMessage(completed) error = %v", err)
			}
			if !completedResult.WorkTransitioned || completedResult.WorkState != store.NetworkWorkStateCompleted {
				t.Fatalf("completed result = %#v, want completed transition", completedResult)
			}

			duplicateCompleted, err := globalDB.WriteConversationMessage(testutil.Context(t), completed)
			if err != nil {
				t.Fatalf("WriteConversationMessage(duplicate completed) error = %v", err)
			}
			if !duplicateCompleted.Duplicate {
				t.Fatalf("duplicate completed result = %#v, want duplicate", duplicateCompleted)
			}

			rejected := threadTraceMessage(
				"msg_work_after_terminal",
				"thread_store_work",
				"reviewer.sess-xyz",
				"work_thread_review",
				store.NetworkWorkStateWorking,
				startedAt.Add(2*time.Minute),
			)
			_, err = globalDB.WriteConversationMessage(testutil.Context(t), rejected)
			if !errors.Is(err, store.ErrNetworkWorkClosed) {
				t.Fatalf("WriteConversationMessage(after terminal) error = %v, want ErrNetworkWorkClosed", err)
			}

			thread, err := globalDB.GetThread(
				testutil.Context(t),
				store.NetworkChannelRef{WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders"},
				"thread_store_work",
			)
			if err != nil {
				t.Fatalf("GetThread() error = %v", err)
			}
			if got, want := thread.MessageCount, 2; got != want {
				t.Fatalf("thread.MessageCount = %d, want %d", got, want)
			}
			if got, want := thread.OpenWorkCount, 0; got != want {
				t.Fatalf("thread.OpenWorkCount = %d, want %d", got, want)
			}
			if got := networkRowCount(
				t,
				globalDB,
				"network_timeline_log",
				"message_id",
				"msg_work_after_terminal",
			); got != 0 {
				t.Fatalf("rolled-back message count = %d, want 0", got)
			}
			if got := networkRowCount(
				t,
				globalDB,
				"network_audit_log",
				"message_id",
				"msg_work_after_terminal",
			); got != 0 {
				t.Fatalf("rolled-back audit count = %d, want 0", got)
			}
		},
	)

	t.Run("Should roll back message and side effects when direct room binding fails", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		directID, _, _, err := store.NetworkDirectRoomIdentity(
			networkStoreTestWorkspaceID,
			"builders",
			"coder.sess-abc",
			"reviewer.sess-xyz",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		message := store.NetworkConversationMessage{
			MessageID:   "msg_direct_collision_rollback",
			SessionID:   "sess-direct-collision",
			WorkspaceID: networkStoreTestWorkspaceID,
			Channel:     "builders",
			Surface:     store.NetworkSurfaceDirect,
			DirectID:    directID,
			Direction:   "sent",
			PeerFrom:    "coder.sess-abc",
			PeerTo:      "other.sess-peer",
			Kind:        store.NetworkKindSay,
			Text:        "wrong room",
			PreviewText: "wrong room",
			Body:        []byte(`{"text":"wrong room"}`),
			Timestamp:   time.Date(2026, 5, 5, 16, 30, 0, 0, time.UTC),
		}

		_, err = globalDB.WriteConversationMessage(testutil.Context(t), message)
		if !errors.Is(err, store.ErrNetworkDirectRoomCollision) {
			t.Fatalf("WriteConversationMessage(direct collision) error = %v, want ErrNetworkDirectRoomCollision", err)
		}
		if got := networkRowCount(t, globalDB, "network_timeline_log", "message_id", message.MessageID); got != 0 {
			t.Fatalf("rolled-back direct message count = %d, want 0", got)
		}
		if got := networkRowCount(t, globalDB, "network_direct_rooms", "channel", "builders"); got != 0 {
			t.Fatalf("direct room count = %d, want 0", got)
		}
		if got := networkRowCount(t, globalDB, "network_audit_log", "message_id", message.MessageID); got != 0 {
			t.Fatalf("rolled-back direct audit count = %d, want 0", got)
		}
	})
}

func TestGlobalDBWriteConversationMessageRejectsRawClaimTokens(t *testing.T) {
	t.Parallel()

	t.Run("Should reject raw claim tokens before persisting message or audit rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		message := threadMessage(
			"msg_raw_claim_token",
			"thread_store_redaction",
			"coder.sess-abc",
			"agh_claim_NET05TOKEN123",
			time.Date(2026, 5, 5, 17, 0, 0, 0, time.UTC),
		)

		_, err := globalDB.WriteConversationMessage(testutil.Context(t), message)
		assertRawClaimTokenRejected(t, err, "agh_claim_NET05TOKEN123")
		if got := networkRowCount(t, globalDB, "network_timeline_log", "message_id", message.MessageID); got != 0 {
			t.Fatalf("raw-token message count = %d, want 0", got)
		}
		if got := networkRowCount(t, globalDB, "network_audit_log", "message_id", message.MessageID); got != 0 {
			t.Fatalf("raw-token audit count = %d, want 0", got)
		}
	})

	t.Run("Should reject raw claim tokens in timeline text fields even when body is redacted", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		message := threadMessage(
			"msg_raw_claim_token_text",
			"thread_store_redaction",
			"coder.sess-abc",
			"agh_claim_NET05TEXT123",
			time.Date(2026, 5, 5, 17, 30, 0, 0, time.UTC),
		)
		message.Body = []byte(`{"text":"[redacted]"}`)

		_, err := globalDB.WriteConversationMessage(testutil.Context(t), message)
		assertRawClaimTokenRejected(t, err, "agh_claim_NET05TEXT123")
		assertNoTimelineOrAuditRows(t, globalDB, message.MessageID)
	})

	t.Run("Should reject raw claim tokens in direct audit writes", func(t *testing.T) {
		t.Parallel()

		globalDB := openNetworkConversationRepositoryTestDB(t)
		err := globalDB.WriteNetworkAudit(testutil.Context(t), store.NetworkAuditEntry{
			WorkspaceID: networkStoreTestWorkspaceID,
			SessionID:   "sess-audit-token",
			Direction:   "rejected",
			Kind:        store.NetworkKindSay,
			Channel:     "builders",
			PeerFrom:    "coder.sess-abc",
			MessageID:   "msg_audit_token",
			Reason:      "agh_claim_NET05TOKEN123",
			Size:        1,
		})
		assertRawClaimTokenRejected(t, err, "agh_claim_NET05TOKEN123")
		if got := networkRowCount(t, globalDB, "network_audit_log", "message_id", "msg_audit_token"); got != 0 {
			t.Fatalf("raw-token audit count = %d, want 0", got)
		}
	})
}

func threadMessage(
	messageID string,
	threadID string,
	peerFrom string,
	text string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: networkStoreTestWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
		Direction:   "sent",
		PeerFrom:    peerFrom,
		Kind:        store.NetworkKindSay,
		Text:        text,
		PreviewText: text,
		Body:        []byte(`{"text":"` + text + `"}`),
		Timestamp:   timestamp,
	}
}

func networkWakeAcceptanceRequest(
	t *testing.T,
	messageID string,
	wakeID string,
	taskRunID string,
	timestamp time.Time,
) store.AcceptNetworkMessageRequest {
	t.Helper()

	directID, _, _, err := store.NetworkDirectRoomIdentity(
		networkStoreTestWorkspaceID,
		"builders",
		"coder.sess-abc",
		"reviewer.sess-xyz",
	)
	if err != nil {
		t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
	}
	spec := participation.Spec{
		Version: participation.SpecVersion, Mode: participation.ModeLive,
		WorkspaceID: networkStoreTestWorkspaceID, ChannelStrategy: participation.StrategyNamed,
		ChannelID: "builders", Source: participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes: 3, MaxWakeWallTime: "500ms", MaxTotalWallTime: "2s",
			MaxInputTokens: 1000, MaxOutputTokens: 1000, MaxWakeDepth: 3,
			CoalesceWindow: "250ms",
		},
	}
	return store.AcceptNetworkMessageRequest{
		Message: store.NetworkConversationMessage{
			MessageID: messageID, SessionID: "coder.sess-abc",
			WorkspaceID: networkStoreTestWorkspaceID, Channel: "builders",
			Surface: store.NetworkSurfaceDirect, DirectID: directID,
			Direction: "sent", PeerFrom: "coder.sess-abc", PeerTo: "reviewer.sess-xyz",
			Kind: store.NetworkKindSay, Text: "Please review", PreviewText: "Please review",
			Body: []byte(`{"text":"Please review"}`), Timestamp: timestamp,
		},
		Dispositions: []store.NetworkMessageDisposition{{
			RecipientSessionID: "reviewer.sess-xyz", Decision: store.NetworkDispositionDeliver,
		}},
		Admissions: []store.NetworkWakeAdmissionInput{{
			WorkspaceID:        networkStoreTestWorkspaceID,
			RecipientSessionID: "reviewer.sess-xyz", OwnerKey: "session:reviewer.sess-xyz",
			Spec: spec, Trigger: store.NetworkWakeTriggerDirect, Eligible: true, Addressed: true,
			RootID: messageID, Depth: 0, WakeID: wakeID, TaskRunID: taskRunID,
		}},
	}
}

func assertNetworkAcceptanceRowCounts(
	t *testing.T,
	globalDB *GlobalDB,
	messageID string,
	ownerKey string,
	wakeCount int,
	sourceCount int,
) {
	t.Helper()

	if got := networkRowCount(t, globalDB, "network_timeline_log", "message_id", messageID); got != 1 {
		t.Fatalf("timeline count for %q = %d, want 1", messageID, got)
	}
	if got := networkRowCount(t, globalDB, "network_message_dispositions", "message_id", messageID); got != 1 {
		t.Fatalf("disposition count for %q = %d, want 1", messageID, got)
	}
	if got := networkRowCount(t, globalDB, "network_live_wakes", "owner_key", ownerKey); got != wakeCount {
		t.Fatalf("wake count for %q = %d, want %d", ownerKey, got, wakeCount)
	}
	if got := networkRowCount(t, globalDB, "network_wake_sources", "owner_key", ownerKey); got != sourceCount {
		t.Fatalf("wake source count for %q = %d, want %d", ownerKey, got, sourceCount)
	}
}

func assertNetworkWakeUsage(
	t *testing.T,
	globalDB *GlobalDB,
	wakeID string,
	wantState string,
	wantWallMS int64,
	wantInput int64,
	wantOutput int64,
	wantUsageState string,
) {
	t.Helper()

	var state string
	var wallMS, inputTokens, outputTokens int64
	var usageState string
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT state, actual_wall_ms, input_tokens, output_tokens, usage_state
FROM network_live_wakes WHERE wake_id = ?`,
		wakeID,
	).Scan(&state, &wallMS, &inputTokens, &outputTokens, &usageState); err != nil {
		t.Fatalf("query network wake usage error = %v", err)
	}
	if state != wantState || wallMS != wantWallMS || inputTokens != wantInput ||
		outputTokens != wantOutput || usageState != wantUsageState {
		t.Fatalf(
			"wake usage = (%q, %d, %d, %d, %q), want (%q, %d, %d, %d, %q)",
			state, wallMS, inputTokens, outputTokens, usageState,
			wantState, wantWallMS, wantInput, wantOutput, wantUsageState,
		)
	}
}

func claimNetworkWakeSettlementForTest(
	t *testing.T,
	globalDB *GlobalDB,
	reservation store.WakeReservation,
) taskpkg.NetworkWakeSettlement {
	t.Helper()

	run, err := globalDB.GetTaskRun(testutil.Context(t), reservation.TaskRunID)
	if err != nil {
		t.Fatalf("GetTaskRun(network wake) error = %v", err)
	}
	wakeID, targetSessionID, ownerKey := run.NetworkWakeCorrelation()
	actor, err := taskpkg.DeriveDaemonActorContext("network-wake-test", "test.network_wake")
	if err != nil {
		t.Fatalf("DeriveDaemonActorContext() error = %v", err)
	}
	claimedBy := actor.Actor
	now := globalDB.now().UTC()
	claim, err := globalDB.ClaimNextRun(testutil.Context(t), taskpkg.ClaimCriteria{
		RunID:            run.ID,
		Scope:            taskpkg.ScopeWorkspace,
		WorkspaceID:      run.WorkspaceID,
		RunKind:          taskpkg.RunKindNetworkWake,
		TargetSessionID:  targetSessionID,
		ClaimerSessionID: targetSessionID,
		ClaimedBy:        &claimedBy,
		LeaseDuration:    taskpkg.DefaultRunLeaseDuration,
		Now:              now,
	})
	if err != nil {
		t.Fatalf("ClaimNextRun(network wake) error = %v", err)
	}
	return taskpkg.NetworkWakeSettlement{
		WorkspaceID: run.WorkspaceID, WakeID: wakeID, TaskRunID: run.ID,
		ClaimToken: claim.ClaimToken, TargetSessionID: targetSessionID, OwnerKey: ownerKey,
		Now: now, Actor: actor,
	}
}

func networkWakeEventCount(t *testing.T, globalDB *GlobalDB, wakeID string, eventType string) int {
	t.Helper()

	var count int
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM network_wake_events WHERE wake_id = ? AND event_type = ?`,
		wakeID,
		eventType,
	).Scan(&count); err != nil {
		t.Fatalf("count network wake events error = %v", err)
	}
	return count
}

func assertNetworkWakeBudget(
	t *testing.T,
	globalDB *GlobalDB,
	ownerKey string,
	wantWakes int,
	wantWallMS int64,
	wantInput int64,
	wantOutput int64,
) {
	t.Helper()

	var wakes int
	var wallMS, inputTokens, outputTokens int64
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT wakes_used, wall_ms_used, input_tokens_used, output_tokens_used
FROM network_participation_budgets WHERE owner_key = ?`,
		ownerKey,
	).Scan(&wakes, &wallMS, &inputTokens, &outputTokens); err != nil {
		t.Fatalf("query network wake budget error = %v", err)
	}
	if wakes != wantWakes || wallMS != wantWallMS || inputTokens != wantInput || outputTokens != wantOutput {
		t.Fatalf(
			"wake budget = (%d, %d, %d, %d), want (%d, %d, %d, %d)",
			wakes, wallMS, inputTokens, outputTokens,
			wantWakes, wantWallMS, wantInput, wantOutput,
		)
	}
}

func threadTraceMessage(
	messageID string,
	threadID string,
	peerFrom string,
	workID string,
	state string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: networkStoreTestWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
		Direction:   "received",
		PeerFrom:    peerFrom,
		Kind:        store.NetworkKindTrace,
		WorkID:      workID,
		PreviewText: state,
		Body:        []byte(`{"state":"` + state + `"}`),
		Timestamp:   timestamp,
	}
}

func threadReceiptMessage(
	messageID string,
	threadID string,
	peerFrom string,
	workID string,
	status string,
	timestamp time.Time,
) store.NetworkConversationMessage {
	return store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: networkStoreTestWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    threadID,
		Direction:   "received",
		PeerFrom:    peerFrom,
		Kind:        store.NetworkKindReceipt,
		WorkID:      workID,
		PreviewText: status,
		Body:        []byte(`{"status":"` + status + `"}`),
		Timestamp:   timestamp,
	}
}

func writeDirectMessage(
	t *testing.T,
	globalDB *GlobalDB,
	messageID string,
	peerFrom string,
	peerTo string,
	text string,
	timestamp time.Time,
) (string, store.NetworkConversationWriteResult, error) {
	t.Helper()

	directID, _, _, err := store.NetworkDirectRoomIdentity(networkStoreTestWorkspaceID, "builders", peerFrom, peerTo)
	if err != nil {
		return "", store.NetworkConversationWriteResult{}, err
	}
	result, err := globalDB.WriteConversationMessage(testutil.Context(t), store.NetworkConversationMessage{
		MessageID:   messageID,
		SessionID:   peerFrom,
		WorkspaceID: networkStoreTestWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    directID,
		Direction:   "sent",
		PeerFrom:    peerFrom,
		PeerTo:      peerTo,
		Kind:        store.NetworkKindSay,
		Text:        text,
		PreviewText: text,
		Body:        []byte(`{"text":"` + text + `"}`),
		Timestamp:   timestamp,
	})
	return directID, result, err
}

func openNetworkConversationRepositoryTestDB(t *testing.T) *GlobalDB {
	t.Helper()

	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"network-store",
		filepath.Join(t.TempDir(), "workspace"),
	)
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	for _, sessionID := range []string{
		"alpha.sess",
		"coder.sess-abc",
		"founder.sess-main",
		"other.sess-peer",
		"peer.alpha",
		"peer.author",
		"peer.beta",
		"peer.direct",
		"peer.mentioned",
		"peer.mutation",
		"peer.presence",
		"peer.reviewer",
		"peer.second",
		"peer.tail",
		"peer.work",
		"peer.zulu",
		"planner.sess-123",
		"planner.sess-plan",
		"reviewer.sess-xyz",
		"sess-direct",
		"sess-direct-collision",
		"sess-projection-presence",
		"zulu.sess",
	} {
		if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
			ID:          sessionID,
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			State:       "active",
			CreatedAt:   now,
			UpdatedAt:   now,
		}); err != nil {
			t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
		}
	}
	return globalDB
}

func messageIDs(messages []store.NetworkConversationMessage) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.MessageID)
	}
	return ids
}

func threadSummaryIDs(threads []store.NetworkThreadSummary) []string {
	ids := make([]string, 0, len(threads))
	for _, thread := range threads {
		ids = append(ids, thread.ThreadID)
	}
	return ids
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func assertNetworkCursorHidesSequence(t *testing.T, cursor string) {
	t.Helper()

	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		t.Fatalf("decode network cursor error = %v", err)
	}
	if strings.Contains(string(decoded), "sequence") {
		t.Fatalf("network cursor exposes global sequence: %s", decoded)
	}
}

func assertRawClaimTokenRejected(t *testing.T, err error, token string) {
	t.Helper()

	if err == nil {
		t.Fatal("raw claim token write error = nil, want rejection")
	}
	if !strings.Contains(err.Error(), "contains raw claim_token material") {
		t.Fatalf("raw claim token write error = %v, want claim-token validation rejection", err)
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("raw claim token write error leaked token: %v", err)
	}
}

func assertNoTimelineOrAuditRows(t *testing.T, globalDB *GlobalDB, messageID string) {
	t.Helper()

	if got := networkRowCount(t, globalDB, "network_timeline_log", "message_id", messageID); got != 0 {
		t.Fatalf("timeline count for %q = %d, want 0", messageID, got)
	}
	if got := networkRowCount(t, globalDB, "network_audit_log", "message_id", messageID); got != 0 {
		t.Fatalf("audit count for %q = %d, want 0", messageID, got)
	}
}

func networkRowCount(t *testing.T, globalDB *GlobalDB, table string, column string, value string) int {
	t.Helper()

	var count int
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT COUNT(*) FROM `+table+` WHERE `+column+` = ?`,
		value,
	).Scan(&count); err != nil {
		t.Fatalf("count %s.%s error = %v", table, column, err)
	}
	return count
}
