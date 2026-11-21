package core

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var errInvalidPeersQuery = errors.New("invalid peers query or cursor")

type instancePeersCursor struct {
	Profile, Workspace, Agent, State, Caller, After string
	All, AllProfiles                                bool
	ExcludeInstanceID                               string `json:",omitempty"`
}

// PeersPage applies both instance and workspace authority before filling a page.
// It never hydrates reports, session state or private runtime settings for peers.
func (api *AgentInstanceAPI) PeersPage(
	ctx context.Context,
	query agentinstance.ListQuery,
	actor workspaceaccess.ActorRef,
	access workspaceaccess.Policy,
	excludeInstanceID string,
) (contract.AgentInstanceListResponse, error) {
	result := contract.AgentInstanceListResponse{
		Instances: []contract.AgentInstancePayload{},
		Peers:     []contract.AgentInstancePeer{},
	}
	if query.Limit < 1 || query.Limit > 100 {
		return result, errInvalidPeersQuery
	}
	filter := instancePeersCursor{
		Profile:           query.ProfileID,
		Workspace:         query.WorkspaceID,
		Agent:             query.AgentName,
		State:             query.State,
		Caller:            actor.SessionID,
		All:               query.AllWorkspaces,
		AllProfiles:       query.AllProfiles,
		ExcludeInstanceID: excludeInstanceID,
	}
	canReportFiltered := canReportFilteredPeers(query, actor)
	if query.AfterID != "" {
		after, err := filter.decodeAfter(query.AfterID)
		if err != nil {
			return result, err
		}
		query.AfterID = after
	}
	limit := query.Limit
	query.Limit = min(limit+1, 100)
	last := ""
	filtered := false
	for {
		rows, err := api.Store.List(ctx, query)
		if err != nil {
			return result, err
		}
		for _, row := range rows {
			if row.ID == excludeInstanceID {
				continue
			}
			allowed, err := api.peerVisible(ctx, actor, access, row)
			if err != nil {
				return result, err
			}
			if !allowed {
				filtered = true
				continue
			}
			if len(result.Peers) == limit {
				filter.After = last
				encoded, err := json.Marshal(filter)
				if err != nil {
					return result, err
				}
				result.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
				return result, nil
			}
			peer := contract.AgentInstancePeer{
				Name:  row.AgentName,
				Scope: row.WorkspaceID,
				State: row.State,
			}
			if api.ReadPeer != nil {
				peer, err = api.ReadPeer(ctx, row)
				if err != nil {
					return result, err
				}
			}
			result.Peers = append(result.Peers, peer)
			last = row.ID
		}
		if len(rows) < query.Limit {
			if canReportFiltered && filtered && len(result.Peers) == 0 {
				result.PeersNote = "Access restrictions hid every peer candidate."
			}
			return result, nil
		}
		query.AfterID = rows[len(rows)-1].ID
	}
}

func (filter instancePeersCursor) decodeAfter(encoded string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	var cursor instancePeersCursor
	if err != nil || json.Unmarshal(raw, &cursor) != nil {
		return "", errInvalidPeersQuery
	}
	after := cursor.After
	cursor.After = ""
	if cursor != filter || after == "" {
		return "", errInvalidPeersQuery
	}
	return after, nil
}

// Narrow selectors and cursors must not confirm the existence of specific hidden peers.
func canReportFilteredPeers(query agentinstance.ListQuery, actor workspaceaccess.ActorRef) bool {
	return query.AfterID == "" && query.AgentName == "" && query.State == "" &&
		!query.AllProfiles && (query.AllWorkspaces || query.WorkspaceID == actor.WorkspaceID)
}

func (api *AgentInstanceAPI) peerVisible(ctx context.Context, actor workspaceaccess.ActorRef,
	access workspaceaccess.Policy, row agentinstance.Instance) (bool, error) {
	if err := api.Authority.Authorize(ctx, actor, row, "peers"); err != nil {
		if errors.Is(err, agentinstance.ErrForbidden) {
			return false, nil
		}
		return false, err
	}
	if actor.Operator {
		return true, nil
	}
	if access == nil {
		return false, agentinstance.ErrForbidden
	}
	decision, err := access.Authorize(ctx, workspaceaccess.Request{
		Actor: actor,
		Target: workspaceaccess.ScopeTarget(
			row.WorkspaceID,
			row.ProfileID,
		),
		ReadOnly: true,
		Seam:     workspaceaccess.SeamCatalog,
	})
	return decision.Allowed, err
}
