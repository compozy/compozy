package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const sessionStateOrphaned = "orphaned"

const (
	globalDBSessionLocalKey   = "local"
	sessionListSortActivity   = "last_activity"
	sessionListResumableWhere = "state = 'active' AND (failure_kind IS NULL OR trim(failure_kind) = '') AND " +
		"(stall_state IS NULL OR trim(stall_state) = '') AND " +
		"(attached_to = '' OR attach_expires_at IS NULL OR attach_expires_at <= ?)"
)

// RegisterSession inserts or refreshes a session index row.
func (g *SessionRepo) RegisterSession(ctx context.Context, session store.SessionInfo) error {
	if err := g.checkReady(ctx, "register session"); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return err
	}

	normalized := session
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	if err := g.registerSession(ctx, g.db, normalized); err != nil {
		return fmt.Errorf("store: register session %q: %w", normalized.ID, err)
	}
	return nil
}

// UpdateSessionState updates the mutable session state fields.
func (g *SessionRepo) UpdateSessionState(ctx context.Context, update store.SessionStateUpdate) error {
	if err := g.checkReady(ctx, "update session state"); err != nil {
		return err
	}
	if err := update.Validate(); err != nil {
		return err
	}

	updatedAt := update.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = g.now()
	}

	query, args, err := buildUpdateSessionStateStatement(update, updatedAt)
	if err != nil {
		return fmt.Errorf("store: build update session state %q: %w", update.ID, err)
	}
	// dynamic-sql: the mutable session-state field set is explicitly partial and alters assignments.
	result, err := g.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("store: update session state %q: %w", update.ID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store: rows affected for session state %q: %w", update.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("store: session %q not found", update.ID)
	}
	return nil
}

// ListSessions returns indexed sessions ordered by most recent update.
func (g *SessionRepo) ListSessions(
	ctx context.Context,
	query store.SessionListQuery,
) (sessions []store.SessionInfo, err error) {
	if err := g.checkReady(ctx, "list sessions"); err != nil {
		return nil, err
	}
	if err := query.Validate(); err != nil {
		return nil, err
	}
	now := g.now()
	if _, err := g.SweepExpiredSessionAttachLocks(ctx, now); err != nil {
		return nil, err
	}

	sqlQuery := sessionInfoSelectQuery
	where, args := store.BuildClauses(
		store.StringClause("id", query.ID),
		store.StringClause("state", query.State),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("workspace_id", query.WorkspaceID),
		store.StringClause("session_type", query.SessionType),
		store.StringClause("parent_session_id", query.ParentSessionID),
		store.StringClause("root_session_id", query.RootSessionID),
		store.StringClause("spawn_role", query.SpawnRole),
	)
	if query.Resumable {
		where = append(
			where,
			sessionListResumableWhere,
		)
		args = append(args, store.FormatTimestamp(now))
	}
	sqlQuery = store.AppendWhere(sqlQuery, where)
	sqlQuery += sessionListOrderClause(query.Sort)
	sqlQuery, args = store.AppendLimit(sqlQuery, args, query.Limit)

	// dynamic-sql: session catalog filters, resumability, ordering, and limit alter statement structure.
	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query sessions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("store: close session rows: %w", closeErr))
		}
	}()

	sessions = make([]store.SessionInfo, 0)
	for rows.Next() {
		session, scanErr := scanSessionInfo(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate sessions: %w", err)
	}

	return sessions, nil
}

// SweepExpiredSessionAttachLocks clears expired attach leases from the session catalog.
func (g *SessionRepo) SweepExpiredSessionAttachLocks(ctx context.Context, now time.Time) (int64, error) {
	if err := g.checkReady(ctx, "sweep expired session attach locks"); err != nil {
		return 0, err
	}
	if now.IsZero() {
		now = g.now()
	}
	now = now.UTC()
	nowRaw := store.FormatTimestamp(now)
	expiredLocks, err := g.queries.CountExpiredSessionAttachLocks(ctx, nullableSessionTime(now))
	if err != nil {
		return 0, fmt.Errorf("store: count expired session attach locks: %w", err)
	}
	if expiredLocks == 0 {
		return 0, nil
	}
	rows, err := g.queries.SweepExpiredSessionAttachLocks(ctx, nowRaw)
	if err != nil {
		return 0, fmt.Errorf("store: sweep expired session attach locks: %w", err)
	}
	return rows, nil
}

