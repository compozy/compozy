package globaldb

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
)

func (r *InstanceReportRepo) ProjectPicks(
	ctx context.Context,
	report *agentinstance.Report,
	actions ...agentinstance.ActionSnapshot,
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
		picked[int(row.ProposalIndex)] = row.Actor == "auto_accept"
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
			if raw := p["auto_accept_skip"]; len(raw) > 0 {
				if err := json.Unmarshal(raw, &status); err != nil {
					return err
				}
			}
			if picked[i] {
				status = "picked"
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
