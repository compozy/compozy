package core

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/remoteproto"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const placementWorkspacesPath = "/api/workspaces?include_repository=true"

func (h *BaseHandlers) PreviewComputerPlacement(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	var input contract.ComputerPlacementRequest
	if err := decodeStrictJSONBody(c, &input); err != nil || strings.TrimSpace(input.Workspace) == "" {
		invalidComputerBody(c)
		return
	}
	result, err := h.computerPlacement(c.Request.Context(), input)
	h.computerResult(c, http.StatusOK, result, err)
}

func (h *BaseHandlers) computerPlacement(
	ctx context.Context, req contract.ComputerPlacementRequest,
) (contract.ComputerPlacementResponse, error) {
	settings := h.Computers.LocalSettings()
	if !settings.LoadBalancing {
		return contract.ComputerPlacementResponse{}, computers.NoEligibleComputer(
			map[string]string{computers.AutoPlacement: "load_balancing_disabled"},
		)
	}
	views, err := h.Computers.List(ctx)
	if err != nil {
		return contract.ComputerPlacementResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	projectHost := cmp.Or(req.ProjectComputerID, computers.LocalID)
	project, sourceWorkspace := h.placementProject(ctx, projectHost, req.Workspace)
	if project == "" {
		return contract.ComputerPlacementResponse{}, computers.NoEligibleComputer(
			map[string]string{string(projectHost): "project_not_found"},
		)
	}
	candidates := make([]computers.PlacementCandidate, len(views)+1)
	candidates[0] = computers.PlacementCandidate{
		ComputerID: computers.LocalID, Name: "This computer", Enabled: settings.LocalRunAgents,
		Connected: true, Weight: settings.LocalLoadBalancingWeight,
	}
	var wg sync.WaitGroup
	for i := range candidates {
		if i > 0 {
			view := views[i-1]
			candidates[i] = computers.PlacementCandidate{
				ComputerID: view.ID, Name: view.Name, Enabled: view.Enabled,
				Connected: view.Status == computers.Connected, Weight: view.Weight,
			}
		}
		c := &candidates[i]
		if c.Enabled && c.Connected && c.Weight > 0 {
			wg.Go(func() { h.samplePlacementCandidate(ctx, c, projectHost, project, sourceWorkspace) })
		}
	}
	wg.Wait()
	selected, err := computers.SelectPlacement(h.nowUTC(), candidates)
	return contract.ComputerPlacementResponse{
		ComputerID: selected.ComputerID, Name: selected.Name, WorkspaceID: selected.WorkspaceID,
	}, err
}

func (h *BaseHandlers) placementProject(ctx context.Context, host remoteproto.ComputerID, ref string) (string, string) {
	if host == computers.LocalID {
		workspace, err := h.Workspaces.Get(ctx, ref)
		if err != nil {
			return "", ""
		}
		return h.workspaceRepositoryKey(ctx, workspace), workspace.ID
	}
	var catalog contract.WorkspacesResponse
	if err := h.readPlacementComputer(ctx, host, placementWorkspacesPath, &catalog); err != nil {
		return "", ""
	}
	for _, workspace := range catalog.Workspaces {
		if workspace.ID == ref {
			return workspace.RepositoryKey, workspace.ID
		}
	}
	return "", ""
}

func (h *BaseHandlers) samplePlacementCandidate(
	ctx context.Context,
	c *computers.PlacementCandidate,
	source remoteproto.ComputerID,
	project, sourceWorkspace string,
) {
	switch c.ComputerID {
	case source:
		c.WorkspaceID = sourceWorkspace
	case computers.LocalID:
		c.WorkspaceID = h.localWorkspaceForPlacement(ctx, project)
	default:
		var catalog contract.WorkspacesResponse
		if err := h.readPlacementComputer(ctx, c.ComputerID, placementWorkspacesPath, &catalog); err == nil {
			for _, workspace := range catalog.Workspaces {
				if workspace.RepositoryKey == project && (c.WorkspaceID == "" || workspace.ID < c.WorkspaceID) {
					c.WorkspaceID = workspace.ID
				}
			}
		}
	}
	if c.WorkspaceID == "" {
		return
	}
	if c.ComputerID == computers.LocalID {
		c.Resources = computers.ResourceSample(h.hostResourceSample())
		return
	}
	var sample contract.HostResourcesResponse
	if err := h.readPlacementComputer(ctx, c.ComputerID, contract.HostResourcesPath, &sample); err == nil {
		c.Resources = computers.ResourceSample(sample)
	}
}

func (h *BaseHandlers) localWorkspaceForPlacement(ctx context.Context, project string) string {
	workspaces, err := h.Workspaces.List(ctx)
	if err != nil {
		return ""
	}
	slices.SortFunc(workspaces, func(a, b workspacepkg.Workspace) int { return cmp.Compare(a.ID, b.ID) })
	for _, workspace := range workspaces {
		if h.workspaceRepositoryKey(ctx, workspace) == project {
			return workspace.ID
		}
	}
	return ""
}

func (h *BaseHandlers) workspaceRepositoryKey(ctx context.Context, workspace workspacepkg.Workspace) string {
	if workspace.ArchivedAt != nil || workspace.RootMissing {
		return ""
	}
	reader, ok := h.Worktrees.(interface {
		RepositoryKey(context.Context, string) (string, error)
	})
	if !ok {
		return ""
	}
	key, err := reader.RepositoryKey(ctx, workspace.RootDir)
	if err != nil {
		return ""
	}
	return key
}

func (h *BaseHandlers) readPlacementComputer(
	ctx context.Context,
	id remoteproto.ComputerID,
	path string,
	out any,
) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	if err != nil {
		return err
	}
	response, err := h.Computers.RoundTrip(ctx, id, request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("computer placement probe: HTTP %d", response.StatusCode)
	}
	return json.UnmarshalRead(response.Body, out)
}