// AttachSession acquires a short-lived attach lease for a resumable session.
func (g *SessionRepo) AttachSession(ctx context.Context, req store.SessionAttachRequest) (store.SessionAttach, error) {
	if err := g.checkReady(ctx, "attach session"); err != nil {
		return store.SessionAttach{}, err
	}
	normalized := req.Normalize()
	if normalized.Now.IsZero() {
		normalized.Now = g.now().UTC()
	}
	if err := normalized.Validate(); err != nil {
		return store.SessionAttach{}, err
	}
	if _, err := g.SweepExpiredSessionAttachLocks(ctx, normalized.Now); err != nil {
		return store.SessionAttach{}, err
	}
	expiresAt := normalized.Now.Add(normalized.TTL).UTC()
	nowRaw := store.FormatTimestamp(normalized.Now)

	affected, err := g.queries.AttachSession(ctx, sqlcgen.AttachSessionParams{
		AttachedTo: normalized.AttachedTo, AttachExpiresAt: nullableSessionTime(expiresAt),
		UpdatedAt: nowRaw, ID: normalized.SessionID,
	})
	if err != nil {
		return store.SessionAttach{}, fmt.Errorf("store: attach session %q: %w", normalized.SessionID, err)
	}
	if affected == 0 {
		if classifyErr := g.classifyAttachFailure(ctx, normalized.SessionID, normalized.Now); classifyErr != nil {
			return store.SessionAttach{}, classifyErr
		}
		return store.SessionAttach{}, fmt.Errorf("%w: %s", store.ErrSessionAttachLocked, normalized.SessionID)
	}
	return store.SessionAttach{
		SessionID:       normalized.SessionID,
		AttachedTo:      normalized.AttachedTo,
		AttachExpiresAt: expiresAt,
		AttachedAt:      normalized.Now,
	}, nil
}

func (g *SessionRepo) classifyAttachFailure(ctx context.Context, sessionID string, now time.Time) error {
	row, err := g.queries.GetSessionAttachState(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
		}
		return fmt.Errorf("store: classify attach session %q: %w", sessionID, err)
	}
	if strings.TrimSpace(row.State) != globalDBSessionStateActive ||
		strings.TrimSpace(row.FailureKind.String) != "" ||
		strings.TrimSpace(row.StallState) != "" {
		return fmt.Errorf("%w: %s", store.ErrSessionNotAttachable, sessionID)
	}
	if strings.TrimSpace(row.AttachedTo) == "" ||
		!row.AttachExpiresAt.Valid ||
		strings.TrimSpace(row.AttachExpiresAt.String) == "" {
		return nil
	}
	expiresAt, err := store.ParseTimestamp(row.AttachExpiresAt.String)
	if err != nil {
		return fmt.Errorf("store: parse session attach expiry for %q: %w", sessionID, err)
	}
	if expiresAt.After(now.UTC()) {
		return fmt.Errorf("%w: %s", store.ErrSessionAttachLocked, sessionID)
	}
	return nil
}

func sessionListOrderClause(sortKey string) string {
	switch strings.TrimSpace(sortKey) {
	case sessionListSortActivity:
		return " ORDER BY COALESCE(last_update_at, updated_at) DESC, updated_at DESC, id DESC"
	default:
		return " ORDER BY updated_at DESC, created_at DESC, id DESC"
	}
}

// ReconcileSessions upserts on-disk sessions and marks missing ones as orphaned.
func (g *SessionRepo) ReconcileSessions(
	ctx context.Context,
	sessions []store.SessionInfo,
) (store.ReconcileResult, error) {
	if err := g.checkReady(ctx, "reconcile sessions"); err != nil {
		return store.ReconcileResult{}, err
	}

	var result store.ReconcileResult
	err := g.withImmediateTransaction(ctx, "reconcile sessions", func(exec globalSQLExecutor) error {
		existing, err := g.loadSessionIDs(ctx, exec)
		if err != nil {
			return err
		}

		attemptResult := store.ReconcileResult{
			Indexed:  make([]string, 0),
			Orphaned: make([]string, 0),
		}
		seen := make(map[string]struct{}, len(sessions))

		for _, session := range sessions {
			if err := session.Validate(); err != nil {
				return err
			}
			normalized := session
			if normalized.CreatedAt.IsZero() {
				normalized.CreatedAt = g.now()
			}
			if normalized.UpdatedAt.IsZero() {
				normalized.UpdatedAt = normalized.CreatedAt
			}
			if _, ok := seen[normalized.ID]; ok {
				continue
			}
			seen[normalized.ID] = struct{}{}
			if _, ok := existing[normalized.ID]; !ok {
				attemptResult.Indexed = append(attemptResult.Indexed, normalized.ID)
			}
			if err := g.registerSession(ctx, exec, normalized); err != nil {
				return fmt.Errorf("store: reconcile session %q: %w", normalized.ID, err)
			}
		}

		orphanedAt := store.FormatTimestamp(g.now())
		queries := sqlcgen.New(exec)
		for id := range existing {
			if _, ok := seen[id]; ok {
				continue
			}
			if err := queries.MarkSessionOrphaned(ctx, sqlcgen.MarkSessionOrphanedParams{
				State: sessionStateOrphaned, UpdatedAt: orphanedAt, ID: id,
			}); err != nil {
				return fmt.Errorf("store: mark orphaned session %q: %w", id, err)
			}
			attemptResult.Orphaned = append(attemptResult.Orphaned, id)
		}

		result = attemptResult
		return nil
	})
	if err != nil {
		return store.ReconcileResult{}, err
	}
	return result, nil
}

