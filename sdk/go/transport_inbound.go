package compozysdk

import (
	"context"
	"encoding/json"
	"strings"
)

type inboundCancel struct {
	cancel context.CancelFunc
}

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
	entry := &inboundCancel{cancel: cancel}
	t.mu.Lock()
	previous := t.inbound[key]
	t.inbound[key] = entry
	t.mu.Unlock()
	if previous != nil {
		previous.cancel()
	}
	go t.dispatchRequest(requestCtx, request, entry)
}

func (t *StdioTransport) dispatchRequest(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	entry *inboundCancel,
) {
	key := pendingKey(request.ID)
	defer t.releaseInbound(key, entry)
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
	entry := t.inbound[pendingKey(params.ID)]
	t.mu.Unlock()
	if entry != nil {
		entry.cancel()
	}
}

func (t *StdioTransport) releaseInbound(key string, entry *inboundCancel) {
	t.mu.Lock()
	if t.inbound[key] == entry {
		delete(t.inbound, key)
	}
	t.mu.Unlock()
	entry.cancel()
}
