package globaldb

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func (r *InstanceReportRepo) ProjectPicks(
	ctx context.Context,
	report *agentinstance.Report,
	approved bool,
	limit, cap int, actions ...agentinstance.ActionSnapshot,
) error {
	rows, err := r.readQueries.ListProposalPicks(ctx, report.ID)
	if err != nil {
		return err
	}
	picks := make([]agentinstance.Pick, 0, len(rows))
	picked := map[int]bool{}
	for _, row := range rows {
		at, err := store.ParseTimestamp(row.PickedAt)
		if err != nil {
			return err
		}
		picks = append(
			picks,
			agentinstance.Pick{
				InstanceID:    row.InstanceID,
				ReportID:      row.ReportID,
				ProposalIndex: int(row.ProposalIndex),
				Actor:         row.Actor,
				ActionID:      row.ActionID.String,
				TargetSession: row.TargetSessionID,
				QueueEntryID:  row.QueueEntryID.String,
				State:         pickQueueState(row.QueueStatus),
				PickedAt:      at,
			},
		)
		picked[int(row.ProposalIndex)] = true
	}
	var payload map[string]jsontext.Value
	if err := json.Unmarshal(report.Payload, &payload); err != nil {
		return err
	}
	payload["picks"], err = json.Marshal(picks)
	if err != nil {
		return err
	}
	if report.Kind == "proposal" {
		var proposals []map[string]jsontext.Value
		if err := json.Unmarshal(payload["proposals"], &proposals); err != nil {
			return err
		}
		count, err := r.readQueries.CountAutoPicks(
			ctx,
			sqlcgen.CountAutoPicksParams{
				InstanceID: report.InstanceID,
				PickedAt:   store.FormatTimestamp(r.now().Add(-24 * time.Hour)),
			},
		)
		if err != nil {
			return err
		}
		instance, err := r.readQueries.GetAgentInstance(ctx, report.InstanceID)
		if err != nil {
			return err
		}
		for i, p := range proposals {
			status := "none"
			var action agentinstance.ActionSnapshot
			if raw, ok := p["action"]; ok {
				var id string
				if err := json.Unmarshal(raw, &id); err != nil {
					if err := json.Unmarshal(raw, &action); err != nil {
						return err
					}
					id = action.ID
				}
				action.ID = id
				for _, decl := range actions {
					if decl.ID == id {
						action = decl
						break
					}
				}
			}
			if action.ID != "" {
				p["action"], err = json.Marshal(struct {
					ID         string `json:"id"`
					Title      string `json:"title"`
					AutoAccept bool   `json:"auto_accept"`
				}{action.ID, action.Title, action.AutoAccept})
				if err != nil {
					return err
				}
			}
			switch {
			case picked[i]:
				status = "picked"
			case action.AutoAccept && !approved:
				status = "unapproved"
			case action.AutoAccept && count >= int64(limit):
				status = "limit"
			case action.AutoAccept && instance.MainSessionID.Valid:
				pending, err := countPendingSessionInputs(
					ctx,
					r.readDB,
					instance.MainSessionID.String,
					store.SessionInputOriginClassSynthetic,
				)
				if err != nil {
					return err
				}
				if pending >= cap {
					status = "queue_full"
				}
			}
			p["auto_accept_status"], err = json.Marshal(status)
			if err != nil {
				return err
			}
		}
		payload["proposals"], err = json.Marshal(proposals)
		if err != nil {
			return err
		}
	}
	report.Payload, err = json.Marshal(payload)
	return err
}
