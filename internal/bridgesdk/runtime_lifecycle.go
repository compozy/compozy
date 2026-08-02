package bridgesdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/compozy/compozy/internal/subprocess"
)

func (r *Runtime) handleInitialize(ctx context.Context, raw json.RawMessage) (any, error) {
	if r == nil {
		return nil, errors.New("bridgesdk: runtime is required")
	}

	var request subprocess.InitializeRequest
	if err := decodeParams(raw, &request); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: err.Error(),
		})
	}
	if request.Runtime.Bridge == nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: "initialize bridge runtime is required",
		})
	}

	r.mu.Lock()
	if r.session != nil || r.initializing {
		r.mu.Unlock()
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInternal, "Internal error", map[string]string{
			runtimeErrorKey: "provider runtime already initialized",
		})
	}
	peer := r.peer
	r.initializing = true
	r.mu.Unlock()

	control := request.Runtime.Bridge.Purpose == subprocess.BridgeRuntimePurposeControl
	if control {
		if err := r.validateControlHandler(request.Runtime.Bridge); err != nil {
			r.mu.Lock()
			r.initializing = false
			r.mu.Unlock()
			return nil, err
		}
	}
	var host *HostAPIClient
	if !control {
		host = NewHostAPIClient(peer)
	}
	cache := NewInstanceCache(request.Runtime.Bridge)
	response := r.initializeResponse(request)
	session := &Session{
		request:  request,
		response: response,
		host:     host,
		cache:    cache,
		now:      r.config.Now,
	}

	if !control && r.config.Initialize != nil {
		if err := r.config.Initialize(ctx, session); err != nil {
			r.mu.Lock()
			r.initializing = false
			r.mu.Unlock()
			return nil, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		r.initializing = false
		return nil, err
	}
	r.session = session
	r.initializing = false
	return response, nil
}

func (r *Runtime) handleShutdown(ctx context.Context, raw json.RawMessage) (any, error) {
	session, err := r.requireSession()
	if err != nil {
		return nil, err
	}

	var request subprocess.ShutdownRequest
	trimmedRaw := bytes.TrimSpace(raw)
	if len(trimmedRaw) > 0 && !bytes.Equal(trimmedRaw, []byte("null")) {
		if err := decodeParams(raw, &request); err != nil {
			return nil, err
		}
	}

	shouldRun, err := r.beginShutdown()
	if err != nil {
		return nil, err
	}
	if shouldRun {
		var shutdownErr error
		if !sessionIsControl(session) && r.config.Shutdown != nil {
			shutdownErr = r.config.Shutdown(ctx, session, request)
		}
		r.completeShutdown(shutdownErr)
		if shutdownErr != nil {
			return nil, shutdownErr
		}
	}

	return subprocess.ShutdownResponse{Acknowledged: true}, nil
}

func (r *Runtime) beginShutdown() (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch r.shutdownState {
	case runtimeShutdownSucceeded:
		return false, nil
	case runtimeShutdownRunning:
		return false, subprocess.NewRPCError(bridgeSDKRPCCodeShutdownRunning, "Shutdown running", nil)
	default:
		r.shutdownState = runtimeShutdownRunning
		return true, nil
	}
}

func (r *Runtime) completeShutdown(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.shutdownState = runtimeShutdownIdle
		return
	}
	r.shutdownState = runtimeShutdownSucceeded
}

func (r *Runtime) requireSession() (*Session, error) {
	if r == nil {
		return nil, errors.New("bridgesdk: runtime is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.session == nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeNotInitialized, "Not initialized", nil)
	}
	return r.session, nil
}
