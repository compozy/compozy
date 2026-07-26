package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	jsonRPCVersion = "2.0"
)

const (
	goHandshakeEnv = "AGH_SECRET_GUARD_HANDSHAKE_PATH"
	goHostCallEnv  = "AGH_SECRET_GUARD_HOST_CALL_PATH"
	goStartsEnv    = "AGH_SECRET_GUARD_STARTS_PATH"
	goCrashOnceEnv = "AGH_SECRET_GUARD_CRASH_ONCE_PATH"
	goShutdownEnv  = "AGH_SECRET_GUARD_SHUTDOWN_PATH"
)

var secretPatterns = []string{
	"sk-",
	"AKIA",
	"ghp_",
	"-----BEGIN RSA",
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) > 0 {
		switch strings.TrimSpace(args[0]) {
		case "hook":
			if len(args) < 2 {
				return errors.New("secret-guard: hook name is required")
			}
			return runHook(strings.TrimSpace(args[1]), stdin, stdout)
		case "--hook":
			if len(args) < 2 {
				return errors.New("secret-guard: hook name is required")
			}
			return runHook(strings.TrimSpace(args[1]), stdin, stdout)
		case "serve":
			return runServe(stdin, stdout, stderr)
		}
	}

	return runServe(stdin, stdout, stderr)
}

func runHook(name string, stdin io.Reader, stdout io.Writer) error {
	switch strings.TrimSpace(name) {
	case "input_pre_submit":
		var payload hookspkg.InputPreSubmitPayload
		if err := json.NewDecoder(stdin).Decode(&payload); err != nil {
			return fmt.Errorf("secret-guard: decode input_pre_submit payload: %w", err)
		}
		patch := evaluateSecretGuard(payload)
		return json.NewEncoder(stdout).Encode(patch)
	default:
		return fmt.Errorf("secret-guard: unsupported hook %q", name)
	}
}

func runServe(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	appendMarkerLine(os.Getenv(goStartsEnv), fmt.Sprintf("pid=%d", os.Getpid()))

	peer := newRPCPeer(stdin, stdout)
	runtime := &secretGuardRuntime{
		stderr: stderr,
		peer:   peer,
	}

	peer.handle("initialize", runtime.handleInitialize)
	peer.handle("execute_hook", runtime.handleExecuteHook)
	peer.handle("health_check", runtime.handleHealthCheck)
	peer.handle("shutdown", runtime.handleShutdown)

	return peer.serve()
}

type secretGuardRuntime struct {
	stderr io.Writer
	peer   *rpcPeer

	mu          sync.RWMutex
	initialized bool
	session     runtimeSession
}

type runtimeSession struct {
	request  subprocess.InitializeRequest
	response subprocess.InitializeResponse
}

type executeHookParams struct {
	InvocationID string `json:"invocation_id"`
	Hook         struct {
		Name     string            `json:"name"`
		Event    string            `json:"event"`
		Mode     string            `json:"mode"`
		Required bool              `json:"required"`
		Timeout  int64             `json:"timeout_ms"`
		Source   string            `json:"source"`
		Metadata map[string]string `json:"metadata,omitempty"`
	} `json:"hook"`
	Payload json.RawMessage `json:"payload"`
}

type hostSessionSummary struct {
	ID string `json:"id"`
}

type rpcEnvelope struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *rpcErrorPayload `json:"error,omitempty"`
}

type rpcErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcPeer struct {
	scanner *bufio.Scanner
	encoder *json.Encoder

	writeMu sync.Mutex
	pending sync.Map
	wg      sync.WaitGroup
	nextID  atomic.Int64
	errMu   sync.Mutex
	err     error

	handlers map[string]func(json.RawMessage) (any, error)
}

func newRPCPeer(stdin io.Reader, stdout io.Writer) *rpcPeer {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)

	return &rpcPeer{
		scanner:  scanner,
		encoder:  encoder,
		handlers: make(map[string]func(json.RawMessage) (any, error)),
	}
}

func (p *rpcPeer) handle(method string, handler func(json.RawMessage) (any, error)) {
	p.handlers[strings.TrimSpace(method)] = handler
}