func (g *SessionRepo) registerSession(ctx context.Context, exec globalSQLExecutor, session store.SessionInfo) error {
	record, err := newSessionCatalogRecord(session)
	if err != nil {
		return err
	}
	params, err := upsertSessionParams(record)
	if err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).UpsertSession(ctx, params)
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("%w: %s", store.ErrSessionParticipationMismatch, session.ID)
	}
	return nil
}

type sessionCatalogRecord struct {
	session              store.SessionInfo
	lineage              *store.SessionLineage
	spawnBudgetJSON      string
	permissionPolicyJSON string
	activityJSON         string
}

func newSessionCatalogRecord(session store.SessionInfo) (sessionCatalogRecord, error) {
	lineage := store.NormalizeSessionLineage(session.ID, session.Lineage)
	if err := store.ValidateSessionLineage(session.ID, lineage); err != nil {
		return sessionCatalogRecord{}, err
	}
	spawnBudgetJSON, err := store.EncodeSessionSpawnBudget(lineage.SpawnBudget)
	if err != nil {
		return sessionCatalogRecord{}, err
	}
	permissionPolicyJSON, err := store.EncodeSessionPermissionPolicy(lineage.PermissionPolicy)
	if err != nil {
		return sessionCatalogRecord{}, err
	}
	activityJSON, err := sessionLivenessActivityJSON(session.Liveness)
	if err != nil {
		return sessionCatalogRecord{}, err
	}
	return sessionCatalogRecord{
		session:              session,
		lineage:              lineage,
		spawnBudgetJSON:      spawnBudgetJSON,
		permissionPolicyJSON: permissionPolicyJSON,
		activityJSON:         activityJSON,
	}, nil
}

func sessionFailureKind(failure *store.SessionFailure) any {
	if failure == nil {
		return nil
	}
	normalized := failure.Normalize()
	return store.NullableString(string(normalized.Kind))
}

func sessionFailureSummary(failure *store.SessionFailure) string {
	if failure == nil {
		return ""
	}
	return failure.Normalize().Summary
}

func sessionCrashBundlePath(failure *store.SessionFailure) string {
	if failure == nil {
		return ""
	}
	return failure.Normalize().CrashBundlePath
}

func buildUpdateSessionStateStatement(update store.SessionStateUpdate, updatedAt time.Time) (string, []any, error) {
	assignments := []string{"state = ?"}
	args := []any{update.State}

	if update.ACPSessionID != nil {
		assignments = append(assignments, "acp_session_id = ?")
		args = append(args, store.NullableStringPointer(update.ACPSessionID))
	}
	if update.StopReasonSet {
		assignments = append(assignments, "stop_reason = ?", "stop_detail = ?")
		args = append(args, store.NullableStringPointer(update.StopReason), store.NullableString(update.StopDetail))
	}
	if update.FailureSet {
		assignments = append(assignments, "failure_kind = ?", "failure_summary = ?", "crash_bundle_path = ?")
		args = append(
			args,
			sessionFailureKind(update.Failure),
			sessionFailureSummary(update.Failure),
			sessionCrashBundlePath(update.Failure),
		)
	}
	if update.Liveness != nil {
		activityJSON, err := sessionLivenessActivityJSON(update.Liveness)
		if err != nil {
			return "", nil, err
		}
		assignments = append(
			assignments,
			"subprocess_pid = ?",
			"subprocess_started_at = ?",
			"last_update_at = ?",
			"stall_state = ?",
			"stall_reason = ?",
			"activity_json = ?",
		)
		args = append(
			args,
			sessionLivenessPID(update.Liveness),
			sessionLivenessStartedAt(update.Liveness),
			sessionLivenessLastUpdateAt(update.Liveness),
			sessionLivenessStallState(update.Liveness),
			sessionLivenessStallReason(update.Liveness),
			activityJSON,
		)
	}
	if update.Sandbox != nil {
		assignments = append(
			assignments,
			"sandbox_id = ?",
			"sandbox_backend = ?",
			"sandbox_profile = ?",
			"sandbox_instance_id = ?",
			"sandbox_state = ?",
			"sandbox_provider_state_json = ?",
			"sandbox_last_sync_at = ?",
			"sandbox_last_sync_error = ?",
		)
		args = append(
			args,
			sessionSandboxID(update.Sandbox),
			sessionSandboxBackend(update.Sandbox),
			sessionSandboxProfile(update.Sandbox),
			sessionSandboxInstanceID(update.Sandbox),
			sessionSandboxState(update.Sandbox),
			sessionSandboxProviderStateJSON(update.Sandbox),
			sessionSandboxLastSyncAt(update.Sandbox),
			sessionSandboxLastSyncError(update.Sandbox),
		)
	}
	if strings.TrimSpace(update.State) == globalDBSessionStateStopped {
		assignments = append(assignments, "attached_to = ''", "attach_expires_at = NULL")
	}

	assignments = append(assignments, "updated_at = ?")
	args = append(args, store.FormatTimestamp(updatedAt), update.ID)
	return fmt.Sprintf("UPDATE sessions SET %s WHERE id = ?", strings.Join(assignments, ", ")), args, nil
}
