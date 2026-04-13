package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const sdkName = "github.com/compozy/compozy/sdk/extension"

type rawHookHandler func(context.Context, HookContext, json.RawMessage) (json.RawMessage, error)

type registeredHook struct {
	mutable bool
	handler rawHookHandler
}

// EventHandler handles one forwarded Compozy event.
type EventHandler func(context.Context, Event) error

// RawHookHandler handles one hook using the raw JSON payload.
type RawHookHandler func(context.Context, HookContext, json.RawMessage) (json.RawMessage, error)

// HealthCheckHandler overrides the default health check result.
type HealthCheckHandler func(context.Context, HealthCheckRequest) (HealthCheckResponse, error)

// ShutdownHandler overrides the default graceful shutdown behavior.
type ShutdownHandler func(context.Context, ShutdownRequest) error

// ReviewProviderHandler serves one registered executable review provider.
type ReviewProviderHandler interface {
	FetchReviews(context.Context, ReviewProviderContext, FetchRequest) ([]ReviewItem, error)
	ResolveIssues(context.Context, ReviewProviderContext, ResolveIssuesRequest) error
}

// ReviewProvider adapts plain functions to ReviewProviderHandler.
type ReviewProvider struct {
	FetchReviewsFunc  func(context.Context, ReviewProviderContext, FetchRequest) ([]ReviewItem, error)
	ResolveIssuesFunc func(context.Context, ReviewProviderContext, ResolveIssuesRequest) error
}

// FetchReviews implements ReviewProviderHandler.
func (p ReviewProvider) FetchReviews(
	ctx context.Context,
	reviewCtx ReviewProviderContext,
	req FetchRequest,
) ([]ReviewItem, error) {
	if p.FetchReviewsFunc == nil {
		return nil, fmt.Errorf("review provider %q is missing FetchReviews", reviewCtx.Provider)
	}
	return p.FetchReviewsFunc(ctx, reviewCtx, req)
}

// ResolveIssues implements ReviewProviderHandler.
func (p ReviewProvider) ResolveIssues(
	ctx context.Context,
	reviewCtx ReviewProviderContext,
	req ResolveIssuesRequest,
) error {
	if p.ResolveIssuesFunc == nil {
		return fmt.Errorf("review provider %q is missing ResolveIssues", reviewCtx.Provider)
	}
	return p.ResolveIssuesFunc(ctx, reviewCtx, req)
}

type eventRegistration struct {
	kinds   map[EventKind]struct{}
	handler EventHandler
}

func (r eventRegistration) wantsAll() bool {
	return len(r.kinds) == 0
}

func (r eventRegistration) matches(kind EventKind) bool {
	if len(r.kinds) == 0 {
		return true
	}
	_, ok := r.kinds[kind]
	return ok
}

type callResult struct {
	message Message
	err     error
}

// Extension is the public Go SDK runtime used by executable extension authors.
type Extension struct {
	name    string
	version string

	sdkVersion string
	transport  Transport
	host       *HostAPI

	mu                   sync.RWMutex
	started              bool
	initialized          bool
	draining             bool
	initializeRequest    InitializeRequest
	initializeResponse   InitializeResponse
	acceptedCapabilities map[Capability]struct{}
	capabilities         []Capability

	hooks           map[HookName]registeredHook
	reviewProviders map[string]ReviewProviderHandler
	eventHandlers   []eventRegistration

	healthHandler   HealthCheckHandler
	shutdownHandler ShutdownHandler

	requestID atomic.Uint64

	pendingMu sync.Mutex
	pending   map[string]chan callResult

	done    chan struct{}
	doneErr error
}

// New constructs a new extension runtime with the provided identity.
func New(name string, version string) *Extension {
	ext := &Extension{
		name:            strings.TrimSpace(name),
		version:         strings.TrimSpace(version),
		sdkVersion:      strings.TrimSpace(version),
		hooks:           make(map[HookName]registeredHook),
		reviewProviders: make(map[string]ReviewProviderHandler),
		pending:         make(map[string]chan callResult),
		done:            make(chan struct{}),
	}
	ext.host = newHostAPI(ext)
	return ext
}

