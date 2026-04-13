package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/core/subprocess"
	"github.com/compozy/compozy/internal/version"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

var errExtensionSessionClosed = errors.New("extension session closed")

type extensionSession struct {
	manager *Manager
	runtime *RuntimeExtension
	process *subprocess.Process

	transport *subprocess.Transport
	supports  initializeSupports

	defaultHookTimeout time.Duration
	healthInterval     time.Duration
	onEventQueue       chan events.Event

	requestID atomic.Uint64
	dropped   atomic.Uint64
	degraded  atomic.Bool

	pendingMu sync.Mutex
	pending   map[string]chan sessionCallResult

	closeOnce sync.Once
	done      chan struct{}

	closeMu  sync.Mutex
	closeErr error

	failureOnce sync.Once
	stopOnce    sync.Once
}

type sessionCallResult struct {
	message subprocess.Message
	err     error
}

type initializeRequest struct {
	ProtocolVersion           string                    `json:"protocol_version"`
	SupportedProtocolVersions []string                  `json:"supported_protocol_versions"`
	CompozyVersion            string                    `json:"compozy_version"`
	Extension                 initializeRequestIdentity `json:"extension"`
	GrantedCapabilities       []Capability              `json:"granted_capabilities,omitempty"`
	Runtime                   initializeRuntime         `json:"runtime"`
}

type initializeRequestIdentity struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

type initializeRuntime struct {
	RunID                 string `json:"run_id"`
	ParentRunID           string `json:"parent_run_id"`
	WorkspaceRoot         string `json:"workspace_root"`
	InvokingCommand       string `json:"invoking_command"`
	ShutdownTimeoutMS     int64  `json:"shutdown_timeout_ms"`
	DefaultHookTimeoutMS  int64  `json:"default_hook_timeout_ms"`
	HealthCheckIntervalMS int64  `json:"health_check_interval_ms"`
}

type initializeResponse struct {
	ProtocolVersion           string                 `json:"protocol_version"`
	ExtensionInfo             initializeResponseInfo `json:"extension_info"`
	AcceptedCapabilities      []Capability           `json:"accepted_capabilities,omitempty"`
	SupportedHookEvents       []HookName             `json:"supported_hook_events,omitempty"`
	RegisteredReviewProviders []string               `json:"registered_review_providers,omitempty"`
	Supports                  initializeSupports     `json:"supports"`
}

type initializeResponseInfo struct {
	Name       string `json:"name,omitempty"`
	Version    string `json:"version,omitempty"`
	SDKName    string `json:"sdk_name,omitempty"`
	SDKVersion string `json:"sdk_version,omitempty"`
}

type initializeSupports struct {
	HealthCheck bool `json:"health_check"`
	OnEvent     bool `json:"on_event"`
}

func (m *Manager) startExtension(ctx context.Context, extension *RuntimeExtension) error {
	if extension == nil {
		return fmt.Errorf("start extension: missing runtime extension")
	}
	if extension.Manifest == nil || extension.Manifest.Subprocess == nil {
		return fmt.Errorf("start extension %q: missing subprocess config", extension.normalizedName())
	}

	command, err := resolveExtensionCommand(extension)
	if err != nil {
		return m.failExtensionStart(
			ctx,
			extension,
			extensionFailurePhaseSpawn,
			fmt.Errorf("resolve extension command %q: %w", extension.normalizedName(), err),
		)
	}

	process, err := m.launchExtensionProcess(ctx, extension, command)
	if err != nil {
		return m.failExtensionStart(
			ctx,
			extension,
			extensionFailurePhaseSpawn,
			fmt.Errorf("launch extension %q: %w", extension.normalizedName(), err),
		)
	}

	session, response, err := m.initializeExtensionSession(ctx, extension, process)
	if err != nil {
		return err
	}
	return m.emitReadyEvent(ctx, extension, session, response)
}

