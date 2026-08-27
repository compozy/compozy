package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

type execTerminalRequest = contract.TerminalExecRequest
type waitTerminalRequest = contract.TerminalWaitRequest
type signalTerminalRequest = contract.TerminalSignalRequest
type answerTerminalInputRequest = contract.TerminalAnswerInputRequest
type rejectTerminalInputRequest = contract.TerminalRejectInputRequest
type terminalRecordingRequest = contract.TerminalRecordingRequest

type terminalInputRequestLister interface {
	InputRequests(context.Context, string, store.ReadScope, terminalpkg.ID) ([]terminalpkg.PendingInputRequest, error)
}

type terminalInputRequestInspector interface {
	PendingInput(terminalpkg.InputRequestID) (*terminalpkg.PendingInputRequest, error)
}

func (h *BaseHandlers) ExecTerminal(c *gin.Context) {
	service, profileID, ok := h.terminalService(c, true)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	actor, ok := h.terminalActor(c, workspaceID, profileID, "terminal.exec")
	if !ok {
		return
	}
	var request execTerminalRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	capabilities, err := h.terminalCapabilities(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondError(c, StatusForWorkspaceError(err), err)
		return
	}
	result, err := service.Exec(c.Request.Context(), terminalpkg.ExecRequest{
		WS: workspaceID, Command: request.Command, Args: request.Args, Cwd: request.Cwd,
		Env: request.Env, YieldMs: request.YieldMs, Visible: request.Visible, Output: request.Output,
		Actor: actor, Capabilities: capabilities,
	})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	status := http.StatusOK
	if result.StillRunning {
		status = http.StatusAccepted
	}
	c.JSON(status, result)
}