// WithCapabilities declares the capabilities this extension requires.
func (e *Extension) WithCapabilities(capabilities ...Capability) *Extension {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.capabilities = mergeCapabilities(e.capabilities, capabilities)
	return e
}

// WithTransport overrides the transport used by Start.
func (e *Extension) WithTransport(transport Transport) *Extension {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.transport = transport
	return e
}

// WithSDKVersion overrides the sdk_version reported during initialize.
func (e *Extension) WithSDKVersion(version string) *Extension {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.sdkVersion = strings.TrimSpace(version)
	return e
}

// Handle registers a raw hook handler for one hook event.
func (e *Extension) Handle(event HookName, handler RawHookHandler) *Extension {
	if e == nil || handler == nil {
		return e
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	normalized := normalizeHookName(event)
	if normalized == "" {
		return e
	}
	e.hooks[normalized] = registeredHook{
		mutable: hookIsMutable(normalized),
		handler: rawHookHandler(handler),
	}
	return e
}

// OnEvent registers one forwarded event handler with an optional filter.
func (e *Extension) OnEvent(handler EventHandler, kinds ...EventKind) *Extension {
	if e == nil || handler == nil {
		return e
	}

	filter := make(map[EventKind]struct{}, len(kinds))
	for _, kind := range kinds {
		trimmed := EventKind(strings.TrimSpace(string(kind)))
		if trimmed == "" {
			continue
		}
		filter[trimmed] = struct{}{}
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.eventHandlers = append(e.eventHandlers, eventRegistration{
		kinds:   filter,
		handler: handler,
	})
	return e
}

// OnHealthCheck overrides the default healthy response.
func (e *Extension) OnHealthCheck(handler HealthCheckHandler) *Extension {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.healthHandler = handler
	return e
}

// OnShutdown overrides the default graceful shutdown behavior.
func (e *Extension) OnShutdown(handler ShutdownHandler) *Extension {
	if e == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.shutdownHandler = handler
	return e
}

// RegisterReviewProvider registers one executable review provider handler by name.
func (e *Extension) RegisterReviewProvider(name string, handler ReviewProviderHandler) *Extension {
	if e == nil || handler == nil {
		return e
	}

	normalized := strings.TrimSpace(name)
	if normalized == "" {
		return e
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.reviewProviders[normalized] = handler
	return e
}

// Host returns the Host API client bound to the current extension session.
func (e *Extension) Host() *HostAPI {
	if e == nil {
		return nil
	}
	return e.host
}

// AcceptedCapabilities returns the negotiated capability list after initialize.
func (e *Extension) AcceptedCapabilities() []Capability {
	if e == nil {
		return nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	values := make([]Capability, 0, len(e.acceptedCapabilities))
	for capability := range e.acceptedCapabilities {
		values = append(values, capability)
	}
	sortCapabilities(values)
	return values
}

// InitializeRequest returns the last initialize request processed by the
// extension, when available.
func (e *Extension) InitializeRequest() (InitializeRequest, bool) {
	if e == nil {
		return InitializeRequest{}, false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.initialized {
		return InitializeRequest{}, false
	}
	return e.initializeRequest, true
}

// Start serves the extension over the configured transport until shutdown or
// transport termination.
func (e *Extension) Start(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("start extension: missing extension")
	}
	if strings.TrimSpace(e.name) == "" {
		return fmt.Errorf("start extension: name is required")
	}
	if strings.TrimSpace(e.version) == "" {
		return fmt.Errorf("start extension: version is required")
	}

	e.mu.Lock()
	if e.started {
		err := e.doneErr
		e.mu.Unlock()
		if err != nil {
			return err
		}
		return fmt.Errorf("start extension: already started")
	}
	e.started = true
	if e.transport == nil {
		e.transport = NewStdIOTransport(os.Stdin, os.Stdout)
	}
	transport := e.transport
	e.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}

	readErr := make(chan error, 1)
	go func() {
		readErr <- e.readLoop()
	}()

	select {
	case err := <-readErr:
		e.finish(err)
		return err
	case <-ctx.Done():
		_ = transport.Close()
		err := contextError(ctx)
		<-readErr
		e.finish(err)
		return err
	case <-e.done:
		return e.doneErr
	}
}

func (e *Extension) readLoop() error {
	for {
		message, err := e.transport.ReadMessage()
		if err != nil {
			return err
		}
		if len(message.ID) == 0 {
			continue
		}

		if strings.TrimSpace(message.Method) == "" {
			e.resolvePending(message)
			continue
		}

		if !e.isInitialized() {
			if strings.TrimSpace(message.Method) != "initialize" {
				if err := e.writeError(message.ID, newNotInitializedError()); err != nil {
					return err
				}
				continue
			}
			if err := e.handleInitialize(message); err != nil {
				return err
			}
			continue
		}

		switch strings.TrimSpace(message.Method) {
		case "shutdown":
			return e.handleShutdown(message)
		default:
			go e.handleRequest(message)
		}
	}
}

func (e *Extension) handleInitialize(message Message) error {
	var request InitializeRequest
	if err := unmarshalJSON(message.Params, &request); err != nil {
		requestErr := newInvalidParamsError(map[string]any{
			"method": "initialize",
			"error":  err.Error(),
		})
		if writeErr := e.writeError(message.ID, requestErr); writeErr != nil {
			return writeErr
		}
		return requestErr
	}

	response, err := e.buildInitializeResponse(request)
	if err != nil {
		requestErr := toRPCError(err)
		if writeErr := e.writeError(message.ID, requestErr); writeErr != nil {
			return writeErr
		}
		return err
	}

	if err := e.writeResult(message.ID, response); err != nil {
		return err
	}

	e.mu.Lock()
	e.initialized = true
	e.initializeRequest = request
	e.initializeResponse = response
	e.acceptedCapabilities = make(map[Capability]struct{}, len(response.AcceptedCapabilities))
	for _, capability := range response.AcceptedCapabilities {
		e.acceptedCapabilities[capability] = struct{}{}
	}
	e.mu.Unlock()

	go e.subscribeFilteredEvents()
	return nil
}

func (e *Extension) buildInitializeResponse(request InitializeRequest) (InitializeResponse, error) {
	selectedVersion := negotiateProtocolVersion(request)
	if selectedVersion == "" {
		return InitializeResponse{}, newInvalidParamsError(map[string]any{
			"reason":                      "unsupported_protocol_version",
			"requested":                   strings.TrimSpace(request.ProtocolVersion),
			"supported_protocol_versions": []string{ProtocolVersion},
		})
	}

	required := e.requiredCapabilities()
	granted := capabilitySetFromSlice(request.GrantedCapabilities)
	missing := make([]Capability, 0)
	for _, capability := range required {
		if _, ok := granted[capability]; ok {
			continue
		}
		missing = append(missing, capability)
	}
	if len(missing) > 0 {
		sortCapabilities(missing)
		return InitializeResponse{}, newCapabilityDeniedError("initialize", missing, request.GrantedCapabilities)
	}

	response := InitializeResponse{
		ProtocolVersion: selectedVersion,
		ExtensionInfo: InitializeResponseInfo{
			Name:       e.name,
			Version:    e.version,
			SDKName:    sdkName,
			SDKVersion: e.reportedSDKVersion(),
		},
		AcceptedCapabilities:      required,
		SupportedHookEvents:       e.supportedHookEvents(),
		RegisteredReviewProviders: e.registeredReviewProviders(),
		Supports: Supports{
			HealthCheck: true,
			OnEvent:     true,
		},
	}
	return response, nil
}

func (e *Extension) handleShutdown(message Message) error {
	e.mu.Lock()
	e.draining = true
	e.mu.Unlock()

	var request ShutdownRequest
	if err := unmarshalJSON(message.Params, &request); err != nil {
		requestErr := newInvalidParamsError(map[string]any{
			"method": "shutdown",
			"error":  err.Error(),
		})
		if writeErr := e.writeError(message.ID, requestErr); writeErr != nil {
			return writeErr
		}
		return requestErr
	}

	if handler := e.currentShutdownHandler(); handler != nil {
		if err := handler(context.Background(), request); err != nil {
			requestErr := toRPCError(err)
			if writeErr := e.writeError(message.ID, requestErr); writeErr != nil {
				return writeErr
			}
			return err
		}
	}

	if err := e.writeResult(message.ID, ShutdownResponse{Acknowledged: true}); err != nil {
		return err
	}
	return nil
}

func (e *Extension) handleRequest(message Message) {
	if e.isDraining() {
		if err := e.writeError(message.ID, newShutdownInProgressError(0)); err != nil {
			e.finish(err)
		}
		return
	}

	var err error
	switch strings.TrimSpace(message.Method) {
	case "execute_hook":
		err = e.handleExecuteHook(message)
	case "on_event":
		err = e.handleOnEvent(message)
	case "health_check":
		err = e.handleHealthCheck(message)
	case "fetch_reviews":
		err = e.handleFetchReviews(message)
	case "resolve_issues":
		err = e.handleResolveIssues(message)
	default:
		err = e.writeError(message.ID, newMethodNotFoundError(message.Method))
	}
	if err != nil {
		e.finish(err)
	}
}

func (e *Extension) handleExecuteHook(message Message) error {
	var request ExecuteHookRequest
	if err := unmarshalJSON(message.Params, &request); err != nil {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"method": "execute_hook",
			"error":  err.Error(),
		}))
	}

	hook := normalizeHookName(request.Hook.Event)
	handler, ok := e.hookHandler(hook)
	if !ok {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"reason": "unsupported_hook_event",
			"hook":   request.Hook.Event,
		}))
	}
	if err := e.checkCapabilityForHook(hook); err != nil {
		return e.writeError(message.ID, err)
	}

	patch, err := handler(context.Background(), HookContext{
		InvocationID: request.InvocationID,
		Hook:         request.Hook,
		Host:         e.host,
	}, request.Payload)
	if err != nil {
		return e.writeError(message.ID, toRPCError(err))
	}
	return e.writeResult(message.ID, ExecuteHookResponse{Patch: patch})
}

