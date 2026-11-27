package computers

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
)

func TestComputerPlacement(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	candidate := func(id remoteproto.ComputerID, weight, cpus int, cpu float64, free uint64) PlacementCandidate {
		return PlacementCandidate{
			ComputerID: id, Name: string(id), Enabled: true, Connected: true, Weight: weight,
			WorkspaceID: "ws-" + string(id), Resources: ResourceSample{
				CPUCount: cpus, CPUUtil: new(cpu), MemTotal: new(uint64(100)), MemAvailable: new(free), SampledAt: now,
			},
		}
	}
	assertSelection := func(t *testing.T, id remoteproto.ComputerID, candidates ...PlacementCandidate) {
		t.Helper()
		got, err := SelectPlacement(now, candidates)
		if err != nil || got.ComputerID != id || got.WorkspaceID != "ws-"+string(id) {
			t.Fatalf("SelectPlacement() = %+v, %v, want %s and its workspace", got, err, id)
		}
	}

	// Placement Rules 13 and 16 bind the new session to the winning computer's matching workspace.
	t.Run("Should select the highest score and its matching workspace [UT-102]", func(t *testing.T) {
		t.Parallel()
		a, b := candidate("a", 50, 8, 0.2, 50), candidate("b", 50, 16, 0.8, 50)
		assertSelection(t, "a", b, a)
	})
	t.Run("Should exclude saturated, manual-only and unmatched computers [UT-103]", func(t *testing.T) {
		t.Parallel()
		a, b := candidate("a", 25, 1, 0.9, 50), candidate("b", 100, 16, 0.95, 50)
		c, d := candidate("c", 0, 64, 0, 100), candidate("d", 100, 64, 0, 100)
		d.WorkspaceID = ""
		lowMemory := candidate("e", 100, 64, 0, 5)
		assertSelection(t, "a", b, c, d, lowMemory, a)
	})
	t.Run("Should prefer weight, then local, then name when samples are stale [UT-104]", func(t *testing.T) {
		t.Parallel()
		local, a, b := candidate(LocalID, 50, 1, .8, 10), candidate("a", 50, 3, .8, 10), candidate("b", 100, 1, .9, 10)
		for _, entry := range []*PlacementCandidate{&local, &a, &b} {
			entry.Resources.SampledAt = now.Add(-16 * time.Second)
		}
		manual, unmatched := candidate("manual", 0, 32, 0, 100), candidate("unmatched", 100, 32, 0, 100)
		unmatched.WorkspaceID = ""
		assertSelection(t, "b", local, a, b, manual, unmatched)
		b.Weight = 50
		assertSelection(t, LocalID, a, b, local)
		assertSelection(t, "a", b, a)
	})
	t.Run("Should explain why no computer can run the draft [UT-106]", func(t *testing.T) {
		t.Parallel()
		local, manual, missing := candidate(LocalID, 50, 1, 0, 100), candidate("manual", 0, 32, 0, 100),
			candidate("missing", 100, 32, 0, 100)
		local.Enabled, missing.WorkspaceID = false, ""
		_, err := SelectPlacement(now, []PlacementCandidate{local, manual, missing})
		failure, ok := errors.AsType[*Failure](err)
		want := map[string]string{
			"local":   "local_execution_disabled",
			"manual":  "manual_only",
			"missing": "project_not_found",
		}
		if !ok || failure.Status != http.StatusConflict || failure.Code != "no_eligible_computer" ||
			failure.Message != "No computer can run this project." || !reflect.DeepEqual(failure.Details, want) {
			t.Fatalf("SelectPlacement() failure = %+v, want ineligible reasons %v", failure, want)
		}
	})
	t.Run("Should treat any missing resource measurement as unsampled [UT-108]", func(t *testing.T) {
		t.Parallel()
		fresh := candidate("fresh", 25, 1, .94, 6)
		for _, omit := range []func(*ResourceSample){
			func(s *ResourceSample) { s.CPUUtil = nil },
			func(s *ResourceSample) { s.MemTotal = nil },
			func(s *ResourceSample) { s.MemAvailable = nil },
		} {
			unsampled := candidate("unsampled", 100, 128, .01, 99)
			omit(&unsampled.Resources)
			assertSelection(t, "fresh", unsampled, fresh)
			assertSelection(t, "unsampled", unsampled)
		}
	})
}

func TestLocalExecutionSettings(t *testing.T) {
	t.Parallel()

	// Rule 14 requires a remote computer before switching this installation to controller-only.
	t.Run("Should leave execution enabled when no remote computer has been added [UT-101]", func(t *testing.T) {
		t.Parallel()
		initial := config.DefaultComputersConfig()
		next := initial
		next.LocalRunAgents = false
		err := ValidateLocalExecutionChange(initial, next, 0)
		failure, ok := errors.AsType[*Failure](err)
		if !ok || failure.Status != http.StatusBadRequest || failure.Code != "config_invalid" ||
			failure.Message != "Add a computer first" {
			t.Fatalf("turn execution off with no remote computer = %+v", failure)
		}
		if err := ValidateLocalExecutionChange(initial, next, 1); err != nil {
			t.Fatalf("turn execution off with an added computer: %v", err)
		}
	})
}
