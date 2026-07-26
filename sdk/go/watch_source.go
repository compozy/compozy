package aghsdk

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
)

const extensionKindKey = "kind"

type registeredWatchSource struct {
	kind    string
	handler func(context.Context, rawWatchSourceRequest) (WatchPollResponse, error)
}

type rawWatchSourceRequest struct {
	kind                string
	spec                json.RawMessage
	expectedStateDigest string
	context             ExtensionContext
}

// WatchSource registers a raw JSON watch-source handler on the extension.
func (e *Extension) WatchSource(
	kind string,
	options WatchSourceOptions,
	fn WatchSourceHandlerFunc[json.RawMessage],
) error {
	return WatchSource[json.RawMessage](e, kind, options, fn)
}

// WatchSource registers a typed Go function as a loop watch-source handler.
func WatchSource[TSpec any](
	extension *Extension,
	kind string,
	options WatchSourceOptions,
	fn WatchSourceHandlerFunc[TSpec],
) error {
	if extension == nil {
		return NewInternalError("extension is required")
	}
	if fn == nil {
		return NewInvalidParamsError("watch source handler function is required", nil)
	}
	return extension.registerWatchSource(kind, options, func(
		ctx context.Context,
		req rawWatchSourceRequest,
	) (WatchPollResponse, error) {
		var spec TSpec
		rawSpec := req.spec
		if len(rawSpec) == 0 {
			rawSpec = json.RawMessage(`{}`)
		}
		if err := json.Unmarshal(rawSpec, &spec); err != nil {
			return WatchPollResponse{}, NewInvalidParamsError(
				"watch source spec does not match handler type",
				map[string]any{
					extensionKindKey:  req.kind,
					extensionErrorKey: err.Error(),
				},
			)
		}
		return fn(ctx, WatchSourceRequest[TSpec]{
			Kind:                req.kind,
			Spec:                spec,
			RawSpec:             cloneRawMessage(rawSpec),
			ExpectedStateDigest: req.expectedStateDigest,
			Context:             req.context,
			Host:                req.context.Host,
			Session:             req.context.Session,
		})
	})
}

// GetWatchSourceKinds returns the sorted watch-source kinds registered by WatchSource.
func (e *Extension) GetWatchSourceKinds() []string {
	if e == nil {
		return nil
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.watchSourceKindsLocked()
}

func (e *Extension) registerWatchSource(
	kind string,
	_ WatchSourceOptions,
	fn func(context.Context, rawWatchSourceRequest) (WatchPollResponse, error),
) error {
	cleanKind := strings.TrimSpace(kind)
	if cleanKind == "" {
		return NewInvalidParamsError("watch source kind is required", nil)
	}
	if fn == nil {
		return NewInvalidParamsError("watch source handler function is required", nil)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	if err := e.ensureWatchSourceRegistrationAvailableLocked(cleanKind); err != nil {
		return err
	}
	e.watchHandlers[cleanKind] = registeredWatchSource{kind: cleanKind, handler: fn}
	e.definition.Capabilities.Provides = normalizeStrings(
		append(e.definition.Capabilities.Provides, CapabilityProvideWatchSource),
	)
	e.transport.Handle(ExtensionServiceMethodWatchPoll, e.dispatch)
	return nil
}

func (e *Extension) ensureWatchSourceRegistrationAvailableLocked(kind string) error {
	if _, ok := e.watchHandlers[kind]; ok {
		return NewInvalidParamsError("watch source already registered", map[string]any{extensionKindKey: kind})
	}
	if _, ok := e.handlers[ExtensionServiceMethodWatchPoll]; ok {
		return NewInvalidParamsError("watch/poll is reserved by WatchSource", nil)
	}
	return nil
}

func (e *Extension) handleWatchPoll(
	ctx context.Context,
	request JSONRPCRequestEnvelope,
	params json.RawMessage,
) (WatchPollResponse, error) {
	var poll WatchPollRequest
	if err := json.Unmarshal(params, &poll); err != nil {
		return WatchPollResponse{}, NewInvalidParamsError(
			"watch/poll params must be an object",
			map[string]any{extensionErrorKey: err.Error()},
		)
	}
	kind, err := watchSourceKind(poll.Spec)
	if err != nil {
		return WatchPollResponse{}, err
	}
	e.mu.RLock()
	registered, ok := e.watchHandlers[kind]
	e.mu.RUnlock()
	if !ok {
		return WatchPollResponse{}, NewInvalidParamsError(
			"watch source kind is not registered",
			map[string]any{extensionKindKey: kind},
		)
	}
	contextValue := e.makeContext(request)
	return registered.handler(ctx, rawWatchSourceRequest{
		kind:                registered.kind,
		spec:                cloneRawMessage(poll.Spec),
		expectedStateDigest: strings.TrimSpace(poll.ExpectedStateDigest),
		context:             contextValue,
	})
}

func (e *Extension) watchSourceKindsLocked() []string {
	if len(e.watchHandlers) == 0 {
		return nil
	}
	out := make([]string, 0, len(e.watchHandlers))
	for kind := range e.watchHandlers {
		out = append(out, kind)
	}
	slices.Sort(out)
	return out
}

func isWatchSourceMethod(method string) bool {
	return method == ExtensionServiceMethodWatchPoll
}

func watchSourceKind(spec json.RawMessage) (string, error) {
	if len(spec) == 0 || !json.Valid(spec) {
		return "", NewInvalidParamsError("watch source spec must be valid JSON", nil)
	}
	var payload struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(spec, &payload); err != nil {
		return "", NewInvalidParamsError(
			"watch source spec must be an object",
			map[string]any{extensionErrorKey: err.Error()},
		)
	}
	kind := strings.TrimSpace(payload.Kind)
	if kind == "" {
		return "", NewInvalidParamsError("watch source spec.kind is required", nil)
	}
	return kind, nil
}
