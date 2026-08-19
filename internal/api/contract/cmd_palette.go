package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type CmdPaletteCommand struct {
	ID                 cmdpalette.CommandID       `json:"id"`
	Title              string                     `json:"title"`
	Source             string                     `json:"source"`
	Section            string                     `json:"section"`
	Icon               string                     `json:"icon"`
	Keywords           []string                   `json:"keywords,omitempty"`
	Available          bool                       `json:"available"`
	Reason             string                     `json:"reason,omitempty"`
	Bindings           []string                   `json:"bindings"`
	Alias              *string                    `json:"alias"`
	Destructive        bool                       `json:"destructive"`
	Confirmation       *cmdpalette.Confirmation   `json:"confirmation,omitempty"`
	Arguments          []cmdpalette.Argument      `json:"arguments"`
	Action             cmdpalette.Action          `json:"action"`
	Execution          cmdpalette.ExecutionPolicy `json:"execution"`
	When               []cmdpalette.Predicate     `json:"when,omitempty"`
	AvailabilityExempt bool                       `json:"availability_exempt"`
}

type CmdPaletteCommandsResponse struct {
	Commands        []CmdPaletteCommand       `json:"commands"`
	Sources         []cmdpalette.SourceStatus `json:"sources"`
	CatalogRevision string                    `json:"catalog_revision"`
	ContextRevision string                    `json:"context_revision,omitempty"`
}

type CmdPaletteInvokeRequest struct {
	Workspace string         `json:"workspace"`
	Args      map[string]any `json:"args"`
	Client    string         `json:"client,omitempty"`
}

type CmdPaletteInvokeResult struct {
	Status     cmdpalette.InvokeStatus `json:"status"`
	Result     json.RawMessage         `json:"result,omitempty"`
	ApprovalID string                  `json:"approval_id,omitempty"`
}

type CmdPaletteClient struct {
	ClientID   cmdpalette.ClientID    `json:"client_id"`
	Kind       string                 `json:"kind"`
	Workspace  cmdpalette.WorkspaceID `json:"workspace"`
	AttachedAt time.Time              `json:"attached_at"`
}

type CmdPaletteCatalogChangedEvent struct {
	Workspace       cmdpalette.WorkspaceID `json:"workspace"`
	CatalogRevision string                 `json:"catalog_revision"`
}

type CmdPaletteUsageRequest struct {
	Workspace string               `json:"workspace"`
	CommandID cmdpalette.CommandID `json:"command_id"`
	Query     string               `json:"query"`
}

type CmdPalettePinResponse struct {
	Pinned bool `json:"pinned"`
}

type CmdPaletteRankSignalsResponse struct {
	Weights   cmdpalette.Weights       `json:"weights"`
	Usage     []cmdpalette.UsageSignal `json:"usage"`
	QueryHits []cmdpalette.QueryHit    `json:"query_hits"`
	Pins      []cmdpalette.CommandID   `json:"pins"`
	Revision  string                   `json:"revision"`
}

type CmdPalettePersonalizationResponse struct {
	Workspace         cmdpalette.WorkspaceID `json:"workspace"`
	Pins              []cmdpalette.CommandID `json:"pins"`
	Recents           int                    `json:"recents"`
	FrecencyEntries   int                    `json:"frecency_entries"`
	QueryAssociations int                    `json:"query_associations"`
}

type CmdPalettePersonalizationResetResponse struct {
	Status string `json:"status"`
}

type CmdPaletteError struct {
	Error   string                `json:"error"`
	Message string                `json:"message,omitempty"`
	Reason  string                `json:"reason,omitempty"`
	Fields  map[string]string     `json:"fields,omitempty"`
	Clients []cmdpalette.ClientID `json:"clients,omitempty"`
}

type ToolApprovalStatusResponse struct {
	ApprovalStatus  toolspkg.ApprovalOutcome         `json:"approval_status"`
	ExecutionStatus toolspkg.ApprovalExecutionStatus `json:"execution_status,omitempty"`
	Result          json.RawMessage                  `json:"result,omitempty"`
	Error           json.RawMessage                  `json:"error,omitempty"`
	ExpiresAt       *time.Time                       `json:"expires_at,omitempty"`
}

func CmdPaletteCommandsFromDomain(catalog cmdpalette.Catalog) CmdPaletteCommandsResponse {
	commands := make([]CmdPaletteCommand, 0, len(catalog.Commands))
	for _, command := range catalog.Commands {
		commands = append(commands, CmdPaletteCommand{
			ID: command.ID, Title: command.Title, Source: command.Source.ID(), Section: command.Section,
			Icon: command.Icon, Keywords: append([]string(nil), command.Keywords...),
			Available: command.Available, Reason: command.UnavailableReason,
			Bindings: append([]string(nil), command.Bindings...), Alias: cloneContractString(command.Alias),
			Destructive: command.Destructive, Confirmation: cloneConfirmation(command.Confirmation),
			Arguments: append([]cmdpalette.Argument(nil), command.Arguments...), Action: command.Action,
			Execution: command.Policy, When: append([]cmdpalette.Predicate(nil), command.When...),
			AvailabilityExempt: command.AvailabilityExempt,
		})
	}
	return CmdPaletteCommandsResponse{
		Commands: commands, Sources: append([]cmdpalette.SourceStatus(nil), catalog.Sources...),
		CatalogRevision: catalog.Revision, ContextRevision: catalog.ContextRevision,
	}
}

func CmdPaletteClientsFromDomain(clients []cmdpalette.Client) []CmdPaletteClient {
	result := make([]CmdPaletteClient, 0, len(clients))
	for _, client := range clients {
		result = append(result, CmdPaletteClient{
			ClientID: client.ID, Kind: client.Kind, Workspace: client.WorkspaceID, AttachedAt: client.AttachedAt,
		})
	}
	return result
}

func CmdPaletteRankSignalsFromDomain(snapshot cmdpalette.Snapshot) CmdPaletteRankSignalsResponse {
	pins := make([]cmdpalette.CommandID, 0, len(snapshot.Pins))
	for _, pin := range snapshot.Pins {
		pins = append(pins, pin.CommandID)
	}
	return CmdPaletteRankSignalsResponse{
		Weights: snapshot.Weights, Usage: append([]cmdpalette.UsageSignal(nil), snapshot.Usage...),
		QueryHits: append([]cmdpalette.QueryHit(nil), snapshot.QueryHits...),
		Pins:      pins, Revision: snapshot.Revision,
	}
}

func CmdPalettePersonalizationFromDomain(
	summary cmdpalette.PersonalizationSummary,
) CmdPalettePersonalizationResponse {
	return CmdPalettePersonalizationResponse{
		Workspace: summary.Workspace, Pins: append([]cmdpalette.CommandID(nil), summary.Pins...),
		Recents: summary.Recents, FrecencyEntries: summary.FrecencyEntries,
		QueryAssociations: summary.QueryAssociations,
	}
}

func ToolApprovalStatusFromDomain(status toolspkg.ApprovalStatus) ToolApprovalStatusResponse {
	response := ToolApprovalStatusResponse{
		ApprovalStatus: status.ApprovalStatus, ExecutionStatus: status.ExecutionStatus,
		Result: append(json.RawMessage(nil), status.Result...), Error: append(json.RawMessage(nil), status.Error...),
	}
	if status.ApprovalStatus == toolspkg.ApprovalPending {
		expiresAt := status.ExpiresAt
		response.ExpiresAt = &expiresAt
	}
	return response
}

func cloneContractString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneConfirmation(value *cmdpalette.Confirmation) *cmdpalette.Confirmation {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
