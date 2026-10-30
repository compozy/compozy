package core

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/gin-gonic/gin"
)

const processUserScope = "@user"

type SessionProcessService interface {
	Refresh(context.Context) (sessionprocess.Snapshot, error)
	Summary(string) sessionprocess.Summary
	Acquire() func()
	Terminate(
		context.Context,
		string,
		[]sessionprocess.Identity,
		bool,
		func(sessionprocess.Owner) bool,
	) ([]sessionprocess.Result, error)
	AfterStop(string, func(context.Context) error)
}

func (h *BaseHandlers) requireSessionProcesses(c *gin.Context) bool {
	if h.SessionProcesses != nil {
		return true
	}
	h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session processes unavailable"))
	return false
}

func (h *BaseHandlers) ListSessionProcesses(c *gin.Context) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) || !h.requireSessionProcesses(c) {
		return
	}
	snapshot, err := h.SessionProcesses.Refresh(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	response := contract.SessionProcessesResponse{
		Supported: snapshot.Supported,
		ScannedAt: snapshot.ScannedAt,
		Items:     []contract.SessionProcessPayload{},
	}
	for _, p := range snapshot.Processes {
		if p.Owner.SessionID == id && scope.Matches(p.Owner.ProfileID) {
			response.Items = append(response.Items, SessionProcessPayload(p, snapshot.ScannedAt))
		}
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) TerminateSessionProcesses(c *gin.Context) {
	_, id, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) || !h.requireSessionProcesses(c) {
		return
	}
	var req contract.TerminateSessionProcessesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	targets, err := ProcessTargets(req.Targets, req.All)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	results, err := h.SessionProcesses.Terminate(
		c.Request.Context(),
		id,
		targets,
		req.All,
		func(o sessionprocess.Owner) bool {
			return scope.Matches(o.ProfileID) && o.WorkspaceID == info.WorkspaceID
		},
	)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, ProcessResults(h.SessionProcesses.Summary(id).Supported, results))
}

func (h *BaseHandlers) ListProcesses(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session process aggregate") || !h.requireSessionProcesses(c) {
		return
	}
	scope, allWorkspaces, workspaceID, err := h.parseSessionCatalogReadScope(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if workspaceID == processUserScope {
		workspaceID = ""
	}
	orphaned, err := parseBoolQuery(c, "orphaned")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	snapshot, err := h.SessionProcesses.Refresh(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	owners, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	response := contract.ProcessesResponse{
		Supported: snapshot.Supported,
		ScannedAt: snapshot.ScannedAt,
		Groups:    []contract.SessionProcessGroupPayload{},
	}
	groups := make(map[string]int)
	for _, p := range snapshot.Processes {
		if !scope.Matches(p.Owner.ProfileID) || (!allWorkspaces && p.Owner.WorkspaceID != workspaceID) ||
			(orphaned && p.Owner.Live) {
			continue
		}
		key := p.Owner.ProfileID + "/" + p.Owner.WorkspaceID + "/" + p.Owner.SessionID
		i, ok := groups[key]
		if !ok {
			i = len(response.Groups)
			groups[key] = i
			profile := p.Owner.ProfileID
			if owner, ok := owners[profile]; ok {
				profile = owner.Name
			}
			response.Groups = append(
				response.Groups,
				contract.SessionProcessGroupPayload{
					SessionID:   p.Owner.SessionID,
					WorkspaceID: p.Owner.WorkspaceID,
					Profile:     profile,
					SessionName: p.Owner.Name,
					Orphaned:    !p.Owner.Live,
					Items:       []contract.SessionProcessPayload{},
				},
			)
		}
		response.Groups[i].Items = append(response.Groups[i].Items, SessionProcessPayload(p, snapshot.ScannedAt))
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) TerminateProcesses(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session process aggregate termination") || !h.requireSessionProcesses(c) {
		return
	}
	var req contract.TerminateProcessesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	targets, err := ProcessTargets(req.Targets, false)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	results, err := h.SessionProcesses.Terminate(
		c.Request.Context(),
		"",
		targets,
		false,
		func(sessionprocess.Owner) bool { return true },
	)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, ProcessResults(h.SessionProcesses.Summary("").Supported, results))
}

func SessionProcessPayload(p sessionprocess.Process, at time.Time) contract.SessionProcessPayload {
	ports := append([]int{}, p.Ports...)
	return contract.SessionProcessPayload{
		Attribution: contract.SessionProcessAttribution(p.Attribution),
		PID:         p.PID,
		PGID:        p.PGID,
		StartedAt:   p.StartedAt,
		Kind: contract.SessionProcessKind(
			p.Kind,
		),
		Command:     redact.String(cmp.Or(p.Command, filepath.Base(p.Argv0))),
		Argv0:       p.Argv0,
		CWD:         p.CWD,
		Ports:       ports,
		AgeSeconds:  max(0, at.Sub(p.StartedAt).Seconds()),
		ParentAlive: p.Kind == "tool",
		TerminalID:  p.TerminalID,
	}
}
func ProcessTargets(targets []contract.SessionProcessTarget, all bool) ([]sessionprocess.Identity, error) {
	if all == (len(targets) > 0) || len(targets) > 100 {
		return nil, errors.New("provide either all:true or 1–100 targets")
	}
	out := make([]sessionprocess.Identity, 0, len(targets))
	for _, t := range targets {
		if t.PID <= 1 || t.StartedAt.IsZero() {
			return nil, errors.New("targets require pid > 1 and started_at")
		}
		out = append(out, sessionprocess.Identity{PID: t.PID, StartedAt: t.StartedAt.UTC()})
	}
	return out, nil
}
func ProcessResults(supported bool, results []sessionprocess.Result) contract.TerminateSessionProcessesResponse {
	response := contract.TerminateSessionProcessesResponse{
		Supported: supported,
		Results:   []contract.SessionProcessResult{},
	}
	for _, r := range results {
		response.Results = append(
			response.Results,
			contract.SessionProcessResult{
				PID:       r.PID,
				StartedAt: r.StartedAt,
				Outcome:   contract.SessionProcessOutcome(r.Outcome),
				Error:     r.Error,
			},
		)
	}
	return response
}
func (h *BaseHandlers) decorateProcessSummaries(payloads []contract.SessionPayload) {
	if h.SessionProcesses == nil {
		return
	}
	for i := range payloads {
		s := h.SessionProcesses.Summary(payloads[i].ID)
		payloads[i].ProcessSummary = &contract.SessionProcessSummaryPayload{
			Supported:  s.Supported,
			Background: s.Background,
			Terminals:  s.Terminals,
			Total:      s.Background + s.Terminals,
			Ports:      s.Ports,
		}
	}
}