func (e *Extension) handleOnEvent(message Message) error {
	if err := e.checkCapability(CapabilityEventsRead, "on_event"); err != nil {
		return e.writeError(message.ID, err)
	}

	var request OnEventRequest
	if err := unmarshalJSON(message.Params, &request); err != nil {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"method": "on_event",
			"error":  err.Error(),
		}))
	}

	for _, registration := range e.currentEventHandlers() {
		if !registration.matches(request.Event.Kind) {
			continue
		}
		if err := registration.handler(context.Background(), request.Event); err != nil {
			return e.writeError(message.ID, toRPCError(err))
		}
	}
	return e.writeResult(message.ID, map[string]any{})
}

func (e *Extension) handleHealthCheck(message Message) error {
	var request HealthCheckRequest
	if err := unmarshalJSON(message.Params, &request); err != nil {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"method": "health_check",
			"error":  err.Error(),
		}))
	}

	response := HealthCheckResponse{
		Healthy: true,
		Details: map[string]any{
			"active_requests": len(e.currentPending()),
			"queue_depth":     0,
		},
	}
	if handler := e.currentHealthHandler(); handler != nil {
		overridden, err := handler(context.Background(), request)
		if err != nil {
			return e.writeError(message.ID, toRPCError(err))
		}
		response = overridden
		if response.Details == nil {
			response.Details = map[string]any{}
		}
		if _, ok := response.Details["active_requests"]; !ok {
			response.Details["active_requests"] = len(e.currentPending())
		}
		if _, ok := response.Details["queue_depth"]; !ok {
			response.Details["queue_depth"] = 0
		}
	}

	return e.writeResult(message.ID, response)
}

