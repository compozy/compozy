package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	storepkg "github.com/compozy/agh/internal/store"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

// ReplayResult reports boot-time recovery work applied from memory_decisions.
type ReplayResult struct {
	Applied   int
	Stamped   int
	Reindexed int
}

type replayDecision struct {
	ID              string
	WorkspaceID     string
	Scope           memcontract.Scope
	AgentName       string
	AgentTier       memcontract.AgentTier
	Op              memcontract.Op
	TargetFilename  string
	PostContent     string
	PostContentHash string
}

// ReplayPendingDecisions applies unapplied memory_decisions rows idempotently.
func (s *Store) ReplayPendingDecisions(ctx context.Context) (ReplayResult, error) {
	if ctx == nil {
		return ReplayResult{}, errors.New("memory: replay context is required")
	}
	if s == nil || s.catalog == nil {
		return ReplayResult{}, nil
	}
	unlock := s.lockControllerDecisions()
	defer unlock()
	db, err := s.catalog.ensureDB(ctx)
	if err != nil {
		return ReplayResult{}, err
	}
	if db == nil {
		return ReplayResult{}, nil
	}

	decisions, err := pendingReplayDecisions(ctx, db)
	if err != nil {
		return ReplayResult{}, err
	}

	var result ReplayResult
	for _, decision := range decisions {
		target, err := s.storeForReplayDecision(ctx, decision)
		if err != nil {
			return ReplayResult{}, err
		}
		applied, reindexed, err := target.applyReplayDecision(ctx, decision)
		if err != nil {
			return ReplayResult{}, err
		}
		if err := markReplayDecisionApplied(ctx, db, decision.ID); err != nil {
			return ReplayResult{}, err
		}
		if applied {
			result.Applied++
		} else {
			result.Stamped++
		}
		result.Reindexed += reindexed
	}
	return result, nil
}

func pendingReplayDecisions(ctx context.Context, db *sql.DB) (decisions []replayDecision, err error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT id, workspace_id, scope, agent_name, agent_tier, op, target_filename,
			post_content, post_content_hash
		 FROM memory_decisions
		 WHERE applied_at IS NULL
		 ORDER BY decided_at ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("memory: query pending replay decisions: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("memory: close pending replay decision rows: %w", closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()

	decisions = make([]replayDecision, 0)
	for rows.Next() {
		decision, scanErr := scanReplayDecision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate pending replay decisions: %w", err)
	}
	return decisions, nil
}

func scanReplayDecision(scanner interface{ Scan(dest ...any) error }) (replayDecision, error) {
	var (
		decision        replayDecision
		workspaceID     sql.NullString
		scopeRaw        string
		agentName       sql.NullString
		agentTierRaw    sql.NullString
		opRaw           string
		postContent     sql.NullString
		postContentHash sql.NullString
	)
	if err := scanner.Scan(
		&decision.ID,
		&workspaceID,
		&scopeRaw,
		&agentName,
		&agentTierRaw,
		&opRaw,
		&decision.TargetFilename,
		&postContent,
		&postContentHash,
	); err != nil {
		return replayDecision{}, fmt.Errorf("memory: scan replay decision: %w", err)
	}
	decision.WorkspaceID = nullableSQLString(workspaceID)
	decision.Scope = memcontract.Scope(scopeRaw).Normalize()
	if err := decision.Scope.Validate(); err != nil {
		return replayDecision{}, fmt.Errorf("memory: replay decision %q scope: %w", decision.ID, err)
	}
	decision.AgentName = nullableSQLString(agentName)
	decision.AgentTier = memcontract.AgentTier(nullableSQLString(agentTierRaw)).Normalize()
	op, err := replayOp(opRaw)
	if err != nil {
		return replayDecision{}, fmt.Errorf("memory: replay decision %q op: %w", decision.ID, err)
	}
	decision.Op = op
	decision.PostContent = nullableSQLStringRaw(postContent)
	decision.PostContentHash = nullableSQLString(postContentHash)
	return decision, nil
}

func (s *Store) storeForReplayDecision(ctx context.Context, decision replayDecision) (*Store, error) {
	switch decision.Scope.Normalize() {
	case memcontract.ScopeGlobal:
		return s, nil
	case memcontract.ScopeWorkspace:
		if err := s.validateReplayWorkspace(ctx, decision.WorkspaceID); err != nil {
			return nil, err
		}
		return s, nil
	case memcontract.ScopeAgent:
		tier := decision.AgentTier.Normalize()
		if err := tier.Validate(); err != nil {
			return nil, fmt.Errorf("memory: replay decision %q agent tier: %w", decision.ID, err)
		}
		if tier == memcontract.AgentTierWorkspace {
			if err := s.validateReplayWorkspace(ctx, decision.WorkspaceID); err != nil {
				return nil, err
			}
		}
		return s.ForAgent(decision.WorkspaceID, decision.AgentName, tier), nil
	default:
		return nil, fmt.Errorf("memory: unsupported replay scope %q", decision.Scope)
	}
}

func (s *Store) validateReplayWorkspace(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(s.workspaceRoot) == "" {
		return errors.New("memory: replay workspace decision requires a workspace-bound store")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("memory: replay workspace decision missing workspace_id")
	}
	if !aghworkspace.IsWorkspaceID(workspaceID) {
		return fmt.Errorf("memory: replay workspace decision has invalid workspace_id %q", workspaceID)
	}
	actual, err := s.workspaceIDForRoot(ctx, s.workspaceRoot)
	if err != nil {
		return err
	}
	if actual != strings.TrimSpace(workspaceID) {
		return fmt.Errorf("memory: replay workspace_id %q does not match bound workspace %q", workspaceID, actual)
	}
	return nil
}

func (s *Store) applyReplayDecision(ctx context.Context, decision replayDecision) (bool, int, error) {
	switch decision.Op {
	case memcontract.OpNoop, memcontract.OpReject:
		return false, 0, nil
	case memcontract.OpAdd, memcontract.OpUpdate:
		if strings.TrimSpace(decision.PostContent) == "" {
			return false, 0, fmt.Errorf("memory: replay decision %q missing post_content", decision.ID)
		}
		if strings.TrimSpace(decision.PostContentHash) == "" {
			decision.PostContentHash = hashMemoryContent([]byte(decision.PostContent))
		}
		matches, err := s.replayTargetMatchesHash(decision)
		if err != nil {
			return false, 0, err
		}
		if matches {
			indexed, reindexErr := s.reindexReplayScope(ctx, decision.Scope)
			return false, indexed, reindexErr
		}
		if err := s.writeRaw(
			ctx,
			decision.Scope,
			decision.TargetFilename,
			[]byte(decision.PostContent),
			false,
		); err != nil {
			return false, 0, err
		}
		indexed, err := s.reindexReplayScope(ctx, decision.Scope)
		return true, indexed, err
	case memcontract.OpDelete:
		err := s.deleteRaw(ctx, decision.Scope, decision.TargetFilename, false)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, 0, err
		}
		indexed, reindexErr := s.reindexReplayScope(ctx, decision.Scope)
		return err == nil, indexed, reindexErr
	default:
		return false, 0, fmt.Errorf("memory: unsupported replay op %q", decision.Op.String())
	}
}

