package core

import (
	"cmp"
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/computers"
	"github.com/compozy/compozy/internal/computers/sshsetup"
	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/store"
)

type ComputersService interface {
	LocalSettings() config.ComputersConfig
	PatchLocalSettings(context.Context, computers.LocalSettingsPatch) (config.ComputersConfig, error)
	List(context.Context) ([]computers.View, error)
	Available(context.Context) ([]remoteproto.AccountComputer, error)
	Get(context.Context, remoteproto.ComputerID) (computers.View, error)
	AddFromAccount(context.Context, remoteproto.ComputerID) (computers.View, error)
	Update(context.Context, remoteproto.ComputerID, store.ComputerPatch) (computers.View, error)
	Remove(context.Context, remoteproto.ComputerID, computers.RemoveOptions) (string, error)
	RemoveRoute(context.Context, remoteproto.ComputerID, string) error
	Subscribe(context.Context, string) (computers.Subscription, error)
	RoundTrip(context.Context, remoteproto.ComputerID, *http.Request) (*http.Response, error)
}

var _ ComputersService = (*computers.Registry)(nil)

func (h *BaseHandlers) ListComputers(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	result, err := ComputerList(c.Request.Context(), h.Computers)
	h.computerResult(c, http.StatusOK, result, err)
}

func ComputerList(ctx context.Context, service ComputersService) (contract.ComputerListResponse, error) {
	views, err := service.List(ctx)
	rows := make([]contract.Computer, 0, len(views))
	for _, view := range views {
		rows = append(rows, ComputerDTO(view))
	}
	return contract.ComputerListResponse{Computers: rows, Local: localComputerDTO(service.LocalSettings())}, err
}

func (h *BaseHandlers) PatchLocalComputer(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	var input contract.PatchLocalComputerRequest
	if err := decodeStrictJSONBody(c, &input); err != nil || input == (contract.PatchLocalComputerRequest{}) {
		invalidComputerBody(c)
		return
	}
	config, err := h.Computers.PatchLocalSettings(c.Request.Context(), computers.LocalSettingsPatch{
		RunAgents: input.RunAgents, LoadBalancing: input.LoadBalancing, Weight: input.LoadBalancingWeight,
	})
	h.computerResult(c, http.StatusOK, localComputerDTO(config), err)
}

func localComputerDTO(cfg config.ComputersConfig) contract.LocalComputer {
	return contract.LocalComputer{
		RunAgents:           cfg.LocalRunAgents,
		LoadBalancing:       cfg.LoadBalancing,
		LoadBalancingWeight: cfg.LocalLoadBalancingWeight,
	}
}

func (h *BaseHandlers) AvailableComputers(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	results, err := h.Computers.Available(c.Request.Context())
	rows := make([]contract.AvailableComputer, 0, len(results))
	for _, item := range results {
		rows = append(rows, contract.AvailableComputer{ID: item.ID, Name: item.Name, Online: item.Online})
	}
	h.computerResult(c, http.StatusOK, contract.AvailableComputersResponse{Computers: rows}, err)
}

func (h *BaseHandlers) GetComputer(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	view, err := h.Computers.Get(c.Request.Context(), computerID(c))
	h.computerResult(c, http.StatusOK, ComputerDTO(view), err)
}

func (h *BaseHandlers) AddComputer(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	var input contract.AddComputerRequest
	if err := decodeStrictJSONBody(c, &input); err != nil {
		invalidComputerBody(c)
		return
	}
	if input.Kind == contract.ComputerAddKindSSH && input.ComputerID == "" {
		h.addSSHComputer(c, input)
		return
	}
	if input.Kind != contract.ComputerAddKindAccount || input.ComputerID == "" {
		invalidComputerBody(c)
		return
	}
	view, err := h.Computers.AddFromAccount(c.Request.Context(), input.ComputerID)
	h.computerResult(c, http.StatusCreated, ComputerDTO(view), err)
}

func (h *BaseHandlers) PatchComputer(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	var input contract.PatchComputerRequest
	if err := decodeStrictJSONBody(c, &input); err != nil {
		invalidComputerBody(c)
		return
	}
	view, err := h.Computers.Update(c.Request.Context(), computerID(c), store.ComputerPatch(input))
	h.computerResult(c, http.StatusOK, ComputerDTO(view), err)
}