func (e *Extension) handleFetchReviews(message Message) error {
	if err := e.checkCapability(CapabilityProvidersRegister, "fetch_reviews"); err != nil {
		return e.writeError(message.ID, err)
	}

	var request struct {
		Provider string `json:"provider"`
		FetchRequest
	}
	if err := unmarshalJSON(message.Params, &request); err != nil {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"method": "fetch_reviews",
			"error":  err.Error(),
		}))
	}

	handler, ok := e.reviewProvider(request.Provider)
	if !ok {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"reason":   "unsupported_review_provider",
			"provider": request.Provider,
		}))
	}

	items, err := handler.FetchReviews(context.Background(), ReviewProviderContext{
		Provider: strings.TrimSpace(request.Provider),
		Host:     e.host,
	}, request.FetchRequest)
	if err != nil {
		return e.writeError(message.ID, toRPCError(err))
	}
	return e.writeResult(message.ID, items)
}

func (e *Extension) handleResolveIssues(message Message) error {
	if err := e.checkCapability(CapabilityProvidersRegister, "resolve_issues"); err != nil {
		return e.writeError(message.ID, err)
	}

	var request struct {
		Provider string `json:"provider"`
		ResolveIssuesRequest
	}
	if err := unmarshalJSON(message.Params, &request); err != nil {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"method": "resolve_issues",
			"error":  err.Error(),
		}))
	}

	handler, ok := e.reviewProvider(request.Provider)
	if !ok {
		return e.writeError(message.ID, newInvalidParamsError(map[string]any{
			"reason":   "unsupported_review_provider",
			"provider": request.Provider,
		}))
	}

	if err := handler.ResolveIssues(context.Background(), ReviewProviderContext{
		Provider: strings.TrimSpace(request.Provider),
		Host:     e.host,
	}, request.ResolveIssuesRequest); err != nil {
		return e.writeError(message.ID, toRPCError(err))
	}
	return e.writeResult(message.ID, map[string]any{})
}