func (s *Store) replayTargetMatchesHash(decision replayDecision) (bool, error) {
	path, err := s.pathFor(decision.Scope, decision.TargetFilename)
	if err != nil {
		return false, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("memory: read replay target %q: %w", path, err)
	}
	return hashMemoryContent(content) == strings.TrimSpace(decision.PostContentHash), nil
}

func (s *Store) reindexReplayScope(
	ctx context.Context,
	scope memcontract.Scope,
) (int, error) {
	result, err := s.Reindex(ctx, memcontract.ReindexOptions{Scope: scope.Normalize()})
	if err != nil {
		return 0, err
	}
	return result.IndexedFiles, nil
}

func markReplayDecisionApplied(ctx context.Context, db *sql.DB, id string) error {
	return storepkg.ExecuteWrite(ctx, db, func(ctx context.Context, tx *storepkg.WriteTx) error {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE memory_decisions
			 SET applied_at = ?
			 WHERE id = ? AND applied_at IS NULL`,
			timeToUnixMillis(time.Now().UTC()),
			strings.TrimSpace(id),
		)
		if err != nil {
			return fmt.Errorf("memory: mark replay decision %q applied: %w", id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("memory: inspect replay decision %q update: %w", id, err)
		}
		if affected == 0 {
			return fmt.Errorf("memory: replay decision %q was already applied", id)
		}
		return nil
	})
}

func replayOp(value string) (memcontract.Op, error) {
	switch strings.TrimSpace(value) {
	case memcontract.OpNoop.String():
		return memcontract.OpNoop, nil
	case memcontract.OpAdd.String():
		return memcontract.OpAdd, nil
	case memcontract.OpUpdate.String():
		return memcontract.OpUpdate, nil
	case memcontract.OpDelete.String():
		return memcontract.OpDelete, nil
	case memcontract.OpReject.String():
		return memcontract.OpReject, nil
	default:
		return memcontract.OpNoop, fmt.Errorf("unsupported operation %q", value)
	}
}

func nullableSQLString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func nullableSQLStringRaw(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
