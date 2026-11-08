package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// preparePickReport stamps server tracking without copying projected or authored data.
func preparePickReport(data []byte) ([]byte, error) {
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	fields["picks_tracked"] = jsontext.Value(`true`)
	return json.Marshal(fields)
}

func pickProposals(data []byte, actions []agentinstance.ActionSnapshot) ([]agentinstance.ProposalPayload, error) {
	var payload struct {
		Proposals []heartbeat.TriageProposal `json:"proposals"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	out := make([]agentinstance.ProposalPayload, 0, len(payload.Proposals))
	for _, raw := range payload.Proposals {
		p := agentinstance.ProposalPayload{Title: raw.Title, Body: raw.Body, ItemKeys: raw.ItemKeys}
		for _, action := range actions {
			if action.ID == raw.Action {
				p.Action = new(action)
				break
			}
		}
		out = append(out, p)
	}
	return out, nil
}

func actionSnapshots(actions []heartbeat.ActionDecl) []agentinstance.ActionSnapshot {
	out := make([]agentinstance.ActionSnapshot, 0, len(actions))
	for _, action := range actions {
		out = append(
			out,
			agentinstance.ActionSnapshot{
				ID:           action.ID,
				Title:        action.Title,
				Instructions: action.Instructions,
				AutoAccept:   action.AutoAccept,
			},
		)
	}
	return out
}

func autoPickFinishedReport(
	ctx context.Context,
	exec globalSQLExecutor,
	req *heartbeat.FinishRequest,
) ([]agentinstance.Pick, error) {
	if req.Result != "reported" || req.Report == nil || req.Report.Kind != instanceReportProposal {
		return nil, nil
	}
	q := sqlcgen.New(exec)
	instance, err := q.GetAgentInstance(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}
	approvedDigest, err := autoPickAuthorityDigest(ctx, q, req, instance.ProfileID)
	if err != nil {
		return nil, err
	}
	report, err := q.GetAgentInstanceReportBySource(
		ctx,
		sqlcgen.GetAgentInstanceReportBySourceParams{InstanceID: req.InstanceID, SourceKey: "occ:" + req.OccurrenceID},
	)
	if err != nil {
		return nil, err
	}
	proposals, err := pickProposals([]byte(report.Payload), actionSnapshots(req.Actions))
	if err != nil {
		return nil, err
	}
	since := store.FormatTimestamp(req.Now.Add(-24 * time.Hour))
	count, err := q.CountAutoPicks(ctx, sqlcgen.CountAutoPicksParams{InstanceID: req.InstanceID, PickedAt: since})
	if err != nil {
		return nil, err
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal([]byte(report.Payload), &fields); err != nil {
		return nil, err
	}
	var stored []map[string]jsontext.Value
	if err := json.Unmarshal(fields["proposals"], &stored); err != nil {
		return nil, err
	}
	picks := []agentinstance.Pick{}
	for index, p := range proposals {
		if p.Action == nil || !p.Action.AutoAccept {
			continue
		}
		skip := autoPickSkip(req, approvedDigest, instance, count)
		if skip != "" {
			stored[index]["auto_accept_skip"], err = json.Marshal(skip)
			if err != nil {
				return nil, err
			}
			continue
		}
		pick, err := insertInstancePick(
			ctx,
			exec,
			agentinstance.PickRequest{
				InstanceID:    req.InstanceID,
				ReportID:      report.ID,
				Actor:         pickActorAutoAccept,
				ProposalIndex: index,
				QueueCap:      req.QueueCap,
			},
			p,
			req.Now,
		)
		if errors.Is(err, store.ErrSessionInputQueueFull) {
			stored[index]["auto_accept_skip"] = jsontext.Value(`"queue_full"`)
			continue
		}
		if err != nil {
			return nil, err
		}
		picks = append(picks, pick)
		count++
	}
	return picks, saveAutoPickDecisions(ctx, q, report.ID, fields, stored)
}

func autoPickSkip(
	req *heartbeat.FinishRequest,
	approvedDigest string,
	instance sqlcgen.AgentInstance,
	count int64,
) string {
	skip := ""
	switch {
	case req.AuthorityDigest == "" || approvedDigest != req.AuthorityDigest:
		skip = "unapproved"
	case !instance.MainSessionID.Valid:
		skip = "no_main"
	case instance.State != instanceStateActive || instance.ArchivedAt.Valid:
		skip = "inactive"
	case count >= int64(req.AutoAcceptPerDay):
		skip = "limit"
	}

	return skip
}

func autoPickAuthorityDigest(
	ctx context.Context,
	q *sqlcgen.Queries,
	req *heartbeat.FinishRequest,
	profileID string,
) (string, error) {
	approvedDigest := ""
	if req.AuthorityDigest != "" {
		dir, err := filepath.EvalSymlinks(req.DefinitionDir)
		if err != nil {
			return "", err
		}
		approvedDigest, err = q.GetHeartbeatAuthorityApproval(
			ctx,
			sqlcgen.GetHeartbeatAuthorityApprovalParams{ProfileID: profileID, DefinitionDir: dir},
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}
	return approvedDigest, nil
}

func saveAutoPickDecisions(ctx context.Context, q *sqlcgen.Queries, reportID string,
	fields map[string]jsontext.Value, stored []map[string]jsontext.Value) error {
	var err error
	fields["proposals"], err = json.Marshal(stored)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	err = q.UpdateInstanceReportPickDecisions(
		ctx,
		sqlcgen.UpdateInstanceReportPickDecisionsParams{Payload: string(payload), ID: reportID},
	)
	return err
}