func (m *Manager) launchExtensionProcess(
	ctx context.Context,
	extension *RuntimeExtension,
	command []string,
) (*subprocess.Process, error) {
	return subprocess.Launch(contextWithoutCancel(ctx), subprocess.LaunchConfig{
		Command:         command,
		Env:             extensionEnvironment(m, extension),
		WorkingDir:      extensionWorkingDir(extension, command),
		WaitDelay:       defaultShutdownTimeout(extension),
		WaitErrorPrefix: fmt.Sprintf("wait for extension process %q", extension.normalizedName()),
	})
}

func (m *Manager) initializeExtensionSession(
	ctx context.Context,
	extension *RuntimeExtension,
	process *subprocess.Process,
) (*extensionSession, initializeResponse, error) {
	transport := subprocess.NewTransport(process.Stdout(), process.Stdin())
	request := m.initializeRequestForExtension(extension)
	startedAt := time.Now()
	response, err := initializeExtensionTransport(ctx, transport, request)
	if err != nil {
		shutdownErr := process.Kill()
		combined := errors.Join(
			fmt.Errorf("initialize extension %q: %w%s", extension.normalizedName(), err, formatProcessStderr(process)),
			shutdownErr,
		)
		m.recordLifecycleAudit(extension, "initialize", time.Since(startedAt), combined)
		return nil, initializeResponse{}, m.failExtensionStart(
			ctx,
			extension,
			extensionFailurePhaseInitialize,
			combined,
		)
	}
	if err := validateInitializeContract(extension, request, response); err != nil {
		shutdownErr := process.Kill()
		combined := errors.Join(
			fmt.Errorf("validate initialize response for %q: %w", extension.normalizedName(), err),
			shutdownErr,
		)
		m.recordLifecycleAudit(extension, "initialize", time.Since(startedAt), combined)
		return nil, initializeResponse{}, m.failExtensionStart(
			ctx,
			extension,
			extensionFailurePhaseInitialize,
			combined,
		)
	}

	session := &extensionSession{
		manager:            m,
		runtime:            extension,
		process:            process,
		transport:          transport,
		supports:           response.Supports,
		defaultHookTimeout: durationFromMilliseconds(request.Runtime.DefaultHookTimeoutMS),
		healthInterval:     durationFromMilliseconds(request.Runtime.HealthCheckIntervalMS),
		onEventQueue:       make(chan events.Event, extensionEventQueueCap),
		pending:            make(map[string]chan sessionCallResult),
		done:               make(chan struct{}),
	}

	extension.Caller = session
	extension.Capabilities = NewCapabilityChecker(response.AcceptedCapabilities)
	extension.SetShutdownDeadline(durationFromMilliseconds(request.Runtime.ShutdownTimeoutMS))
	extension.SetDefaultHookTimeout(session.defaultHookTimeout)
	extension.SetState(ExtensionStateReady)

	m.registerSession(session)
	m.startSessionWorkers(session)
	m.recordLifecycleAudit(extension, "initialize", time.Since(startedAt), nil)

	return session, response, nil
}

func (m *Manager) startSessionWorkers(session *extensionSession) {
	m.backgroundGroup.Add(1)
	go session.readLoop()

	if session.supports.OnEvent && session.runtime.Capabilities.Has(CapabilityEventsRead) {
		m.backgroundGroup.Add(1)
		go m.runEventWorker(m.backgroundContext(), session)
	}
	if session.supports.HealthCheck && session.healthInterval > 0 {
		m.backgroundGroup.Add(1)
		go m.runHealthLoop(session)
	}
}

