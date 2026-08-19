package globaldb

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
)

var _ looppkg.RequestStore = (*LoopRepo)(nil)

type requestCursor struct {
	NullExpiry bool      `json:"null_expiry,omitempty"`
	Primary    time.Time `json:"primary"`
	Secondary  time.Time `json:"secondary,omitzero"`
	RowID      int64     `json:"row_id"`
}

type storedRequest struct {
	request       looppkg.Request
	rowID         int64
	contextRef    string
	answerSchema  json.RawMessage
	editSchema    json.RawMessage
	respondSchema json.RawMessage
	proposedRef   string
	answeredRef   string
	answeredNote  string
}

const requestSelectColumns = `request.rowid, request.loop_run_id, run.loop_name,
	request.generation, request.node_id, request.item_index, request.kind, request.state,
	request.prompt, request.context_preview_json, request.context_ref,
	request.answer_schema_json, request.edit_schema_json, request.respond_schema_json,
	request.decisions_json, request.proposed_ref, request.proposed_preview_json, request.answered_decision,
	request.answered_payload_ref, request.answered_note, request.actor_kind, request.actor_id,
	request.opened_at, request.resolved_at, request.expires_at`

// ListRequests returns a stable page without exposing private execution refs.
func (g *LoopRepo) ListRequests(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	query looppkg.RequestQuery,
) (page looppkg.RequestPage, err error) {
	if err := g.checkReady(ctx, "list Loop requests"); err != nil {
		return looppkg.RequestPage{}, err
	}
	workspaceID = looppkg.WorkspaceID(strings.TrimSpace(string(workspaceID)))
	if workspaceID == "" {
		return looppkg.RequestPage{}, fmt.Errorf("%w: workspace id is required", looppkg.ErrValidation)
	}
	state, limit, cursor, err := normalizeRequestQuery(query)
	if err != nil {
		return looppkg.RequestPage{}, err
	}
	rows, err := g.queryRequestRows(ctx, workspaceID, query.RunID, state, cursor, limit+1)
	if err != nil {
		return looppkg.RequestPage{}, fmt.Errorf("store: list Loop requests: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close Loop request rows: %w", closeErr))
		}
	}()

	page, err = scanRequestPage(rows, limit, state)
	if err != nil {
		return looppkg.RequestPage{}, err
	}
	if err := g.db.QueryRowContext(ctx, `SELECT count(*) FROM loop_requests
		WHERE workspace_id = ? AND state = 'pending'`, workspaceID).Scan(&page.Pending); err != nil {
		return looppkg.RequestPage{}, fmt.Errorf("store: count pending Loop requests: %w", err)
	}
	return page, nil
}

func normalizeRequestQuery(query looppkg.RequestQuery) (string, int, *requestCursor, error) {
	state := strings.TrimSpace(query.State)
	if state == "" {
		state = looppkg.RequestStatePending
	}
	if state != looppkg.RequestStatePending && state != "resolved" {
		return "", 0, nil, fmt.Errorf("%w: request state must be pending or resolved", looppkg.ErrValidation)
	}
	limit := query.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return "", 0, nil, fmt.Errorf("%w: request limit must be between 1 and 200", looppkg.ErrValidation)
	}
	cursor, err := decodeRequestCursor(query.Cursor)
	return state, limit, cursor, err
}

func scanRequestPage(rows *sql.Rows, limit int, state string) (looppkg.RequestPage, error) {
	items := make([]looppkg.Request, 0, limit)
	var lastIncluded storedRequest
	hasMore := false
	for rows.Next() {
		stored, err := scanStoredRequest(rows)
		if err != nil {
			return looppkg.RequestPage{}, err
		}
		if len(items) == limit {
			hasMore = true
			break
		}
		items = append(items, stored.request)
		lastIncluded = stored
	}
	if err := rows.Err(); err != nil {
		return looppkg.RequestPage{}, fmt.Errorf("store: iterate Loop requests: %w", err)
	}
	page := looppkg.RequestPage{Items: items}
	if hasMore {
		cursor, err := encodeRequestCursor(cursorForRequest(lastIncluded, state))
		if err != nil {
			return looppkg.RequestPage{}, err
		}
		page.NextCursor = cursor
	}
	return page, nil
}

