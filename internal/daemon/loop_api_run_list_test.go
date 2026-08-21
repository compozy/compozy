package daemon

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestLoopRunListOrderingAndCursorContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	runs := []contract.LoopRunPayload{
		{ID: "terminal-new", Status: contract.LoopRunStatusDone, CreatedAt: now.Add(3 * time.Minute)},
		{ID: "active-new", Status: contract.LoopRunStatusRunning, CreatedAt: now.Add(2 * time.Minute)},
		{
			ID: "needs-you", Status: contract.LoopRunStatusRunning, CreatedAt: now,
			Attention: &contract.LoopRunAttention{Kind: "request", Count: 1, Since: now},
		},
		{ID: "active-old", Status: contract.LoopRunStatusQueued, CreatedAt: now.Add(time.Minute)},
		{ID: "terminal-old", Status: contract.LoopRunStatusFailed, CreatedAt: now},
	}
	sortLoopRunList(runs)
	want := []string{"needs-you", "active-new", "active-old", "terminal-new", "terminal-old"}
	for index, run := range runs {
		if run.ID != want[index] {
			t.Fatalf("ordered run %d = %q, want %q", index, run.ID, want[index])
		}
	}

	query := core.LoopRunListQuery{LoopName: "delivery"}
	cursorValue, err := encodeLoopRunListCursor(runs[1], "ws-a", query)
	if err != nil {
		t.Fatalf("encodeLoopRunListCursor() error = %v", err)
	}
	cursor, err := decodeLoopRunListCursor(cursorValue)
	if err != nil {
		t.Fatalf("decodeLoopRunListCursor() error = %v", err)
	}
	if cursor.Rank != 1 || cursor.ID != "active-new" || !cursor.CreatedAt.Equal(runs[1].CreatedAt) {
		t.Fatalf("decoded cursor = %#v", cursor)
	}
	if !cursor.matches("ws-a", query) || cursor.matches("ws-b", query) ||
		cursor.matches("ws-a", core.LoopRunListQuery{LoopName: "other"}) {
		t.Fatalf("cursor scope = %#v", cursor)
	}
	if _, err := decodeLoopRunListCursor("not-base64"); !errors.Is(err, looppkg.ErrInvalidRunListCursor) {
		t.Fatalf("decodeLoopRunListCursor(malformed) error = %v", err)
	}

	payload := contract.LoopRunPayload{}
	applyLoopRunListSummary(&payload, looppkg.RunListSummary{
		Progress:  looppkg.StepProgress{Round: 2, StepsDone: 3, StepsTotal: 5},
		Attention: &looppkg.RunListAttention{Kind: "approval", Count: 2, Since: now},
	})
	if payload.Progress.Round != 2 || payload.Progress.StepsDone != 3 ||
		payload.Progress.StepsTotal != 5 || payload.Attention == nil ||
		payload.Attention.Kind != "approval" || payload.Attention.Count != 2 {
		t.Fatalf("summary payload = %#v", payload)
	}
}

func TestLoopRunListShouldPreservePublicLimitFiveHundred(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC)
	runs := make([]looppkg.Run, 501)
	for index := range runs {
		runs[index] = looppkg.Run{
			ID:             looppkg.RunID(fmt.Sprintf("run-%03d", index)),
			WorkspaceID:    "ws-a",
			LoopName:       "delivery",
			Status:         looppkg.StatusRunning,
			CreatedAt:      now.Add(-time.Duration(index) * time.Second),
			StartedAt:      now,
			LastProgressAt: now,
		}
	}
	persistence := &loopRunListPersistenceStub{runs: runs}
	service := &daemonLoopAPIService{persistence: persistence}

	response, err := service.ListLoopRuns(t.Context(), "ws-a", core.LoopRunListQuery{Limit: 500})
	if err != nil {
		t.Fatalf("ListLoopRuns() error = %v", err)
	}
	if persistence.query.Limit != 501 {
		t.Fatalf("store limit = %d, want 501", persistence.query.Limit)
	}
	if len(response.Runs) != 500 || response.NextCursor == "" {
		t.Fatalf("response rows/cursor = %d/%q, want 500/nonempty", len(response.Runs), response.NextCursor)
	}
	_, err = service.ListLoopRuns(t.Context(), "ws-a", core.LoopRunListQuery{Limit: 501})
	if !errors.Is(err, looppkg.ErrValidation) {
		t.Fatalf("ListLoopRuns(limit 501) error = %v, want validation", err)
	}
}

type loopRunListPersistenceStub struct {
	loopAPIPersistence
	runs  []looppkg.Run
	query looppkg.RunListQuery
}

func (s *loopRunListPersistenceStub) ListLoopRuns(
	_ context.Context,
	query looppkg.RunListQuery,
) ([]looppkg.Run, error) {
	s.query = query
	if query.Limit > len(s.runs) {
		return nil, fmt.Errorf("requested %d rows from %d fixtures", query.Limit, len(s.runs))
	}
	return append([]looppkg.Run(nil), s.runs[:query.Limit]...), nil
}
