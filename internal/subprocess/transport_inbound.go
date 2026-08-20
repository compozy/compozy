package subprocess

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/compozy/compozy/internal/redact"
)

func (t *transport) handleRequest(envelope rpcEnvelope) {
	if envelope.Method == jsonRPCCancel {
		t.handleCancel(envelope.Params)
		return
	}
	if len(envelope.ID) == 0 {
		return
	}

	id, err := parseRPCID(envelope.ID)
	if err != nil {
		t.sendErrorOrFail(
			envelope.ID,
			NewRPCError(codeInvalidRequest, "Invalid request", map[string]string{"reason": err.Error()}),
			"subprocess: send invalid request error",
		)
		return
	}

	switch t.process.currentState() {
	case processStateStarting:
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeNotInitialized, "Not initialized", nil),
			"subprocess: send not initialized error",
		)
		return
	case processStateDraining:
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeShutdownProgress, "Shutdown in progress", nil),
			"subprocess: send shutdown-in-progress error",
		)
		return
	case processStateStopped:
		return
	}

	t.handlersMu.RLock()
	handler, ok := t.handlers[envelope.Method]
	t.handlersMu.RUnlock()
	if !ok {
		t.sendErrorOrFail(
			id.raw,
			NewRPCError(codeMethodNotFound, "Method not found", map[string]string{transportMethodKey: envelope.Method}),
			"subprocess: send method-not-found error",
		)
		return
	}

	requestCtx, cancel := context.WithCancel(t.process.lifecycleCtx)
	token := t.inboundSeq.Add(1)
	t.inboundMu.Lock()
	previous := t.inbound[id.key]
	t.inbound[id.key] = inboundCancel{token: token, cancel: cancel}
	t.inboundMu.Unlock()
	if previous.cancel != nil {
		previous.cancel()
	}

	t.handlerWG.Go(func() {
		defer t.releaseInbound(id.key, token, cancel)
		result, callErr := handler(requestCtx, envelope.Params)
		if requestCtx.Err() != nil {
			return
		}
		if callErr != nil {
			if rpcErr, ok := errors.AsType[*RPCError](callErr); ok {
				t.sendErrorOrFail(id.raw, rpcErr, "subprocess: send handler rpc error")
				return
			}
			t.sendErrorOrFail(
				id.raw,
				NewRPCError(codeInternalError, "Internal error", map[string]string{
					transportErrorKey: redact.ClaimTokens(callErr.Error()),
				}),
				"subprocess: send internal error",
			)
			return
		}
		t.sendResultOrFail(id.raw, result, "subprocess: send result")
	})
}

func (t *transport) handleCancel(raw json.RawMessage) {
	var params rpcCancelParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return
	}
	id, err := parseRPCID(params.ID)
	if err != nil {
		return
	}
	t.inboundMu.Lock()
	entry := t.inbound[id.key]
	t.inboundMu.Unlock()
	if entry.cancel != nil {
		entry.cancel()
	}
}

type inboundCancel struct {
	token  uint64
	cancel context.CancelFunc
}

func (t *transport) releaseInbound(key string, token uint64, cancel context.CancelFunc) {
	t.inboundMu.Lock()
	current, ok := t.inbound[key]
	if ok && current.token == token {
		delete(t.inbound, key)
	}
	t.inboundMu.Unlock()
	cancel()
}

func (t *transport) cancelInbound() {
	t.inboundMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(t.inbound))
	for key, entry := range t.inbound {
		delete(t.inbound, key)
		cancels = append(cancels, entry.cancel)
	}
	t.inboundMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}
