package core

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// ListPage fills an authorized page before deciding whether a next page exists.
func (api *AgentInstanceAPI) ListPage(
	ctx context.Context,
	query agentinstance.ListQuery,
	actor workspaceaccess.ActorRef,
) (contract.AgentInstanceListResponse, error) {
	owners, err := profileOwnerIdentities(ctx, api.Profiles)
	if err != nil {
		return contract.AgentInstanceListResponse{}, err
	}
	profileIDs := []string{query.ProfileID}
	if query.AllProfiles {
		profileIDs = ownerProfileIDs(owners)
	}
	result, err := api.ListResponse(ctx, profileIDs...)
	if err != nil {
		return result, err
	}
	limit := query.Limit
	if limit < 1 || limit > 100 {
		return result, errors.New("limit must be from 1 to 100")
	}
	query.Limit = min(limit+1, 100)
	for {
		rows, err := api.Store.List(ctx, query)
		if err != nil {
			return result, err
		}
		for _, row := range rows {
			if err := api.Authority.Authorize(ctx, actor, row, "list"); err != nil {
				if errors.Is(err, agentinstance.ErrForbidden) {
					continue
				}
				return result, err
			}
			if len(result.Instances) == limit {
				result.NextCursor = result.Instances[limit-1].ID
				return result, nil
			}
			payload, err := api.payloadWithOwners(ctx, row, owners)
			if err != nil {
				return result, err
			}
			result.Instances = append(result.Instances, payload)
		}
		if len(rows) < query.Limit {
			return result, nil
		}
		query.AfterID = rows[len(rows)-1].ID
	}
}

// ownerProfileIDs lists each known owner once, in a stable order.
func ownerProfileIDs(owners map[string]profileOwnerIdentity) []string {
	ids := make([]string, 0, len(owners))
	for _, owner := range owners {
		if owner.ID != "" && !slices.Contains(ids, owner.ID) {
			ids = append(ids, owner.ID)
		}
	}
	slices.Sort(ids)
	return ids
}

// ReportsPage shares report hydration and terminal-page detection across transports.
func (api *AgentInstanceAPI) ReportsPage(
	ctx context.Context,
	query agentinstance.ReportQuery,
) (contract.AgentInstanceReportsResponse, error) {
	result := contract.AgentInstanceReportsResponse{Reports: []contract.AgentInstanceReportPayload{}}
	rows, err := api.Reports.ListReports(ctx, query)
	if err != nil {
		return result, err
	}
	for _, report := range rows {
		var payload contract.AgentInstanceReportPayload
		if err := json.Unmarshal(report.Payload, &payload); err != nil {
			return result, err
		}
		payload.ID, payload.Kind, payload.State, payload.CreatedAt = report.ID, report.Kind, report.State, report.CreatedAt
		result.Reports = append(result.Reports, payload)
	}
	if len(rows) == query.Limit {
		last := rows[len(rows)-1]
		probe := query
		probe.AfterID, probe.AfterCreatedAt, probe.Limit = last.ID, last.CreatedAt, 1
		next, err := api.Reports.ListReports(ctx, probe)
		if err != nil {
			return result, err
		}
		if len(next) > 0 {
			result.NextCursor = base64.RawURLEncoding.EncodeToString(
				[]byte(
					query.InstanceID + "\n" + last.CreatedAt.Format(
						time.RFC3339Nano,
					) + "\n" + last.ID + "\n" + query.State,
				),
			)
		}
	}
	return result, nil
}