func (m *Manager) emitReadyEvent(
	ctx context.Context,
	extension *RuntimeExtension,
	session *extensionSession,
	response initializeResponse,
) error {
	if err := m.emitLifecycleEvent(ctx, events.EventKindExtensionReady, kinds.ExtensionReadyPayload{
		Extension:            extension.normalizedName(),
		Source:               string(extension.Ref.Source),
		Version:              extension.manifestVersion(),
		ProtocolVersion:      response.ProtocolVersion,
		AcceptedCapabilities: capabilityStrings(response.AcceptedCapabilities),
		SupportedHookEvents:  hookStrings(response.SupportedHookEvents),
	}); err != nil {
		shutdownErr := m.shutdownSession(context.Background(), session, shutdownReasonManagerError)
		return errors.Join(err, shutdownErr)
	}
	return nil
}

func (m *Manager) failExtensionStart(
	ctx context.Context,
	extension *RuntimeExtension,
	phase string,
	err error,
) error {
	if extension != nil {
		extension.SetState(ExtensionStateStopped)
	}
	return errors.Join(err, m.emitFailureEvent(ctx, extension, phase, err))
}

func (m *Manager) initializeRequestForExtension(extension *RuntimeExtension) initializeRequest {
	healthInterval := defaultHealthInterval(extension)
	shutdownTimeout := defaultShutdownTimeout(extension)

	return initializeRequest{
		ProtocolVersion:           compozyProtocolVersion(),
		SupportedProtocolVersions: []string{compozyProtocolVersion()},
		CompozyVersion:            strings.TrimSpace(version.Version),
		Extension: initializeRequestIdentity{
			Name:    extension.normalizedName(),
			Version: extension.manifestVersion(),
			Source:  string(extension.Ref.Source),
		},
		GrantedCapabilities: grantedCapabilities(extension),
		Runtime: initializeRuntime{
			RunID:                 strings.TrimSpace(m.runID),
			ParentRunID:           strings.TrimSpace(m.parentRunID),
			WorkspaceRoot:         strings.TrimSpace(m.workspaceRoot),
			InvokingCommand:       strings.TrimSpace(m.invokingCommand),
			ShutdownTimeoutMS:     shutdownTimeout.Milliseconds(),
			DefaultHookTimeoutMS:  defaultExtensionHookTimeout.Milliseconds(),
			HealthCheckIntervalMS: healthInterval.Milliseconds(),
		},
	}
}

func initializeExtensionTransport(
	ctx context.Context,
	transport *subprocess.Transport,
	request initializeRequest,
) (initializeResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if transport == nil {
		return initializeResponse{}, fmt.Errorf("initialize extension transport: missing transport")
	}

	params, err := json.Marshal(request)
	if err != nil {
		return initializeResponse{}, subprocess.NewInvalidParams(map[string]any{"error": err.Error()})
	}

	messageID := json.RawMessage("1")
	if err := transport.WriteMessage(subprocess.Message{
		ID:     &messageID,
		Method: "initialize",
		Params: params,
	}); err != nil {
		return initializeResponse{}, subprocess.NewInternalError(map[string]any{"error": err.Error()})
	}

	type result struct {
		message subprocess.Message
		err     error
	}
	readResult := make(chan result, 1)
	go func() {
		message, readErr := transport.ReadMessage()
		readResult <- result{message: message, err: readErr}
	}()

	select {
	case <-ctx.Done():
		return initializeResponse{}, context.Cause(ctx)
	case response := <-readResult:
		if response.err != nil {
			return initializeResponse{}, response.err
		}
		if response.message.Method != "" {
			return initializeResponse{}, subprocess.NewInvalidRequest(map[string]any{
				"reason": "unexpected_initialize_request",
				"method": response.message.Method,
			})
		}
		if response.message.Error != nil {
			return initializeResponse{}, response.message.Error
		}
		if response.message.ID == nil || string(*response.message.ID) != string(messageID) {
			return initializeResponse{}, subprocess.NewInvalidRequest(map[string]any{
				"reason": "unexpected_initialize_response_id",
			})
		}

		var decoded initializeResponse
		if err := json.Unmarshal(response.message.Result, &decoded); err != nil {
			return initializeResponse{}, subprocess.NewInternalError(map[string]any{"error": err.Error()})
		}
		return decoded, nil
	}
}