func (p *rpcPeer) serve() error {
	for p.scanner.Scan() {
		if err := p.currentError(); err != nil {
			return err
		}

		line := strings.TrimSpace(p.scanner.Text())
		if line == "" {
			continue
		}

		var envelope rpcEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			return fmt.Errorf("secret-guard: decode rpc frame: %w", err)
		}

		if strings.TrimSpace(envelope.Method) != "" {
			p.wg.Add(1)
			go func(env rpcEnvelope) {
				defer p.wg.Done()
				p.dispatchRequest(env)
			}(envelope)
			continue
		}

		idKey := string(bytesTrim(envelope.ID))
		if idKey == "" {
			continue
		}
		if pending, ok := p.pending.LoadAndDelete(idKey); ok {
			responseCh, ok := pending.(chan rpcEnvelope)
			if !ok {
				return fmt.Errorf("secret-guard: invalid pending response channel type %T", pending)
			}
			responseCh <- envelope
		}
	}

	p.wg.Wait()
	if err := p.currentError(); err != nil {
		return err
	}
	if err := p.scanner.Err(); err != nil {
		return fmt.Errorf("secret-guard: read rpc frame: %w", err)
	}
	return nil
}

func (p *rpcPeer) dispatchRequest(envelope rpcEnvelope) {
	handler, ok := p.handlers[strings.TrimSpace(envelope.Method)]
	if !ok {
		if err := p.sendError(envelope.ID, rpcErrorPayload{Code: -32601, Message: "Method not found"}); err != nil {
			p.recordError(fmt.Errorf("secret-guard: send method-not-found error: %w", err))
		}
		return
	}

	result, err := handler(envelope.Params)
	if err != nil {
		if rpcErr, ok := errors.AsType[*runtimeRPCError](err); ok {
			if sendErr := p.sendError(
				envelope.ID,
				rpcErrorPayload{Code: rpcErr.Code, Message: rpcErr.Message},
			); sendErr != nil {
				p.recordError(fmt.Errorf("secret-guard: send rpc error: %w", sendErr))
			}
			return
		}
		if sendErr := p.sendError(envelope.ID, rpcErrorPayload{Code: -32603, Message: err.Error()}); sendErr != nil {
			p.recordError(fmt.Errorf("secret-guard: send internal error: %w", sendErr))
		}
		return
	}

	if sendErr := p.sendResult(envelope.ID, result); sendErr != nil {
		p.recordError(fmt.Errorf("secret-guard: send result: %w", sendErr))
	}
}

func (p *rpcPeer) recordError(err error) {
	if err == nil {
		return
	}
	p.errMu.Lock()
	defer p.errMu.Unlock()
	if p.err == nil {
		p.err = err
		return
	}
	p.err = errors.Join(p.err, err)
}

func (p *rpcPeer) currentError() error {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	return p.err
}

func (p *rpcPeer) call(ctx context.Context, method string, params any, result any) error {
	idValue := fmt.Sprintf("secret-guard-%d", p.nextID.Add(1))
	idBytes, err := json.Marshal(idValue)
	if err != nil {
		return err
	}

	responseCh := make(chan rpcEnvelope, 1)
	p.pending.Store(string(idBytes), responseCh)
	if err := p.writeFrame(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		ID:      idBytes,
		Method:  method,
		Params:  mustRawJSON(params),
	}); err != nil {
		p.pending.Delete(string(idBytes))
		return err
	}

	select {
	case <-ctx.Done():
		p.pending.Delete(string(idBytes))
		return ctx.Err()
	case response := <-responseCh:
		if response.Error != nil {
			return &runtimeRPCError{Code: response.Error.Code, Message: response.Error.Message}
		}
		if result == nil {
			return nil
		}
		payload, err := json.Marshal(response.Result)
		if err != nil {
			return err
		}
		if len(payload) == 0 || string(payload) == "null" {
			return nil
		}
		if err := json.Unmarshal(payload, result); err != nil {
			return err
		}
		return nil
	}
}

func (p *rpcPeer) sendResult(id json.RawMessage, result any) error {
	return p.writeFrame(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		ID:      append(json.RawMessage(nil), id...),
		Result:  result,
	})
}

func (p *rpcPeer) sendError(id json.RawMessage, rpcErr rpcErrorPayload) error {
	return p.writeFrame(rpcEnvelope{
		JSONRPC: jsonRPCVersion,
		ID:      append(json.RawMessage(nil), id...),
		Error:   &rpcErr,
	})
}

func (p *rpcPeer) writeFrame(frame rpcEnvelope) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	return p.encoder.Encode(frame)
}