// GetRequest returns one request, optionally hydrating its full redacted context.
func (g *LoopRepo) GetRequest(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	ref looppkg.RequestRef,
	full bool,
) (looppkg.Request, error) {
	if err := g.checkReady(ctx, "get Loop request"); err != nil {
		return looppkg.Request{}, err
	}
	stored, err := getStoredRequest(ctx, g.db, workspaceID, ref)
	if err != nil {
		return looppkg.Request{}, err
	}
	if full && stored.contextRef != "" {
		contextPayload, loadErr := getLoopOutputByRefWithExecutor(ctx, g.db, stored.contextRef)
		if loadErr != nil {
			return looppkg.Request{}, fmt.Errorf("store: load Loop request context: %w", loadErr)
		}
		stored.request.Context = contextPayload
	}
	return stored.request, nil
}

// RespondRequest atomically claims a request and its parked cell, then reserves one coordinator wake.
func (g *LoopRepo) RespondRequest(
	ctx context.Context,
	input looppkg.RespondInput,
) (looppkg.RespondResult, error) {
	if err := g.checkReady(ctx, "respond to Loop request"); err != nil {
		return looppkg.RespondResult{}, err
	}
	var result looppkg.RespondResult
	err := g.withTaskImmediateTransaction(ctx, "respond to Loop request", func(exec taskSQLExecutor) error {
		return g.respondRequestWithExecutor(ctx, exec, input, &result)
	})
	if err != nil {
		return looppkg.RespondResult{}, err
	}
	return result, nil
}

func getStoredRequest(
	ctx context.Context,
	exec interface {
		QueryRowContext(context.Context, string, ...any) *sql.Row
	},
	workspaceID looppkg.WorkspaceID,
	ref looppkg.RequestRef,
) (storedRequest, error) {
	where := `request.workspace_id = ? AND request.loop_run_id = ? AND request.node_id = ? AND request.item_index = ?`
	args := []any{workspaceID, ref.RunID, ref.NodeID, ref.ItemIndex}
	if ref.Generation > 0 {
		where += ` AND request.generation = ?`
		args = append(args, ref.Generation)
	}
	row := exec.QueryRowContext(ctx, `SELECT `+requestSelectColumns+`
		FROM loop_requests AS request JOIN loop_runs AS run ON run.id = request.loop_run_id
		WHERE `+where+` ORDER BY request.generation DESC LIMIT 1`, args...)
	stored, err := scanStoredRequest(row)
	if errors.Is(err, sql.ErrNoRows) {
		return storedRequest{}, looppkg.NewRequestReasonError(
			looppkg.ReasonCodeRequestNotFound,
			fmt.Errorf("%w: request %s/%d not found", looppkg.ErrRequestNotFound, ref.NodeID, ref.ItemIndex),
			nil,
		)
	}
	return stored, err
}

func scanStoredRequest(scanner rowScanner) (storedRequest, error) {
	var stored storedRequest
	var contextPreview, decisions string
	var contextRef, answerSchema, editSchema, respondSchema, proposedRef, proposedPreview sql.NullString
	var answeredDecision, answeredRef, answeredNote sql.NullString
	var actorKind, actorID sql.NullString
	var resolvedAt, expiresAt sql.NullTime
	err := scanner.Scan(&stored.rowID, &stored.request.LoopRunID, &stored.request.LoopName,
		&stored.request.Generation, &stored.request.NodeID, &stored.request.ItemIndex,
		&stored.request.Kind, &stored.request.State, &stored.request.Prompt, &contextPreview,
		&contextRef, &answerSchema, &editSchema, &respondSchema, &decisions, &proposedRef,
		&proposedPreview, &answeredDecision, &answeredRef, &answeredNote,
		&actorKind, &actorID, &stored.request.OpenedAt, &resolvedAt, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedRequest{}, err
		}
		return storedRequest{}, fmt.Errorf("store: scan Loop request: %w", err)
	}
	stored.request.Context = json.RawMessage(contextPreview)
	if answerSchema.Valid {
		stored.answerSchema = json.RawMessage(answerSchema.String)
		stored.request.Expect = append(json.RawMessage(nil), stored.answerSchema...)
	}
	if editSchema.Valid {
		stored.editSchema = json.RawMessage(editSchema.String)
		stored.request.EditSchema = append(json.RawMessage(nil), stored.editSchema...)
	}
	if respondSchema.Valid {
		stored.respondSchema = json.RawMessage(respondSchema.String)
		stored.request.RespondSchema = append(json.RawMessage(nil), stored.respondSchema...)
	}
	if proposedPreview.Valid {
		stored.request.ProposedPreview = json.RawMessage(proposedPreview.String)
	}
	if err := json.Unmarshal([]byte(decisions), &stored.request.Decisions); err != nil {
		return storedRequest{}, fmt.Errorf("store: decode Loop request decisions: %w", err)
	}
	stored.contextRef = contextRef.String
	stored.proposedRef = proposedRef.String
	stored.answeredRef = answeredRef.String
	stored.answeredNote = answeredNote.String
	stored.request.AnsweredDecision = answeredDecision.String
	stored.request.ActorKind = actorKind.String
	stored.request.ActorID = actorID.String
	stored.request.ResolvedAt = loopTimePointer(resolvedAt)
	stored.request.ExpiresAt = loopTimePointer(expiresAt)
	stored.request.OpenedAt = stored.request.OpenedAt.UTC()
	return stored, nil
}

