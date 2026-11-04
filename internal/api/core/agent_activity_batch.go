package core

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) BatchAgentActivity(c *gin.Context) {
	if !h.requireOperatorOrGrantedAgent(c, "agent activity") {
		return
	}
	var input contract.AgentActivityBatchRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		h.respondError(c, 400, err)
		return
	}
	if input.Days == 0 {
		input.Days = 30
	}
	if input.Days < 1 || input.Days > 365 || len(input.Items) == 0 || len(input.Items) > 100 {
		h.respondError(c, 422, errors.New("activity requires 1 to 100 items and days from 1 to 365"))
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if scope.AllProfiles {
		h.respondProfileReadScopeError(c, singleProfileReadError("agent activity belongs to one profile"))
		return
	}
	reader, err := h.agentActivityReader()
	if err != nil {
		h.respondError(c, 503, err)
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	queries, labels, err := h.agentActivityQueries(c, scope.ProfileID, actor, input, now)
	if err != nil {
		h.respondAgentActivityError(c, err)
		return
	}
	activities, err := reader.ReadActivities(c.Request.Context(), queries)
	if err != nil {
		h.respondError(c, 500, err)
		return
	}
	response := contract.AgentActivityBatchResponse{Activities: make([]contract.AgentActivityResponse, len(activities))}
	for i := range activities {
		for j := range activities[i].CostByProject {
			activities[i].CostByProject[j].Scope = labels[activities[i].CostByProject[j].WorkspaceID]
		}
		sortAgentActivity(&activities[i])
		response.Activities[i] = contract.AgentActivityResponse(activities[i])
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) agentActivityQueries(
	c *gin.Context,
	profileID string,
	actor workspaceaccess.ActorRef,
	input contract.AgentActivityBatchRequest,
	now time.Time,
) ([]agentinstance.ActivityQuery, map[string]string, error) {
	queries := make([]agentinstance.ActivityQuery, len(input.Items))
	selection := agentActivityScopes{handlers: h, context: c, profileID: profileID, actor: actor}
	scopeCount := 0
	for i, item := range input.Items {
		if item.Name == "" || len(item.Workspaces) == 0 || len(item.Workspaces) > 100 {
			return nil, nil, fmt.Errorf("%w: each item requires a name and 1 to 100 workspaces", errActivityRequest)
		}
		workspaces, err := selection.selectWorkspaces(item.Workspaces)
		if err != nil {
			return nil, nil, err
		}
		scopeCount += len(workspaces)
		if scopeCount > 5000 {
			return nil, nil, fmt.Errorf("%w: at most 5000 workspace selections", errActivityRequest)
		}
		queries[i] = agentinstance.ActivityQuery{
			ProfileID:    profileID,
			AgentName:    item.Name,
			WorkspaceIDs: workspaces,
			Since:        store.LocalDayStart(now, input.Days-1),
		}
	}
	return queries, selection.labels, nil
}

var errActivityRequest = errors.New("invalid activity request")

type agentActivityScopes struct {
	handlers  *BaseHandlers
	context   *gin.Context
	profileID string
	actor     workspaceaccess.ActorRef
	allowed   map[string]bool
	visible   map[string]bool
	labels    map[string]string
	resolved  map[string]string
}

func (h *BaseHandlers) respondAgentActivityError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	if errors.Is(err, errActivityRequest) {
		status = http.StatusUnprocessableEntity
	}
	h.respondError(c, status, err)
}

func (s *agentActivityScopes) selectWorkspaces(refs []string) ([]string, error) {
	if s.allowed == nil {
		allowed, err := s.handlers.aggregateWorkspaceSet(s.context, WorkspaceBreadth{AllWorkspaces: true})
		if err != nil {
			return nil, err
		}
		s.allowed = allowed
		s.visible = make(map[string]bool)
		s.labels = make(map[string]string)
		s.resolved = make(map[string]string)
	}
	selected := make(map[string]bool)
	for _, ref := range refs {
		id, known := s.resolved[ref]
		scoped := ref != ""
		if !known {
			if _, isID := s.allowed[ref]; isID && ref != "" {
				id = ref
			} else {
				var err error
				id, scoped, err = s.handlers.resolveAgentActivityWorkspace(s.context.Request.Context(), ref)
				if err != nil {
					if errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
						return nil, fmt.Errorf("%w: %w", errActivityRequest, err)
					}
					return nil, err
				}
			}
			s.resolved[ref] = id
		}
		if !scoped {
			for id := range s.allowed {
				selected[id] = true
			}
		} else if _, ok := s.allowed[id]; ok {
			selected[id] = true
		}
	}
	workspaces := make([]string, 0, len(selected))
	for workspace := range selected {
		visible, known := s.visible[workspace]
		if !known {
			var err error
			visible, err = s.handlers.authorizeAgentActivityWorkspace(
				s.context.Request.Context(),
				s.actor,
				s.profileID,
				workspace,
			)
			if err != nil {
				return nil, err
			}
			s.visible[workspace] = visible
			if visible {
				label, err := s.handlers.agentActivityWorkspaceLabel(s.context.Request.Context(), workspace)
				if err != nil {
					return nil, err
				}
				s.labels[workspace] = label
			}
		}
		if visible {
			workspaces = append(workspaces, workspace)
		}
	}
	slices.Sort(workspaces)
	return workspaces, nil
}