func (r *secretGuardRuntime) handleInitialize(params json.RawMessage) (any, error) {
	var request subprocess.InitializeRequest
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, fmt.Errorf("secret-guard: decode initialize request: %w", err)
	}

	response := subprocess.InitializeResponse{
		ProtocolVersion: request.ProtocolVersion,
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    "secret-guard",
			Version: "0.1.0",
		},
		AcceptedCapabilities: subprocess.AcceptedCapabilities{
			Provides: append([]string(nil), request.Capabilities.Provides...),
			Actions:  append([]extensionprotocol.HostAPIMethod(nil), request.Capabilities.GrantedActions...),
			Security: append([]string(nil), request.Capabilities.GrantedSecurity...),
		},
		ImplementedMethods:  []string{"execute_hook", "health_check", "shutdown"},
		SupportedHookEvents: []string{string(hookspkg.HookInputPreSubmit)},
		Supports: subprocess.InitializeSupports{
			HealthCheck: true,
		},
	}

	r.mu.Lock()
	r.initialized = true
	r.session = runtimeSession{request: request, response: response}
	r.mu.Unlock()

	if err := writeJSONFile(os.Getenv(goHandshakeEnv), map[string]any{
		"request":  request,
		"response": response,
		"pid":      os.Getpid(),
	}); err != nil {
		return nil, err
	}

	go r.afterInitialize()

	return response, nil
}

func (r *secretGuardRuntime) afterInitialize() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	summaries, err := r.sessionsListWithRetry(ctx)
	marker := map[string]any{}
	if err != nil {
		marker["error"] = err.Error()
	} else {
		marker["session_count"] = len(summaries)
		marker["sessions"] = summaries
	}
	marker["pid"] = os.Getpid()
	if err := writeJSONFile(os.Getenv(goHostCallEnv), marker); err != nil {
		_, _ = fmt.Fprintf(r.stderr, "write host-call marker: %v\n", err)
	}

	crashMarker := strings.TrimSpace(os.Getenv(goCrashOnceEnv))
	if crashMarker != "" && !fileExists(crashMarker) {
		if err := writeJSONFile(crashMarker, map[string]any{
			"crashed": true,
			"pid":     os.Getpid(),
		}); err != nil {
			_, _ = fmt.Fprintf(r.stderr, "write crash marker: %v\n", err)
			return
		}
		go func() {
			time.Sleep(150 * time.Millisecond)
			os.Exit(23)
		}()
	}
}

func (r *secretGuardRuntime) sessionsListWithRetry(ctx context.Context) ([]hostSessionSummary, error) {
	var lastErr error
	for attempt := range 5 {
		var sessions []hostSessionSummary
		err := r.peer.call(ctx, "sessions/list", map[string]any{}, &sessions)
		if err == nil {
			return sessions, nil
		}
		lastErr = err

		var rpcErr *runtimeRPCError
		if errors.As(err, &rpcErr) && rpcErr.Code == -32003 {
			time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
			continue
		}
		if ctx.Err() != nil {
			break
		}
		return nil, err
	}

	if lastErr == nil {
		lastErr = errors.New("secret-guard: sessions/list failed")
	}
	return nil, lastErr
}

func (r *secretGuardRuntime) handleExecuteHook(params json.RawMessage) (any, error) {
	var request executeHookParams
	if err := json.Unmarshal(params, &request); err != nil {
		return nil, fmt.Errorf("secret-guard: decode execute_hook params: %w", err)
	}

	switch hookspkg.HookEvent(strings.TrimSpace(request.Hook.Event)) {
	case hookspkg.HookInputPreSubmit:
		var payload hookspkg.InputPreSubmitPayload
		if err := json.Unmarshal(request.Payload, &payload); err != nil {
			return nil, fmt.Errorf("secret-guard: decode input.pre_submit payload: %w", err)
		}
		return evaluateSecretGuard(payload), nil
	default:
		return map[string]any{}, nil
	}
}

func (r *secretGuardRuntime) handleHealthCheck(json.RawMessage) (any, error) {
	return subprocess.HealthCheckResponse{
		Healthy: true,
		Message: "",
	}, nil
}

func (r *secretGuardRuntime) handleShutdown(params json.RawMessage) (any, error) {
	var request subprocess.ShutdownRequest
	if len(bytesTrim(params)) > 0 && string(bytesTrim(params)) != "null" {
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, fmt.Errorf("secret-guard: decode shutdown request: %w", err)
		}
	}

	appendMarkerLine(os.Getenv(goShutdownEnv), fmt.Sprintf("reason=%s", request.Reason))
	return subprocess.ShutdownResponse{Acknowledged: true}, nil
}

func evaluateSecretGuard(payload hookspkg.InputPreSubmitPayload) hookspkg.InputPreSubmitPatch {
	if patch, ok := evaluateSecretGuardText("Message", payload.Message); ok {
		return patch
	}

	for _, block := range payload.ContextBlocks {
		if patch, ok := evaluateSecretGuardText("Context block", block.Text); ok {
			return patch
		}
	}

	return hookspkg.InputPreSubmitPatch{}
}

