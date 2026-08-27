package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

type createTerminalRequest = contract.TerminalCreateRequest
type closeTerminalRequest = contract.TerminalCloseRequest
type attachTicketRequest = contract.TerminalAttachTicketRequest

func (h *BaseHandlers) CreateTerminal(c *gin.Context) {
	service, profileID, ok := h.terminalService(c, true)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	actor, ok := h.terminalActor(c, workspaceID, profileID, "terminal.open")
	if !ok {
		return
	}
	var request createTerminalRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	actor, err := h.bindTerminalHumanClient(c, workspaceID, profileID, request.ClientID, actor)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	capabilities, err := h.terminalCapabilities(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondTerminalMappedError(c, StatusForWorkspaceError(err), "terminal_workspace_unavailable", err)
		return
	}
	handle, err := service.Open(c.Request.Context(), terminalpkg.OpenRequest{
		WS: workspaceID, Cwd: request.Cwd, Shell: request.Shell,
		Cols: request.Cols, Rows: request.Rows, Title: terminalpkg.SanitizeTitle(request.Title),
		Actor: actor, Capabilities: capabilities,
	})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	payload, err := h.terminalInfoPayload(c, handle.Info())
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{terminalPayloadKey: payload})
}

func (h *BaseHandlers) ListTerminals(c *gin.Context) {
	service, scope, ok := h.terminalAggregateService(c)
	if !ok {
		return
	}
	items, err := service.List(c.Request.Context(), strings.TrimSpace(c.Param("workspace_id")), scope)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	payloads, err := h.terminalInfoPayloads(c, items)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"terminals": payloads})
}

func (h *BaseHandlers) GetTerminal(c *gin.Context) {
	service, profileID, ok := h.terminalService(c, false)
	if !ok {
		return
	}
	info, err := service.Get(
		c.Request.Context(), strings.TrimSpace(c.Param("workspace_id")), profileID,
		terminalpkg.ID(strings.TrimSpace(c.Param("id"))),
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	payload, err := h.terminalInfoPayload(c, *info)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{terminalPayloadKey: payload})
}

func (h *BaseHandlers) DeleteTerminal(c *gin.Context) {
	service, profileID, ok := h.terminalService(c, true)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	actor, ok := h.terminalActor(c, workspaceID, profileID, "terminal.close")
	if !ok {
		return
	}
	request := closeTerminalRequest{Signal: terminalpkg.SignalHUP}
	if err := decodeOptionalTerminalJSON(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	if !validTerminalSignal(request.Signal) {
		h.respondTerminalError(c, terminalRequestError(errors.New("signal must be INT, TERM, KILL, or HUP")))
		return
	}
	exit, err := service.Close(
		c.Request.Context(), workspaceID, terminalpkg.ID(strings.TrimSpace(c.Param("id"))), actor, request.Signal,
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"exit": terminalExitFromDomain(exit)})
}

func (h *BaseHandlers) MintTerminalAttachTicket(c *gin.Context) {
	service, profileID, ok := h.terminalService(c, true)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	terminalID := terminalpkg.ID(strings.TrimSpace(c.Param("id")))
	actor, ok := h.terminalActor(c, workspaceID, profileID, "terminal.attach")
	if !ok {
		return
	}
	var request attachTicketRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	if request.Mode != contract.TerminalAttachModeRead && request.Mode != contract.TerminalAttachModeWrite {
		h.respondTerminalError(
			c,
			&terminalpkg.Error{
				Code:    "terminal_attach_mode_invalid",
				Message: "terminal attach mode must be read or write",
				Err:     terminalpkg.ErrUnsupported,
			},
		)
		return
	}
	actor, err := h.bindTerminalHumanClient(c, workspaceID, profileID, request.ClientID, actor)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	info, err := service.Get(c.Request.Context(), workspaceID, profileID, terminalID)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	maxSubscribers := h.Config.Terminal.MaxSubscribers
	if maxSubscribers > 0 && info.Viewers >= maxSubscribers {
		h.respondTerminalError(c, &terminalpkg.Error{
			Code: "subscriber_limit_reached", Message: "terminal subscriber limit reached",
			Current: info.Viewers, Max: maxSubscribers, Err: terminalpkg.ErrSubscriberLimit,
		})
		return
	}
	if h.terminalTickets == nil {
		h.respondTerminalUnavailable(c)
		return
	}
	ticket, err := h.terminalTickets.Mint(terminalTicketBinding{
		WorkspaceID: workspaceID, ProfileID: profileID, TerminalID: terminalID, Mode: string(request.Mode),
	}, actor)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.JSON(http.StatusCreated, gin.H{"ticket": ticket.Token, "expires_at": ticket.ExpiresAt})
}

func decodeOptionalTerminalJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (h *BaseHandlers) terminalInfoPayload(c *gin.Context, info terminalpkg.Info) (terminalInfoPayload, error) {
	identities, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		return terminalInfoPayload{}, err
	}
	return terminalInfoFromDomain(info, identities[info.ProfileID].Name), nil
}

func (h *BaseHandlers) terminalInfoPayloads(c *gin.Context, infos []terminalpkg.Info) ([]terminalInfoPayload, error) {
	identities, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		return nil, err
	}
	payloads := make([]terminalInfoPayload, 0, len(infos))
	for _, info := range infos {
		payloads = append(payloads, terminalInfoFromDomain(info, identities[info.ProfileID].Name))
	}
	return payloads, nil
}

func (h *BaseHandlers) terminalListForProfile(
	c *gin.Context,
	service terminalpkg.Manager,
	workspaceID, profileID string,
) ([]terminalInfoPayload, error) {
	infos, err := service.List(c.Request.Context(), workspaceID, store.ReadScope{ProfileID: profileID})
	if err != nil {
		return nil, err
	}
	return h.terminalInfoPayloads(c, infos)
}