func (h *BaseHandlers) ReadTerminal(c *gin.Context) {
	handle, _, ok := h.terminalHandle(c, false, "terminal.read")
	if !ok {
		return
	}
	maxBytes, err := ParseOptionalInt(c.Query("max_bytes"))
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	sinceSeq, err := parseTerminalUint(c.Query("since_seq"), 64)
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	from, err := ParseOptionalInt(c.Query("from"))
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	to, err := ParseOptionalInt(c.Query("to"))
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	result, err := handle.Screen(c.Request.Context(), terminalpkg.ReadOptions{
		View: c.Query("view"), MaxBytes: maxBytes, SinceSeq: sinceSeq,
		FromLine: from, ToLine: to, Grep: c.Query("grep"),
	})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) WaitTerminal(c *gin.Context) {
	handle, _, ok := h.terminalHandle(c, false, "terminal.wait")
	if !ok {
		return
	}
	var request waitTerminalRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	if request.TimeoutMs > 60_000 {
		h.respondTerminalError(
			c,
			&terminalpkg.Error{
				Code:    "timeout_out_of_range",
				Message: "terminal wait timeout_ms must not exceed 60000",
				Err:     terminalpkg.ErrUnsupported,
			},
		)
		return
	}
	result, err := handle.Wait(
		c.Request.Context(),
		terminalpkg.WaitCondition{Until: request.Until, Pattern: request.Pattern, TimeoutMs: request.TimeoutMs},
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) SignalTerminal(c *gin.Context) {
	handle, actor, ok := h.terminalHandle(c, true, "terminal.signal")
	if !ok {
		return
	}
	var request signalTerminalRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	if !validTerminalSignal(request.Signal) {
		h.respondTerminalError(c, terminalRequestError(errors.New("signal must be INT, TERM, KILL, or HUP")))
		return
	}
	if err := handle.Signal(c.Request.Context(), actor, request.Signal); err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivered": true})
}

func (h *BaseHandlers) ListTerminalInputRequests(c *gin.Context) {
	service, scope, ok := h.terminalAggregateService(c)
	if !ok {
		return
	}
	lister, ok := service.(terminalInputRequestLister)
	if !ok {
		h.respondTerminalUnavailable(c)
		return
	}
	requests, err := lister.InputRequests(
		c.Request.Context(),
		strings.TrimSpace(c.Param("workspace_id")),
		scope,
		terminalpkg.ID(strings.TrimSpace(c.Query("terminal_id"))),
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requests": requests})
}

func (h *BaseHandlers) AnswerTerminalInputRequest(c *gin.Context) {
	handle, actor, ok := h.terminalHandle(c, true, "terminal.input.answer")
	if !ok {
		return
	}
	var request answerTerminalInputRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	requestID := terminalpkg.InputRequestID(strings.TrimSpace(c.Param("request_id")))
	redacted := false
	if inspector, available := handle.(terminalInputRequestInspector); available {
		pending, err := inspector.PendingInput(requestID)
		if err != nil {
			h.respondTerminalError(c, err)
			return
		}
		redacted = pending.Redacted
	}
	outcome, err := handle.AnswerInput(
		c.Request.Context(),
		actor,
		requestID,
		terminalpkg.InputAnswer{Input: []byte(request.Input)},
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivered_bytes": outcome.Length, "redacted": redacted})
}

func (h *BaseHandlers) RejectTerminalInputRequest(c *gin.Context) {
	handle, actor, ok := h.terminalHandle(c, true, "terminal.input.reject")
	if !ok {
		return
	}
	var request rejectTerminalInputRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	if err := handle.RejectInput(
		c.Request.Context(),
		actor,
		terminalpkg.InputRequestID(strings.TrimSpace(c.Param("request_id"))),
		request.Reason,
	); err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"outcome": "rejected"})
}

func (h *BaseHandlers) ControlTerminalRecording(c *gin.Context) {
	handle, actor, ok := h.terminalHandle(c, true, "terminal.record")
	if !ok {
		return
	}
	var request terminalRecordingRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	var (
		recording terminalpkg.RecordingRef
		state     string
		err       error
	)
	switch request.Action {
	case taskActionStart:
		recording, err = handle.StartRecording(c.Request.Context(), actor)
		state = "recording"
	case "stop":
		recording, err = handle.StopRecording(c.Request.Context(), actor)
		state = "saved"
	default:
		err = terminalRequestError(errors.New("recording action must be start or stop"))
	}
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	recording.State = state
	c.JSON(http.StatusOK, gin.H{"recording": recording})
}

func (h *BaseHandlers) QueryTerminalJournal(c *gin.Context) {
	service, scope, ok := h.terminalAggregateService(c)
	if !ok {
		return
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	failed, err := ParseOptionalBool(c.Query("failed"))
	if err != nil {
		h.respondTerminalError(c, terminalRequestError(err))
		return
	}
	page, err := service.Journal().
		Query(c.Request.Context(), strings.TrimSpace(c.Param("workspace_id")), scope, terminalpkg.Query{
			Actor: c.Query("actor"), Since: c.Query("since"), Terminal: c.Query("terminal_id"),
			Failed: failed, Limit: limit, Cursor: c.Query("cursor"),
		})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	identities, err := h.profileOwnerIdentities(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	entries := make([]contract.TerminalCommandRowPayload, 0, len(page.Entries))
	for _, row := range page.Entries {
		entries = append(entries, contract.TerminalCommandRowPayloadFromDomain(row, identities[row.ProfileID].Name))
	}
	next := any(nil)
	if page.Next != "" {
		next = page.Next
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "next": next})
}

func (h *BaseHandlers) terminalHandle(
	c *gin.Context,
	mutation bool,
	action string,
) (terminalpkg.Handle, terminalpkg.Actor, bool) {
	service, profileID, ok := h.terminalService(c, mutation)
	if !ok {
		return nil, terminalpkg.Actor{}, false
	}
	workspaceID := strings.TrimSpace(c.Param("workspace_id"))
	actor, ok := h.terminalActor(c, workspaceID, profileID, action)
	if !ok {
		return nil, terminalpkg.Actor{}, false
	}
	handle, err := service.Handle(
		c.Request.Context(),
		workspaceID,
		profileID,
		terminalpkg.ID(strings.TrimSpace(c.Param("id"))),
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return nil, terminalpkg.Actor{}, false
	}
	return handle, actor, true
}

func validTerminalSignal(signal terminalpkg.Signal) bool {
	switch signal {
	case terminalpkg.SignalINT, terminalpkg.SignalTERM, terminalpkg.SignalKILL, terminalpkg.SignalHUP:
		return true
	default:
		return false
	}
}