func requestAllowsDecision(decisions []string, decision string) bool {
	for _, allowed := range decisions {
		if strings.TrimSpace(allowed) == decision {
			return true
		}
	}
	return false
}

func resolvedRequestOutcome(
	ctx context.Context,
	exec taskSQLExecutor,
	stored storedRequest,
	decision string,
	payload json.RawMessage,
	actorKind string,
	actorID string,
	result *looppkg.RespondResult,
) error {
	switch stored.request.State {
	case looppkg.RequestStateAnswered:
		if stored.request.AnsweredDecision == decision && stored.request.ActorKind == actorKind &&
			stored.request.ActorID == actorID && stored.answeredRef != "" {
			persisted, err := getLoopOutputByRefWithExecutor(ctx, exec, stored.answeredRef)
			if err != nil {
				return err
			}
			if string(persisted) == string(payload) {
				*result = looppkg.RespondResult{Request: stored.request, Won: false}
				return nil
			}
		}
		return looppkg.NewRequestReasonError(
			looppkg.ReasonCodeRequestAlreadyAnswered,
			fmt.Errorf("%w: recorded decision is %q", looppkg.ErrRequestAlreadyAnswered,
				stored.request.AnsweredDecision),
			map[string]string{looppkg.ReasonMetaRecordedDecision: stored.request.AnsweredDecision},
		)
	case looppkg.RequestStateExpired:
		return looppkg.NewRequestReasonError(looppkg.ReasonCodeRequestExpired, looppkg.ErrRequestExpired, nil)
	case looppkg.RequestStateCanceled:
		return looppkg.NewRequestReasonError(looppkg.ReasonCodeRequestCanceled, looppkg.ErrRequestCanceled, nil)
	default:
		return fmt.Errorf("%w: invalid request state %q", looppkg.ErrValidation, stored.request.State)
	}
}

func requestResolutionEventPayload(stored storedRequest, decision, actorKind, actorID string) map[string]any {
	return map[string]any{
		loopRunEventPayloadKeyGeneration: stored.request.Generation,
		loopRunEventPayloadKeyNodeID:     stored.request.NodeID,
		loopRunEventPayloadKeyItemIndex:  stored.request.ItemIndex,
		"decision":                       decision,
		loopRunEventPayloadKeyActorKind:  actorKind,
		loopRunEventPayloadKeyActorID:    actorID,
	}
}

func encodeRequestCursor(cursor requestCursor) (string, error) {
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("store: encode Loop request cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func decodeRequestCursor(encoded string) (*requestCursor, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: request cursor is invalid", looppkg.ErrValidation)
	}
	var cursor requestCursor
	if err := json.Unmarshal(raw, &cursor); err != nil || cursor.RowID < 1 || cursor.Primary.IsZero() {
		return nil, fmt.Errorf("%w: request cursor is invalid", looppkg.ErrValidation)
	}
	return &cursor, nil
}

func cursorForRequest(stored storedRequest, state string) requestCursor {
	if state == looppkg.RequestStatePending {
		cursor := requestCursor{Secondary: stored.request.OpenedAt, RowID: stored.rowID}
		if stored.request.ExpiresAt == nil {
			cursor.NullExpiry = true
			cursor.Primary = stored.request.OpenedAt
		} else {
			cursor.Primary = *stored.request.ExpiresAt
		}
		return cursor
	}
	return requestCursor{Primary: *stored.request.ResolvedAt, RowID: stored.rowID}
}

func nullableCursorTime(cursor requestCursor) any {
	if cursor.NullExpiry {
		return ""
	}
	return cursor.Primary
}