func (e *Extension) resolvePending(message Message) {
	e.pendingMu.Lock()
	ch, ok := e.pending[string(message.ID)]
	e.pendingMu.Unlock()
	if !ok {
		return
	}
	ch <- callResult{message: message}
}

func (e *Extension) call(ctx context.Context, method string, params any, result any) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !e.isInitialized() {
		return newNotInitializedError()
	}
	if e.isDraining() {
		return newShutdownInProgressError(0)
	}
	if err := e.checkCapabilityForHostMethod(method); err != nil {
		return err
	}

	requestID := strconv.FormatUint(e.requestID.Add(1), 10)
	paramsRaw, err := marshalJSON(params)
	if err != nil {
		return err
	}

	responseCh := make(chan callResult, 1)
	e.pendingMu.Lock()
	e.pending[requestID] = responseCh
	e.pendingMu.Unlock()
	defer e.removePending(requestID)

	if err := e.transport.WriteMessage(Message{
		ID:     json.RawMessage(requestID),
		Method: method,
		Params: paramsRaw,
	}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return newInternalError(map[string]any{"error": contextError(ctx).Error()})
	case response := <-responseCh:
		if response.err != nil {
			return response.err
		}
		if response.message.Error != nil {
			return response.message.Error
		}
		if result == nil {
			return nil
		}
		return unmarshalJSON(response.message.Result, result)
	}
}

func (e *Extension) subscribeFilteredEvents() {
	registrations := e.currentEventHandlers()
	if len(registrations) == 0 {
		return
	}
	if !e.hasAcceptedCapability(CapabilityEventsRead) {
		return
	}

	kinds, filtered := unionEventKinds(registrations)
	if !filtered {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if _, err := e.host.Events.Subscribe(ctx, EventSubscribeRequest{Kinds: kinds}); err != nil {
		e.finish(err)
	}
}

func (e *Extension) supportedHookEvents() []HookName {
	e.mu.RLock()
	defer e.mu.RUnlock()

	hooks := make([]HookName, 0, len(e.hooks))
	for hook := range e.hooks {
		hooks = append(hooks, hook)
	}
	sort.Slice(hooks, func(i, j int) bool { return string(hooks[i]) < string(hooks[j]) })
	return hooks
}

func (e *Extension) registeredReviewProviders() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	names := make([]string, 0, len(e.reviewProviders))
	for name := range e.reviewProviders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (e *Extension) requiredCapabilities() []Capability {
	e.mu.RLock()
	defer e.mu.RUnlock()

	required := mergeCapabilities(nil, e.capabilities)
	for hook := range e.hooks {
		capability := capabilityForHook(hook)
		if capability != "" {
			required = mergeCapabilities(required, []Capability{capability})
		}
	}
	if len(e.eventHandlers) > 0 {
		required = mergeCapabilities(required, []Capability{CapabilityEventsRead})
	}
	return required
}

func (e *Extension) hookHandler(hook HookName) (rawHookHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	registered, ok := e.hooks[hook]
	return registered.handler, ok
}

func (e *Extension) currentEventHandlers() []eventRegistration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	handlers := make([]eventRegistration, 0, len(e.eventHandlers))
	for _, registration := range e.eventHandlers {
		kinds := make(map[EventKind]struct{}, len(registration.kinds))
		for kind := range registration.kinds {
			kinds[kind] = struct{}{}
		}
		handlers = append(handlers, eventRegistration{kinds: kinds, handler: registration.handler})
	}
	return handlers
}