func validateInitializeContract(
	extension *RuntimeExtension,
	request initializeRequest,
	response initializeResponse,
) error {
	if extension == nil {
		return fmt.Errorf("validate initialize response: missing runtime extension")
	}

	if err := subprocess.ValidateInitializeResponse(
		subprocess.InitializeRequest{
			ProtocolVersion:           request.ProtocolVersion,
			SupportedProtocolVersions: append([]string(nil), request.SupportedProtocolVersions...),
			GrantedCapabilities:       capabilityStrings(request.GrantedCapabilities),
		},
		subprocess.InitializeResponse{
			ProtocolVersion:      response.ProtocolVersion,
			AcceptedCapabilities: capabilityStrings(response.AcceptedCapabilities),
		},
	); err != nil {
		return err
	}

	declaredHooks := manifestHookSet(extension)
	reportedHooks := make(map[HookName]struct{}, len(response.SupportedHookEvents))
	for _, hook := range response.SupportedHookEvents {
		normalized := HookName(strings.TrimSpace(string(hook)))
		if normalized == "" {
			continue
		}
		if !supportedHookNames.contains(normalized) {
			return subprocess.NewInvalidParams(map[string]any{
				"reason": "unsupported_hook_event",
				"hook":   normalized,
			})
		}
		reportedHooks[normalized] = struct{}{}
	}

	if !equalHookSets(declaredHooks, reportedHooks) {
		return subprocess.NewInvalidParams(map[string]any{
			"reason":               "unsupported_hook_contract",
			"declared_hook_events": sortedHookNames(declaredHooks),
			"reported_hook_events": sortedHookNames(reportedHooks),
		})
	}

	accepted := NewCapabilityChecker(response.AcceptedCapabilities)
	declaredReviewProviders := manifestDeclaredReviewProviders(extension)
	reportedReviewProviders := normalizeProviderNames(response.RegisteredReviewProviders)
	if len(declaredReviewProviders) > 0 && !accepted.Has(CapabilityProvidersRegister) {
		return subprocess.NewInvalidParams(map[string]any{
			"reason":                    "missing_provider_registration_capability",
			"declared_review_providers": sortedSetKeys(declaredReviewProviders),
			"accepted_capabilities":     capabilityStrings(response.AcceptedCapabilities),
		})
	}
	if !equalStringSets(declaredReviewProviders, reportedReviewProviders) {
		return subprocess.NewInvalidParams(map[string]any{
			"reason":                    "unsupported_review_provider_contract",
			"declared_review_providers": sortedSetKeys(declaredReviewProviders),
			"reported_review_providers": sortedSetKeys(reportedReviewProviders),
		})
	}
	if accepted.Has(CapabilityEventsRead) && !response.Supports.OnEvent {
		return subprocess.NewInvalidParams(map[string]any{
			"reason":                "inconsistent_event_contract",
			"accepted_capabilities": capabilityStrings(response.AcceptedCapabilities),
			"supports": map[string]bool{
				"on_event": response.Supports.OnEvent,
			},
		})
	}

	return nil
}

func manifestDeclaredReviewProviders(extension *RuntimeExtension) map[string]struct{} {
	reported := make(map[string]struct{})
	if extension == nil || extension.Manifest == nil {
		return reported
	}

	for i := range extension.Manifest.Providers.Review {
		entry := extension.Manifest.Providers.Review[i]
		if reviewProviderKind(entry) != ProviderKindExtension {
			continue
		}
		name := strings.TrimSpace(entry.Name)
		if name != "" {
			reported[name] = struct{}{}
		}
	}
	return reported
}

func normalizeProviderNames(values []string) map[string]struct{} {
	reported := make(map[string]struct{}, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name != "" {
			reported[name] = struct{}{}
		}
	}
	return reported
}

