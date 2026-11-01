package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func appendPendingInteractionReport(ctx context.Context, exec globalSQLExecutor, interaction store.PendingInteraction, transition store.PendingInteractionTransition) error {
	var input agentinstance.ReportInput
	var payload any
	switch {
	case transition.Status == store.PendingInteractionStatusTimedOut && interaction.Kind == store.PendingInteractionKindPermission:
		tool := interaction.Payload.ToolID
		if tool == "" {
			tool = strings.TrimSpace(interaction.Title)
		}
		if tool == "" {
			tool = "provider"
		}
		expiredBy := "timeout"
		if transition.Resolution == "expired-by-provider" {
			expiredBy = "provider"
		}
		input.Kind, input.SourceKey = "approval_expired", "interaction:"+interaction.InteractionID
		payload = struct {
			SessionID string `json:"session_id"`
			Tool      string `json:"tool"`
			ExpiredBy string `json:"expired_by"`
		}{interaction.SessionID, tool, expiredBy}
	case transition.Status == store.PendingInteractionStatusOrphaned || transition.Resolution == "failed-by-restart":
		if strings.TrimSpace(interaction.TurnID) == "" {
			return nil
		}
		cause := "provider_interrupted"
		if transition.Resolution == "failed-by-restart" {
			cause = "daemon_restart"
		}
		input.Kind, input.SourceKey = "interrupted", "turn:"+interaction.TurnID
		payload = struct {
			SessionID string `json:"session_id"`
			Cause     string `json:"cause"`
		}{interaction.SessionID, cause}
	default:
		return nil
	}
	queries := sqlcgen.New(exec)
	caller, err := queries.GetInstanceCaller(ctx, interaction.SessionID)
	if err != nil {
		return fmt.Errorf("store: resolve pending report lineage: %w", err)
	}
	instanceID, ok := caller.OriginatingInstanceID.(string)
	if !ok {
		return fmt.Errorf("store: invalid pending report lineage")
	}
	if instanceID == "" {
		return nil
	}
	instance, err := queries.GetAgentInstance(ctx, instanceID)
	if err != nil {
		return err
	}
	if instance.ArchivedAt.Valid {
		return nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	input.Payload = encoded
	if err := input.Validate(); err != nil {
		return err
	}
	id, err := store.NewID("arpt")
	if err != nil {
		return err
	}
	return queries.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
		ID: id, InstanceID: instanceID, SourceKey: input.SourceKey, Kind: input.Kind,
		Payload: string(input.Payload), CreatedAt: store.FormatTimestamp(transition.At),
	})
}