func (e *Extension) currentHealthHandler() HealthCheckHandler {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.healthHandler
}

func (e *Extension) currentShutdownHandler() ShutdownHandler {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.shutdownHandler
}

func (e *Extension) reviewProvider(name string) (ReviewProviderHandler, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	handler, ok := e.reviewProviders[strings.TrimSpace(name)]
	return handler, ok
}

func (e *Extension) currentPending() map[string]chan callResult {
	e.pendingMu.Lock()
	defer e.pendingMu.Unlock()

	copied := make(map[string]chan callResult, len(e.pending))
	for key, value := range e.pending {
		copied[key] = value
	}
	return copied
}

func (e *Extension) removePending(id string) {
	e.pendingMu.Lock()
	defer e.pendingMu.Unlock()
	delete(e.pending, id)
}

func (e *Extension) finish(err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.doneErr != nil || isClosed(e.done) {
		return
	}
	e.doneErr = err
	close(e.done)
}

func (e *Extension) isInitialized() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.initialized
}

func (e *Extension) isDraining() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.draining
}

func (e *Extension) hasAcceptedCapability(capability Capability) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.acceptedCapabilities[capability]
	return ok
}

func (e *Extension) reportedSDKVersion() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if strings.TrimSpace(e.sdkVersion) != "" {
		return strings.TrimSpace(e.sdkVersion)
	}
	return strings.TrimSpace(e.version)
}

func (e *Extension) writeResult(id json.RawMessage, result any) error {
	raw, err := marshalJSON(result)
	if err != nil {
		return err
	}
	return e.transport.WriteMessage(Message{ID: append(json.RawMessage(nil), id...), Result: raw})
}

func (e *Extension) writeError(id json.RawMessage, requestErr *Error) error {
	return e.transport.WriteMessage(Message{ID: append(json.RawMessage(nil), id...), Error: requestErr})
}

func (e *Extension) checkCapability(capability Capability, target string) *Error {
	if capability == "" || e.hasAcceptedCapability(capability) {
		return nil
	}
	return newCapabilityDeniedError(target, []Capability{capability}, e.AcceptedCapabilities())
}

func (e *Extension) checkCapabilityForHook(hook HookName) *Error {
	return e.checkCapability(capabilityForHook(hook), string(hook))
}

func (e *Extension) checkCapabilityForHostMethod(method string) *Error {
	capability, ok := hostMethodCapabilities[strings.TrimSpace(method)]
	if !ok {
		return newMethodNotFoundError(method)
	}
	return e.checkCapability(capability, method)
}

func negotiateProtocolVersion(request InitializeRequest) string {
	for _, candidate := range request.SupportedProtocolVersions {
		if strings.TrimSpace(candidate) == ProtocolVersion {
			return ProtocolVersion
		}
	}
	if strings.TrimSpace(request.ProtocolVersion) == ProtocolVersion {
		return ProtocolVersion
	}
	return ""
}

func normalizeHookName(hook HookName) HookName {
	return HookName(strings.TrimSpace(string(hook)))
}

func hookIsMutable(hook HookName) bool {
	switch hook {
	case HookAgentPostSessionCreate,
		HookAgentOnSessionUpdate,
		HookAgentPostSessionEnd,
		HookJobPostExecute,
		HookRunPostStart,
		HookRunPreShutdown,
		HookRunPostShutdown,
		HookReviewPostFix,
		HookArtifactPostWrite:
		return false
	default:
		return true
	}
}