func evaluateSecretGuardText(source string, text string) (hookspkg.InputPreSubmitPatch, bool) {
	for _, pattern := range secretPatterns {
		if strings.Contains(text, pattern) {
			reason := fmt.Sprintf("%s contains a potential secret (%s)", source, pattern)
			return hookspkg.InputPreSubmitPatch{
				ControlPatch: hookspkg.ControlPatch{
					Deny:       true,
					DenyReason: reason,
				},
			}, true
		}
	}
	return hookspkg.InputPreSubmitPatch{}, false
}

type runtimeRPCError struct {
	Code    int
	Message string
}

func (e *runtimeRPCError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func mustRawJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return payload
}

func bytesTrim(value []byte) []byte {
	return []byte(strings.TrimSpace(string(value)))
}

func writeJSONFile(path string, value any) error {
	target, err := resolveMarkerPath(path)
	if err != nil {
		return err
	}
	if !target.IsSet() {
		return nil
	}

	if err := target.EnsureParentDir(); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return target.WriteFile(payload)
}

func appendMarkerLine(path string, line string) {
	target, err := resolveMarkerPath(path)
	if err != nil {
		return
	}
	if !target.IsSet() {
		return
	}
	if err := target.EnsureParentDir(); err != nil {
		return
	}
	file, err := target.OpenAppender()
	if err != nil {
		return
	}
	defer func() {
		_ = file.Close()
	}()
	_, _ = fmt.Fprintln(file, strings.TrimSpace(line))
}

func fileExists(path string) bool {
	target, err := resolveMarkerPath(path)
	if err != nil || !target.IsSet() {
		return false
	}
	return target.Exists()
}

type markerPath struct {
	root string
	name string
}

func (p markerPath) IsSet() bool {
	return strings.TrimSpace(p.root) != "" && strings.TrimSpace(p.name) != ""
}

func (p markerPath) parentDir() string {
	return filepath.Dir(p.name)
}

func (p markerPath) openRoot() (*os.Root, error) {
	if !p.IsSet() {
		return nil, errors.New("secret-guard: marker path is not set")
	}
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return nil, fmt.Errorf("secret-guard: open marker root %q: %w", p.root, err)
	}
	return root, nil
}

func (p markerPath) EnsureParentDir() error {
	parent := p.parentDir()
	if parent == "." {
		return nil
	}

	root, err := p.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	return root.MkdirAll(parent, 0o755)
}

func (p markerPath) WriteFile(payload []byte) error {
	root, err := p.openRoot()
	if err != nil {
		return err
	}
	defer root.Close()

	return root.WriteFile(p.name, payload, 0o600)
}

func (p markerPath) OpenAppender() (*os.File, error) {
	root, err := p.openRoot()
	if err != nil {
		return nil, err
	}

	file, openErr := root.OpenFile(p.name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	closeErr := root.Close()
	if openErr != nil {
		return nil, openErr
	}
	if closeErr != nil {
		_ = file.Close()
		return nil, closeErr
	}
	return file, nil
}

func (p markerPath) Exists() bool {
	root, err := p.openRoot()
	if err != nil {
		return false
	}
	defer root.Close()

	_, statErr := root.Stat(p.name)
	return statErr == nil
}

func resolveMarkerPath(path string) (markerPath, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return markerPath{}, nil
	}

	cleanTarget := filepath.Clean(target)
	if !filepath.IsAbs(cleanTarget) {
		return markerPath{}, fmt.Errorf("secret-guard: marker path must be absolute: %q", path)
	}

	tempRoot := filepath.Clean(os.TempDir())
	if marker, ok := markerPathWithinRoot(tempRoot, cleanTarget); ok {
		return marker, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return markerPath{}, fmt.Errorf("secret-guard: resolve cwd: %w", err)
	}
	cleanCWD := filepath.Clean(cwd)
	if marker, ok := markerPathWithinRoot(cleanCWD, cleanTarget); ok {
		return marker, nil
	}

	return markerPath{}, fmt.Errorf("secret-guard: marker path %q is outside allowed roots", cleanTarget)
}

func markerPathWithinRoot(root string, target string) (markerPath, bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return markerPath{}, false
	}
	if relative == "." || relative == ".." {
		return markerPath{}, false
	}
	if filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return markerPath{}, false
	}
	return markerPath{root: root, name: relative}, true
}
