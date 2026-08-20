package compozysdk

import (
	"context"
	"encoding/json"
	"strings"
)

func (t *StdioTransport) sendCancel(id json.RawMessage) {
	go func() {
		if err := t.writeFrame(requestFrame{
			JSONRPC: JSONRPCVersion,
			Method:  jsonRPCCancelMethod,
			Params:  cancelRequestParams{ID: id},
		}); err != nil {
			t.fail(wrapTransportError("transport: send cancel", err))
		}
	}()
}

func (t *StdioTransport) startRequest(parent context.Context, request JSONRPCRequestEnvelope) {
	requestCtx, cancel := context.WithCancel(parent)
	key := pendingKey(request.ID)
	t.mu.Lock()
	previous := t.inbound[key]
	t.inbound[key] = cancel
	t.mu.Unlock()
	if previous != nil {
		previous()
	}
	go t.dispatchRequest(requestCtx, request)
}

func (t *StdioTransport) dispatchRequest(ctx context.Context, request JSONRPCRequestEnvelope) {
	key := pendingKey(request.ID)
	defer t.releaseInbound(key)
	t.mu.Lock()
	handler := t.handlers[strings.TrimSpace(request.Method)]
	t.mu.Unlock()
	if handler == nil {
		t.sendError(request.ID, NewMethodNotFoundError(request.Method))
		return
	}
	result, err := handler(ctx, cloneRawMessage(request.Params), request)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		t.sendError(request.ID, ensureRPCError(err))
		return
	}
	t.sendResult(request.ID, result)
}

func (t *StdioTransport) cancelInbound(raw json.RawMessage) {
	var params cancelRequestParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return
	}
	t.mu.Lock()
	cancel := t.inbound[pendingKey(params.ID)]
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (t *StdioTransport) releaseInbound(key string) {
	t.mu.Lock()
	cancel := t.inbound[key]
	delete(t.inbound, key)
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
