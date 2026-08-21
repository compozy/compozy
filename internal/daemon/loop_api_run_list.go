package daemon

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
)

type loopRunListCursor struct {
	WorkspaceID   string    `json:"workspace_id"`
	LoopName      string    `json:"loop_name,omitempty"`
	Status        string    `json:"status,omitempty"`
	Origin        string    `json:"origin,omitempty"`
	OriginSession string    `json:"origin_session,omitempty"`
	Live          *bool     `json:"live,omitempty"`
	Rank          int       `json:"rank"`
	CreatedAt     time.Time `json:"created_at"`
	ID            string    `json:"id"`
}

func (s *daemonLoopAPIService) ListLoopRuns(
	ctx context.Context,
	workspaceID string,
	query core.LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	cursor, err := decodeLoopRunListCursor(query.Cursor)
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	if query.Limit < 0 || query.Limit > 500 {
		return contract.LoopRunsResponse{}, fmt.Errorf(
			"%w: limit must be 0 or between 1 and 500",
			looppkg.ErrValidation,
		)
	}
	limit := query.Limit
	if limit == 0 {
		limit = 100
	}
	if cursor != nil && !cursor.matches(string(ws), query) {
		return contract.LoopRunsResponse{}, fmt.Errorf(
			"%w: Loop run list cursor scope changed",
			looppkg.ErrInvalidRunListCursor,
		)
	}
	storeQuery := looppkg.RunListQuery{
		WorkspaceID:      ws,
		LoopName:         strings.TrimSpace(query.LoopName),
		Status:           looppkg.Status(strings.TrimSpace(query.Status)),
		OriginKind:       strings.TrimSpace(query.Origin),
		OriginSessionID:  strings.TrimSpace(query.OriginSession),
		Live:             query.Live,
		Limit:            limit + 1,
		OperationalOrder: true,
	}
	if cursor != nil {
		storeQuery.After = &looppkg.RunListPosition{
			Rank:      cursor.Rank,
			CreatedAt: cursor.CreatedAt,
			ID:        looppkg.RunID(cursor.ID),
		}
	}
	runs, err := s.persistence.ListLoopRuns(ctx, storeQuery)
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	summaries, err := s.loopRunListSummaries(ctx, ws, runs)
	if err != nil {
		return contract.LoopRunsResponse{}, err
	}
	payloads := make([]contract.LoopRunPayload, 0, len(runs))
	for _, run := range runs {
		if summary, ok := summaries[run.ID]; ok {
			run.SetForks(summary.Forks)
		}
		payload, payloadErr := loopRunPayload(run)
		if payloadErr != nil {
			return contract.LoopRunsResponse{}, payloadErr
		}
		applyLoopRunListSummary(&payload, summaries[run.ID])
		payloads = append(payloads, payload)
	}
	sortLoopRunList(payloads)
	response := contract.LoopRunsResponse{}
	if len(payloads) > limit {
		response.Runs = payloads[:limit]
		response.NextCursor, err = encodeLoopRunListCursor(
			response.Runs[len(response.Runs)-1],
			string(ws),
			query,
		)
		if err != nil {
			return contract.LoopRunsResponse{}, err
		}
	} else {
		response.Runs = payloads
	}
	response.Aggregates = loopRunsAggregate(runs[:len(response.Runs)])
	return response, nil
}

func (s *daemonLoopAPIService) loopRunListSummaries(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	runs []looppkg.Run,
) (map[looppkg.RunID]looppkg.RunListSummary, error) {
	reader, ok := s.persistence.(looppkg.RunListSummaryReader)
	if !ok {
		return map[looppkg.RunID]looppkg.RunListSummary{}, nil
	}
	runIDs := make([]looppkg.RunID, len(runs))
	for index := range runs {
		runIDs[index] = runs[index].ID
	}
	summaries, err := reader.ListLoopRunSummaries(ctx, workspaceID, runIDs)
	if err != nil {
		return nil, fmt.Errorf("read Loop run list summaries: %w", err)
	}
	return summaries, nil
}

func applyLoopRunListSummary(payload *contract.LoopRunPayload, summary looppkg.RunListSummary) {
	payload.Progress = contract.LoopRunProgress{
		Round:      summary.Progress.Round,
		StepsDone:  summary.Progress.StepsDone,
		StepsTotal: summary.Progress.StepsTotal,
	}
	if summary.Attention != nil {
		payload.Attention = &contract.LoopRunAttention{
			Kind:  summary.Attention.Kind,
			Count: summary.Attention.Count,
			Since: summary.Attention.Since,
		}
	}
}

func sortLoopRunList(runs []contract.LoopRunPayload) {
	sort.SliceStable(runs, func(i, j int) bool {
		left, right := loopRunListRank(runs[i]), loopRunListRank(runs[j])
		if left != right {
			return left < right
		}
		if !runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].CreatedAt.After(runs[j].CreatedAt)
		}
		return runs[i].ID > runs[j].ID
	})
}

func loopRunListRank(run contract.LoopRunPayload) int {
	if run.Attention != nil {
		return 0
	}
	switch run.Status {
	case contract.LoopRunStatusQueued, contract.LoopRunStatusRunning, contract.LoopRunStatusWatching,
		contract.LoopRunStatusNeedsApproval, contract.LoopRunStatusPaused:
		return 1
	default:
		return 2
	}
}

func encodeLoopRunListCursor(
	run contract.LoopRunPayload,
	workspaceID string,
	query core.LoopRunListQuery,
) (string, error) {
	raw, err := json.Marshal(loopRunListCursor{
		WorkspaceID:   strings.TrimSpace(workspaceID),
		LoopName:      strings.TrimSpace(query.LoopName),
		Status:        strings.TrimSpace(query.Status),
		Origin:        strings.TrimSpace(query.Origin),
		OriginSession: strings.TrimSpace(query.OriginSession),
		Live:          query.Live,
		Rank:          loopRunListRank(run),
		CreatedAt:     run.CreatedAt,
		ID:            run.ID,
	})
	if err != nil {
		return "", fmt.Errorf("encode Loop run list cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeLoopRunListCursor(value string) (*loopRunListCursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: malformed Loop run list cursor", looppkg.ErrInvalidRunListCursor)
	}
	var cursor loopRunListCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Rank < 0 || cursor.Rank > 2 ||
		cursor.CreatedAt.IsZero() || strings.TrimSpace(cursor.ID) == "" ||
		strings.TrimSpace(cursor.WorkspaceID) == "" {
		return nil, fmt.Errorf("%w: malformed Loop run list cursor", looppkg.ErrInvalidRunListCursor)
	}
	return &cursor, nil
}

func (c loopRunListCursor) matches(workspaceID string, query core.LoopRunListQuery) bool {
	return c.WorkspaceID == strings.TrimSpace(workspaceID) &&
		c.LoopName == strings.TrimSpace(query.LoopName) &&
		c.Status == strings.TrimSpace(query.Status) &&
		c.Origin == strings.TrimSpace(query.Origin) &&
		c.OriginSession == strings.TrimSpace(query.OriginSession) &&
		equalOptionalBool(c.Live, query.Live)
}

func equalOptionalBool(left, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