func (h *BaseHandlers) RemoveComputer(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	unlink, unlinkErr := parseBoolQuery(c, "unlink")
	uninstall, uninstallErr := parseBoolQuery(c, "uninstall")
	if unlinkErr != nil || uninstallErr != nil {
		invalidComputerBody(c)
		return
	}
	result, err := h.Computers.Remove(c.Request.Context(), computerID(c), computers.RemoveOptions{
		Unlink: unlink, Uninstall: uninstall,
	})
	h.computerResult(c, http.StatusOK, contract.RemoveComputerResponse{Removed: err == nil, Uninstall: result}, err)
}

func (h *BaseHandlers) RemoveComputerRoute(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	err := h.Computers.RemoveRoute(c.Request.Context(), computerID(c), c.Param("route_id"))
	h.computerResult(c, http.StatusNoContent, nil, err)
}

func (h *BaseHandlers) StreamComputers(c *gin.Context) {
	if !h.computersReady(c) {
		return
	}
	after := cmp.Or(c.GetHeader("Last-Event-ID"), c.Query("after_sequence"))
	sub, err := h.Computers.Subscribe(c.Request.Context(), after)
	if err != nil {
		h.computerResult(c, http.StatusOK, nil, err)
		return
	}
	defer sub.Close()
	serveAccountEvents(c, h, sub.Done, sub.Events, sub.Replay, nil, func(event computers.Event) SSEMessage {
		change := contract.ComputerChange{ComputerID: event.Data.ComputerID, Removed: event.Data.Removed}
		if event.Data.Setup != nil {
			setup := ComputerSetupDTO(*event.Data.Setup)
			change.SetupID, change.Step, change.State = setup.SetupID, setup.Step, setup.State
			change.Prompt, change.Error, change.Answered = setup.Prompt, setup.Error, event.Data.Setup.Answered
			change.ComputerID = event.Data.Setup.ComputerID
		}
		if event.Data.Computer != nil {
			change.Computer = new(ComputerDTO(*event.Data.Computer))
		}
		return SSEMessage{ID: event.ID, Name: event.Event, Data: contract.ComputerEvent{
			ID: event.ID, Event: event.Event, Data: change,
		}}
	})
}

func (h *BaseHandlers) computersReady(c *gin.Context) bool {
	if h.Computers != nil {
		c.Header("Cache-Control", "no-store")
		return true
	}
	c.AbortWithStatusJSON(http.StatusServiceUnavailable,
		contract.ErrorPayload{Code: "computers_unavailable", Error: "Computers are unavailable."})
	return false
}

func computerID(c *gin.Context) remoteproto.ComputerID {
	return remoteproto.ComputerID(c.Param("id"))
}

func invalidComputerBody(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusBadRequest,
		contract.ErrorPayload{Code: "invalid_computer_request", Error: "Check the computer request and try again."})
}

func (h *BaseHandlers) computerResult(c *gin.Context, status int, body any, err error) {
	if fault, ok := errors.AsType[*sshsetup.Failure](err); ok {
		c.AbortWithStatusJSON(fault.HTTPStatus, setupErrorDTO(fault))
		return
	}
	if fault, ok := errors.AsType[*computers.Failure](err); ok {
		c.AbortWithStatusJSON(fault.Status, contract.ErrorPayload{Code: fault.Code, Error: fault.Message,
			Details: fault.Details})
		return
	}
	h.accountResult(c, status, body, err)
}

func ComputerDTO(view computers.View) contract.Computer {
	result := contract.Computer{
		ID: view.ID, AccountID: view.AccountID, Name: view.Name, Enabled: view.Enabled,
		Status: string(view.Status), PlanState: view.PlanState, Version: view.Version, Platform: view.Platform,
		Protocol: view.Protocol, Scopes: view.Scopes, LoadBalancingWeight: view.Weight,
		AddedAt: view.AddedAt, LastConnectedAt: view.LastConnAt,
		Routes: make([]contract.ComputerRoute, 0, len(view.Routes)),
	}
	if result.Scopes == nil {
		result.Scopes = []remoteproto.Scope{}
	}
	if route := view.RouteInUse; route != nil {
		result.RouteInUse = &contract.ComputerRouteInUse{ID: route.ID, Kind: route.Kind, Label: route.Address}
	}
	if view.Diagnosis != nil {
		result.Diagnosis = new(contract.ComputerDiagnosis(*view.Diagnosis))
	}
	for _, route := range view.Routes {
		result.Routes = append(result.Routes, contract.ComputerRoute{
			ID: route.ID, Kind: route.Kind, Address: route.Address, Learned: route.Learned,
			LastError: optionalAccountString(view.RouteErrors[route.ID]),
			InUse:     result.RouteInUse != nil && result.RouteInUse.ID == route.ID,
		})
	}
	return result
}
