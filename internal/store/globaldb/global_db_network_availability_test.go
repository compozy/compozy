package globaldb

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestNetworkAvailabilityPersistsMonotonicTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should persist monotonic availability transitions across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		db, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}

		initial, err := db.GetNetworkAvailability(ctx)
		if err != nil {
			t.Fatalf("GetNetworkAvailability(initial) error = %v", err)
		}
		if !initial.Enabled || initial.Epoch != 1 {
			t.Fatalf("initial availability = %#v, want enabled epoch 1", initial)
		}

		unchanged, err := db.SetNetworkAvailability(ctx, true, "test:same")
		if err != nil {
			t.Fatalf("SetNetworkAvailability(same) error = %v", err)
		}
		if !unchanged.Enabled || unchanged.Epoch != 1 || unchanged.UpdatedBy != "test:same" {
			t.Fatalf("unchanged availability = %#v, want enabled epoch 1 with updated actor", unchanged)
		}

		disabled, err := db.SetNetworkAvailability(ctx, false, "test:disable")
		if err != nil {
			t.Fatalf("SetNetworkAvailability(disable) error = %v", err)
		}
		if disabled.Enabled || disabled.Epoch != 2 || disabled.UpdatedBy != "test:disable" {
			t.Fatalf("disabled availability = %#v, want disabled epoch 2", disabled)
		}

		disabledAgain, err := db.SetNetworkAvailability(ctx, false, "test:disable-again")
		if err != nil {
			t.Fatalf("SetNetworkAvailability(disable again) error = %v", err)
		}
		if disabledAgain.Enabled || disabledAgain.Epoch != 2 {
			t.Fatalf("disabled availability repeat = %#v, want disabled epoch 2", disabledAgain)
		}
		summaries, err := db.ListEventSummaries(ctx, EventSummaryQuery{
			Type:  "network.availability.changed",
			Limit: 10,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries(network availability) error = %v", err)
		}
		if got, want := len(summaries), 1; got != want {
			t.Fatalf("len(network availability summaries) = %d, want %d", got, want)
		}
		var content struct {
			PreviousEnabled bool   `json:"previous_enabled"`
			Enabled         bool   `json:"enabled"`
			Epoch           int64  `json:"epoch"`
			UpdatedBy       string `json:"updated_by"`
		}
		if err := json.Unmarshal(summaries[0].Content, &content); err != nil {
			t.Fatalf("json.Unmarshal(network availability summary) error = %v", err)
		}
		if content.PreviousEnabled != true || content.Enabled || content.Epoch != 2 ||
			content.UpdatedBy != "test:disable" {
			t.Fatalf("network availability summary content = %#v", content)
		}

		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
				t.Fatalf("Close(reopened) error = %v", closeErr)
			}
		})
		persisted, err := reopened.GetNetworkAvailability(ctx)
		if err != nil {
			t.Fatalf("GetNetworkAvailability(reopened) error = %v", err)
		}
		if persisted.Enabled || persisted.Epoch != 2 || persisted.UpdatedBy != "test:disable-again" {
			t.Fatalf("reopened availability = %#v, want persisted disabled epoch 2", persisted)
		}
	})
}

func TestNetworkAvailabilityRejectsBlankActorWithoutMutation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a blank actor without mutating availability", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		db.now = func() time.Time {
			return time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
		}
		if _, err := db.SetNetworkAvailability(ctx, false, "   "); err == nil ||
			!strings.Contains(err.Error(), "updated_by is required") {
			t.Fatalf("SetNetworkAvailability(blank actor) error = %v, want updated_by validation", err)
		}
		availability, err := db.GetNetworkAvailability(ctx)
		if err != nil {
			t.Fatalf("GetNetworkAvailability() error = %v", err)
		}
		if !availability.Enabled || availability.Epoch != 1 {
			t.Fatalf("availability after rejected write = %#v, want enabled epoch 1", availability)
		}
	})
}

func TestNetworkAvailabilityRejectsPersistedBlankActor(t *testing.T) {
	t.Parallel()

	t.Run("Should reject persisted availability with a blank actor", func(t *testing.T) {
		t.Parallel()

		_, err := networkAvailabilityFromGenerated(
			1,
			1,
			"2026-07-13T12:00:00Z",
			"   ",
		)
		if err == nil || !strings.Contains(err.Error(), "updated_by is required") {
			t.Fatalf("networkAvailabilityFromGenerated(blank actor) error = %v, want provenance validation", err)
		}
	})
}