func capabilityForHook(hook HookName) Capability {
	switch {
	case strings.HasPrefix(string(hook), "plan."):
		return CapabilityPlanMutate
	case strings.HasPrefix(string(hook), "prompt."):
		return CapabilityPromptMutate
	case strings.HasPrefix(string(hook), "agent."):
		return CapabilityAgentMutate
	case strings.HasPrefix(string(hook), "job."):
		return CapabilityJobMutate
	case strings.HasPrefix(string(hook), "run."):
		return CapabilityRunMutate
	case strings.HasPrefix(string(hook), "review."):
		return CapabilityReviewMutate
	case strings.HasPrefix(string(hook), "artifact."):
		return CapabilityArtifactsWrite
	default:
		return ""
	}
}

var hostMethodCapabilities = map[string]Capability{
	"host.events.subscribe": CapabilityEventsRead,
	"host.events.publish":   CapabilityEventsPublish,
	"host.tasks.list":       CapabilityTasksRead,
	"host.tasks.get":        CapabilityTasksRead,
	"host.tasks.create":     CapabilityTasksCreate,
	"host.runs.start":       CapabilityRunsStart,
	"host.artifacts.read":   CapabilityArtifactsRead,
	"host.artifacts.write":  CapabilityArtifactsWrite,
	"host.prompts.render":   "",
	"host.memory.read":      CapabilityMemoryRead,
	"host.memory.write":     CapabilityMemoryWrite,
}

func mergeCapabilities(base []Capability, extra []Capability) []Capability {
	set := make(map[Capability]struct{}, len(base)+len(extra))
	merged := make([]Capability, 0, len(base)+len(extra))
	for _, capability := range append(append([]Capability(nil), base...), extra...) {
		trimmed := Capability(strings.TrimSpace(string(capability)))
		if trimmed == "" {
			continue
		}
		if _, ok := set[trimmed]; ok {
			continue
		}
		set[trimmed] = struct{}{}
		merged = append(merged, trimmed)
	}
	sortCapabilities(merged)
	return merged
}

func capabilitySetFromSlice(values []Capability) map[Capability]struct{} {
	set := make(map[Capability]struct{}, len(values))
	for _, value := range values {
		trimmed := Capability(strings.TrimSpace(string(value)))
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func sortCapabilities(values []Capability) {
	sort.Slice(values, func(i, j int) bool { return string(values[i]) < string(values[j]) })
}

func unionEventKinds(registrations []eventRegistration) ([]EventKind, bool) {
	filtered := true
	union := make(map[EventKind]struct{})
	for _, registration := range registrations {
		if registration.wantsAll() {
			filtered = false
			break
		}
		for kind := range registration.kinds {
			union[kind] = struct{}{}
		}
	}
	if !filtered {
		return nil, false
	}

	kinds := make([]EventKind, 0, len(union))
	for kind := range union {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool { return string(kinds[i]) < string(kinds[j]) })
	return kinds, true
}

func newCapabilityDeniedError(target string, missing []Capability, granted []Capability) *Error {
	return newRPCError(-32001, "Capability denied", map[string]any{
		"method":   strings.TrimSpace(target),
		"required": append([]Capability(nil), missing...),
		"granted":  append([]Capability(nil), granted...),
	})
}

func newNotInitializedError() *Error {
	return newRPCError(-32003, "Not initialized", map[string]any{
		"allowed_methods": []string{"initialize"},
	})
}

func newShutdownInProgressError(deadlineMS int64) *Error {
	return newRPCError(-32004, "Shutdown in progress", map[string]any{
		"deadline_ms": deadlineMS,
	})
}

func toRPCError(err error) *Error {
	if err == nil {
		return nil
	}
	var requestErr *Error
	if ok := errorAs(err, &requestErr); ok && requestErr != nil {
		return requestErr
	}
	return newInternalError(map[string]any{"error": err.Error()})
}

func errorAs(err error, target **Error) bool {
	if err == nil || target == nil {
		return false
	}
	value, ok := err.(*Error)
	if !ok {
		return false
	}
	*target = value
	return true
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
