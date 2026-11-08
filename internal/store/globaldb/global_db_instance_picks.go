package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"

	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// PickProposal owns manual pick idempotency and queue admission in one transaction.
func (r *InstanceRepo) PickProposal(ctx context.Context, req agentinstance.PickRequest) (agentinstance.Pick, error) {
	if err := r.checkReady(ctx, "pick instance proposal"); err != nil {
		return agentinstance.Pick{}, err
	}
	if req.Actor != pickActorOperator {
		return agentinstance.Pick{}, errors.New("proposal_pick_operator_only")
	}
	var pick agentinstance.Pick
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"pick instance proposal",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			row, err := q.GetInstanceReportForPick(
				ctx,
				sqlcgen.GetInstanceReportForPickParams{ID: req.ReportID, InstanceID: req.InstanceID},
			)
			if errors.Is(err, sql.ErrNoRows) {
				return agentinstance.ErrReportNotFound
			}
			if err != nil {
				return err
			}
			if row.Kind != instanceReportProposal {
				return agentinstance.ErrReportNotProposal
			}
			proposals, err := pickProposals([]byte(row.Payload), req.Actions)
			if err != nil {
				return err
			}
			if req.ProposalIndex < 0 || req.ProposalIndex >= len(proposals) {
				return agentinstance.ErrProposalRange
			}
			_, err = q.GetProposalPick(
				ctx,
				sqlcgen.GetProposalPickParams{ReportID: req.ReportID, ProposalIndex: int64(req.ProposalIndex)},
			)
			if err == nil {
				return agentinstance.ErrAlreadyPicked
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			pick, err = insertInstancePick(ctx, tx, req, proposals[req.ProposalIndex], r.now())
			return err
		},
	)
	return pick, err
}

func insertInstancePick(
	ctx context.Context,
	exec globalSQLExecutor,
	req agentinstance.PickRequest,
	p agentinstance.ProposalPayload,
	now time.Time,
) (agentinstance.Pick, error) {
	q := sqlcgen.New(exec)
	instance, err := q.GetAgentInstance(ctx, req.InstanceID)
	if err != nil {
		return agentinstance.Pick{}, err
	}
	if instance.State != instanceStateActive || instance.ArchivedAt.Valid {
		return agentinstance.Pick{}, agentinstance.ErrInstanceNotActive
	}
	target := instance.MainSessionID.String
	if target == "" {
		return agentinstance.Pick{}, agentinstance.ErrNoMainSession
	}
	generation, err := q.GetSessionInputGeneration(ctx, target)
	if err != nil {
		return agentinstance.Pick{}, err
	}
	id, err := store.NewID("input")
	if err != nil {
		return agentinstance.Pick{}, err
	}
	text, err := agentinstance.PickPromptText(p, p.Action)
	if err != nil {
		return agentinstance.Pick{}, err
	}
	entry := store.SessionInputQueueInsert{
		ID:                id,
		SessionID:         target,
		MessageID:         fmt.Sprintf("pick:%s:%d", req.ReportID, req.ProposalIndex),
		Text:              text,
		Mode:              store.SessionInputQueueModeQueue,
		Delivery:          store.SessionInputDeliveryAfterTurn,
		SessionGeneration: generation,
		QueueCap:          req.QueueCap,
		Now:               now,
		HumanIngress:      req.Actor == pickActorOperator,
	}
	if err := setAutoPickMetadata(&entry, req, p); err != nil {
		return agentinstance.Pick{}, err
	}
	if err := admitPickInput(ctx, exec, entry); err != nil {
		return agentinstance.Pick{}, err
	}
	action := ""
	if p.Action != nil {
		action = p.Action.ID
	}
	err = q.InsertProposalPick(
		ctx,
		sqlcgen.InsertProposalPickParams{
			ReportID:        req.ReportID,
			ProposalIndex:   int64(req.ProposalIndex),
			InstanceID:      req.InstanceID,
			ActionID:        sql.NullString{String: action, Valid: action != ""},
			Actor:           req.Actor,
			TargetSessionID: target,
			QueueEntryID:    sql.NullString{String: id, Valid: true},
			PickedAt:        store.FormatTimestamp(now),
		},
	)
	return agentinstance.Pick{
		InstanceID:    req.InstanceID,
		ReportID:      req.ReportID,
		ProposalIndex: req.ProposalIndex,
		Actor:         req.Actor,
		ActionID:      action,
		TargetSession: target,
		QueueEntryID:  id,
		State:         inputStatusQueued,
		PickedAt:      now,
	}, err
}

func pickQueueState(status string) string {
	switch status {
	case inputStatusQueued, "dispatching":
		return inputStatusQueued
	case "sent":
		return "delivered"
	case "canceled":
		return "cancelled" //nolint:misspell // projected wire enum
	default:
		return "failed"
	}
}

const (
	runtimeStatusRunning   = "running"
	checkInDecisionDenied  = "denied"
	instanceReportProposal = "proposal"
	instanceStateActive    = "active"
	pickActorAutoAccept    = "auto_accept"
	pickActorOperator      = "operator"
	inputStatusQueued      = "queued"
	autoAcceptStatusNone   = "none"
)

func setAutoPickMetadata(
	entry *store.SessionInputQueueInsert,
	req agentinstance.PickRequest,
	p agentinstance.ProposalPayload,
) error {
	if req.Actor == pickActorAutoAccept {
		turn, err := store.NewID("turn")
		if err != nil {
			return err
		}
		run, err := store.NewID("run")
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(
			map[string]any{
				"kind":           pickActorAutoAccept,
				"reason":         "proposal_auto_accept",
				"summary":        p.Title,
				"instance_id":    req.InstanceID,
				"report_id":      req.ReportID,
				"proposal_index": req.ProposalIndex,
			},
		)
		if err != nil {
			return err
		}
		entry.OwnerKind = store.SessionInputOwnerSynthetic
		entry.TurnID = turn
		entry.SyntheticPrompt = &store.SessionInputSyntheticPrompt{RunID: run, Metadata: metadata}
	}
	return nil
}

func admitPickInput(ctx context.Context, exec globalSQLExecutor, entry store.SessionInputQueueInsert) error {
	entry = entry.Normalize()
	if err := entry.Validate(); err != nil {
		return err
	}
	if err := checkSessionInputQueueCap(ctx, exec, entry); err != nil {
		return err
	}
	if _, err := insertSessionInputQueueEntry(ctx, exec, entry); err != nil {
		return err
	}
	return nil
}
