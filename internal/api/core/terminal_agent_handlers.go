package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/gin-gonic/gin"
)

type execTerminalRequest = contract.TerminalExecRequest
type waitTerminalRequest = contract.TerminalWaitRequest
type signalTerminalRequest = contract.TerminalSignalRequest
type answerTerminalInputRequest = contract.TerminalAnswerInputRequest
type rejectTerminalInputRequest = contract.TerminalRejectInputRequest
type terminalRecordingRequest = contract.TerminalRecordingRequest

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
	result, err := service.Exec(c.Request.Context(), terminalpkg.ExecRequest{
		WS: workspaceID, Command: request.Command, Args: request.Args, Cwd: request.Cwd,
		Env: request.Env, YieldMs: request.YieldMs, Visible: request.Visible,
		Output: terminalpkg.OutputShape{
			MaxBytes: request.Output.MaxBytes,
			Strategy: request.Output.Strategy,
			Grep:     request.Output.Grep,
		},
		Actor: actor,
	})
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	status := http.StatusOK
	if result.StillRunning {
		status = http.StatusAccepted
	}
	c.JSON(status, contract.TerminalExecResponseFromDomain(*result))
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
	c.JSON(http.StatusOK, contract.TerminalReadResponseFromDomain(*result))
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
				Code:    terminalpkg.ErrorCodeTimeoutOutOfRange,
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
	c.JSON(http.StatusOK, contract.TerminalWaitResponseFromDomain(*result))
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
	if !validTerminalSignal(terminalpkg.Signal(request.Signal)) {
		h.respondTerminalError(c, terminalRequestError(errors.New("signal must be INT, TERM, KILL, or HUP")))
		return
	}
	if err := handle.Signal(c.Request.Context(), actor, terminalpkg.Signal(request.Signal)); err != nil {
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
	requests, err := service.InputRequests(
		c.Request.Context(),
		strings.TrimSpace(c.Param("workspace_id")),
		scope,
		terminalpkg.ID(strings.TrimSpace(c.Query("terminal_id"))),
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	resolved, err := service.ResolvedInputRequests(
		c.Request.Context(),
		strings.TrimSpace(c.Param("workspace_id")),
		scope,
		terminalpkg.ID(strings.TrimSpace(c.Query("terminal_id"))),
	)
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	response, err := contract.TerminalInputRequestsResponseFromDomain(requests, resolved)
	if err != nil {
		h.respondTerminalMappedError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, response)
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
	pending, err := handle.PendingInput(requestID)
	if err != nil {
		h.respondTerminalError(c, err)
		return
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
	c.JSON(http.StatusOK, gin.H{"delivered_bytes": outcome.DeliveredBytes, "redacted": pending.Redacted})
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
	c.JSON(http.StatusOK, contract.TerminalInputRejectResponse{
		Outcome: contract.TerminalInputRejectOutcomeRejected,
	})
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
		state     contract.TerminalRecordingState
		err       error
	)
	switch request.Action {
	case contract.TerminalRecordingActionStart:
		recording, err = handle.StartRecording(c.Request.Context(), actor)
		state = contract.TerminalRecordingStateRecording
	case contract.TerminalRecordingActionStop:
		recording, err = handle.StopRecording(c.Request.Context(), actor)
		state = contract.TerminalRecordingStateSaved
	default:
		err = terminalRequestError(errors.New("recording action must be start or stop"))
	}
	if err != nil {
		h.respondTerminalError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.TerminalRecordingResponse{
		Recording: contract.TerminalRecordingPayloadFromDomain(recording, state),
	})
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
		h.respondTerminalMappedError(c, http.StatusInternalServerError, err)
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