func equalStringSets(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for value := range left {
		if _, ok := right[value]; !ok {
			return false
		}
	}
	return true
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	slices.Sort(keys)
	return keys
}

func (s *extensionSession) Call(ctx context.Context, method string, params any, result any) error {
	if s == nil {
		return fmt.Errorf("call extension: missing session")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.closedErr(); err != nil {
		return err
	}

	requestID := strconv.FormatUint(s.requestID.Add(1), 10)
	requestIDRaw := json.RawMessage(requestID)
	paramsRaw, err := marshalResult(params)
	if err != nil {
		return err
	}

	responseCh := make(chan sessionCallResult, 1)
	s.pendingMu.Lock()
	s.pending[requestID] = responseCh
	s.pendingMu.Unlock()
	defer s.removePending(requestID)

	if err := s.transport.WriteMessage(subprocess.Message{
		ID:     &requestIDRaw,
		Method: strings.TrimSpace(method),
		Params: paramsRaw,
	}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		if cause := context.Cause(ctx); cause != nil {
			return cause
		}
		return ctx.Err()
	case <-s.done:
		return s.closedErr()
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
		if len(response.message.Result) == 0 {
			return nil
		}
		return json.Unmarshal(response.message.Result, result)
	}
}

func (s *extensionSession) readLoop() {
	defer s.manager.backgroundGroup.Done()

	err := s.readMessages()
	s.close(err)

	if s.runtime.State() == ExtensionStateDraining || s.runtime.State() == ExtensionStateStopped {
		return
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = fmt.Errorf("extension %q transport closed", s.runtime.normalizedName())
	}

	s.runtime.SetState(ExtensionStateStopped)
	s.failureOnce.Do(func() {
		if emitErr := s.manager.emitFailureEvent(
			context.Background(),
			s.runtime,
			extensionFailurePhaseTransport,
			err,
		); emitErr != nil {
			slog.Warn(
				"emit extension transport failure event",
				"component", "extension.manager",
				"extension", s.runtime.normalizedName(),
				"err", emitErr,
			)
		}
	})
}

func (s *extensionSession) readMessages() error {
	for {
		message, err := s.transport.ReadMessage()
		if err != nil {
			return err
		}

		switch {
		case message.Method != "":
			if err := s.handleIncomingRequest(message); err != nil {
				return err
			}
		case message.ID != nil:
			s.deliverResponse(message)
		}
	}
}

func (s *extensionSession) handleIncomingRequest(message subprocess.Message) error {
	if message.ID == nil {
		return nil
	}

	result, err := s.manager.hostAPI.Handle(
		context.Background(),
		s.runtime.normalizedName(),
		message.Method,
		message.Params,
	)
	response := subprocess.Message{ID: message.ID}
	if err != nil {
		response.Error = requestErrorFromError(err, message.Method)
	} else {
		response.Result, err = marshalResult(result)
		if err != nil {
			response.Error = subprocess.NewInternalError(map[string]any{
				"method": message.Method,
				"error":  err.Error(),
			})
		}
	}
	return s.transport.WriteMessage(response)
}

func (s *extensionSession) deliverResponse(message subprocess.Message) {
	if s == nil || message.ID == nil {
		return
	}

	requestID := string(*message.ID)
	s.pendingMu.Lock()
	responseCh, ok := s.pending[requestID]
	if ok {
		delete(s.pending, requestID)
	}
	s.pendingMu.Unlock()
	if !ok {
		return
	}

	responseCh <- sessionCallResult{message: message}
}

func (s *extensionSession) removePending(requestID string) {
	if s == nil {
		return
	}

	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pending, requestID)
}

func (s *extensionSession) close(err error) {
	s.closeOnce.Do(func() {
		if err == nil {
			err = errExtensionSessionClosed
		}

		s.closeMu.Lock()
		s.closeErr = err
		s.closeMu.Unlock()

		s.pendingMu.Lock()
		for requestID, responseCh := range s.pending {
			delete(s.pending, requestID)
			responseCh <- sessionCallResult{err: err}
		}
		s.pendingMu.Unlock()

		close(s.done)
	})
}

func (s *extensionSession) closedErr() error {
	if s == nil {
		return errExtensionSessionClosed
	}

	select {
	case <-s.done:
	default:
		return nil
	}

	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closeErr != nil {
		return s.closeErr
	}
	return errExtensionSessionClosed
}

func resolveExtensionCommand(extension *RuntimeExtension) ([]string, error) {
	if extension == nil || extension.Manifest == nil || extension.Manifest.Subprocess == nil {
		return nil, fmt.Errorf("missing subprocess config")
	}

	command := strings.TrimSpace(extension.Manifest.Subprocess.Command)
	if command == "" {
		return nil, fmt.Errorf("missing subprocess command")
	}
	if !filepath.IsAbs(command) &&
		(strings.HasPrefix(command, ".") || strings.ContainsRune(command, filepath.Separator)) {
		command = filepath.Join(extension.ExtensionDir, command)
	}

	args := append([]string{command}, extension.Manifest.Subprocess.Args...)
	return args, nil
}

func extensionWorkingDir(extension *RuntimeExtension, command []string) string {
	if extension != nil {
		dir := strings.TrimSpace(extension.ExtensionDir)
		if dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir
			}
		}
	}

	if len(command) == 0 {
		return ""
	}
	first := strings.TrimSpace(command[0])
	if filepath.IsAbs(first) {
		return filepath.Dir(first)
	}
	return ""
}

func extensionEnvironment(manager *Manager, extension *RuntimeExtension) []string {
	base := map[string]string{}
	if extension != nil && extension.Manifest != nil && extension.Manifest.Subprocess != nil {
		for key, value := range extension.Manifest.Subprocess.Env {
			base[key] = value
		}
	}

	extra := map[string]string{
		"COMPOZY_PROTOCOL_VERSION": compozyProtocolVersion(),
		"COMPOZY_RUN_ID":           strings.TrimSpace(manager.runID),
		"COMPOZY_PARENT_RUN_ID":    strings.TrimSpace(manager.parentRunID),
		"COMPOZY_WORKSPACE_ROOT":   strings.TrimSpace(manager.workspaceRoot),
		"COMPOZY_EXTENSION_NAME":   extension.normalizedName(),
		"COMPOZY_EXTENSION_SOURCE": string(extension.Ref.Source),
	}
	return subprocess.MergeEnvironment(base, extra)
}

func durationFromMilliseconds(value int64) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Millisecond
}

func capabilityStrings(values []Capability) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(string(value)))
	}
	return out
}

func hookStrings(values []HookName) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(string(value)))
	}
	return out
}

func manifestHookSet(extension *RuntimeExtension) map[HookName]struct{} {
	set := make(map[HookName]struct{})
	if extension == nil || extension.Manifest == nil {
		return set
	}

	for _, hook := range extension.Manifest.Hooks {
		normalized := HookName(strings.TrimSpace(string(hook.Event)))
		if normalized == "" {
			continue
		}
		set[normalized] = struct{}{}
	}
	return set
}

func equalHookSets(left map[HookName]struct{}, right map[HookName]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for hook := range left {
		if _, ok := right[hook]; !ok {
			return false
		}
	}
	return true
}

func sortedHookNames(values map[HookName]struct{}) []string {
	if len(values) == 0 {
		return nil
	}

	names := make([]string, 0, len(values))
	for hook := range values {
		names = append(names, strings.TrimSpace(string(hook)))
	}
	slices.Sort(names)
	return names
}

func formatProcessStderr(process *subprocess.Process) string {
	if process == nil || process.StderrBuffer() == nil {
		return ""
	}

	stderr := strings.TrimSpace(process.StderrBuffer().String())
	if stderr == "" {
		return ""
	}
	return "; stderr: " + stderr
}
