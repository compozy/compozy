package compozysdk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/sdk/go/contracts"
)

const (
	extensionErrorKey   = "error"
	extensionHandlerKey = "handler"
	extensionToolIDKey  = "tool_id"
)

// ExtensionContext is passed to custom service handlers.
type ExtensionContext struct {
	Request   JSONRPCRequestEnvelope
	RequestID json.RawMessage
	Host      *HostAPI
	Session   ExtensionSession
	Logf      func(format string, args ...any)
}

// ExtensionHandler handles one custom Compozy -> extension service request.
type ExtensionHandler func(context.Context, ExtensionContext, json.RawMessage) (any, error)

type registeredTool struct {
	descriptor           ExtensionToolRuntimeDescriptor
	handler              func(context.Context, rawToolRequest) (ToolResult, error)
	sensitiveInputFields []string
}

// Extension is a subprocess-hosted Compozy extension runtime.
type Extension struct {
	definition ExtensionDefinition
	transport  Transport
	stderr     io.Writer
	sdkVersion string
	host       *HostAPI

	mu                   sync.RWMutex
	handlers             map[string]ExtensionHandler
	toolHandlers         map[string]registeredTool
	commandGroups        []contracts.ExtensionCommandGroupSpec
	watchHandlers        map[string]registeredWatchSource
	readyCallbacks       []func(context.Context, *HostAPI, ExtensionSession) error
	readyLifecycle       *readyCallbackLifecycle
	readyCallbacksClosed bool
	initialized          bool
	shutdownStarted      bool
	shutdownDeadlineMS   int64
	shutdownDeadlineAt   time.Time
	session              *ExtensionSession
}

// Option configures an Extension.
type Option func(*Extension)

// WithTransport sets a custom JSON-RPC transport.
func WithTransport(transport Transport) Option {
	return func(extension *Extension) {
		if transport != nil {
			extension.transport = transport
		}
	}
}

// WithStdio configures stdio input and output streams.
func WithStdio(input io.Reader, output io.Writer) Option {
	return func(extension *Extension) {
		extension.transport = NewStdioTransport(StdioTransportOptions{Input: input, Output: output})
	}
}

// WithStderr sets the log sink used by ExtensionContext.Logf.
func WithStderr(stderr io.Writer) Option {
	return func(extension *Extension) {
		if stderr != nil {
			extension.stderr = stderr
		}
	}
}

// WithSDKVersion overrides the advertised SDK version.
func WithSDKVersion(version string) Option {
	return func(extension *Extension) {
		if strings.TrimSpace(version) != "" {
			extension.sdkVersion = strings.TrimSpace(version)
		}
	}
}

// NewExtension creates a public Go extension runtime.
func NewExtension(definition ExtensionDefinition, options ...Option) *Extension {
	extension := &Extension{
		definition:     definition,
		transport:      NewStdioTransport(StdioTransportOptions{}),
		stderr:         os.Stderr,
		sdkVersion:     SDKVersion,
		handlers:       make(map[string]ExtensionHandler),
		toolHandlers:   make(map[string]registeredTool),
		watchHandlers:  make(map[string]registeredWatchSource),
		readyLifecycle: newReadyCallbackLifecycle(),
	}
	extension.host = newHostAPI(extension.transport, extension.ready)
	for _, option := range options {
		option(extension)
	}
	extension.host = newHostAPI(extension.transport, extension.ready)
	extension.bindBuiltins()
	return extension
}

// Handle registers one custom Compozy -> extension service method.
func (e *Extension) Handle(method string, handler ExtensionHandler) error {
	if e == nil {
		return NewInternalError("extension is required")
	}
	cleanMethod := strings.TrimSpace(method)
	if cleanMethod == "" {
		return NewInvalidParamsError("method is required", nil)
	}
	if cleanMethod == initializeMethod {
		return NewInvalidParamsError("initialize is reserved by the SDK", nil)
	}
	if handler == nil {
		return NewInvalidParamsError("handler is required", nil)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.initialized {
		return NewInvalidParamsError("extension registration is closed after initialize", nil)
	}
	if len(e.toolHandlers) > 0 && isToolProviderMethod(cleanMethod) {
		return NewInvalidParamsError(cleanMethod+" is reserved by Tool", nil)
	}
	if len(e.watchHandlers) > 0 && isWatchSourceMethod(cleanMethod) {
		return NewInvalidParamsError(cleanMethod+" is reserved by WatchSource", nil)
	}
	e.handlers[cleanMethod] = handler
	e.transport.Handle(cleanMethod, e.dispatch)
	return nil
}

// Tool registers a raw JSON tool handler on the extension.
func (e *Extension) Tool(
	handler string,
	options ToolOptions,
	fn ToolHandlerFunc[json.RawMessage],
) error {
	return Tool[json.RawMessage](e, handler, options, fn)
}

// Tool registers a typed Go function as an extension-host tool handler.
func Tool[TInput any](
	extension *Extension,
	handler string,
	options ToolOptions,
	fn ToolHandlerFunc[TInput],
) error {
	if extension == nil {
		return NewInternalError("extension is required")
	}
	if fn == nil {
		return NewInvalidParamsError("tool handler function is required", nil)
	}
	return extension.registerTool(handler, options, func(ctx context.Context, req rawToolRequest) (ToolResult, error) {
		var input TInput
		rawInput := req.input
		if len(rawInput) == 0 {
			rawInput = json.RawMessage(`{}`)
		}
		if err := json.Unmarshal(rawInput, &input); err != nil {
			return ToolResult{}, NewInvalidParamsError("tool input does not match handler type", map[string]any{
				extensionHandlerKey: req.handler,
				extensionErrorKey:   err.Error(),
			})
		}
		return fn(ctx, ToolRequest[TInput]{
			Input:            input,
			RawInput:         cloneRawMessage(rawInput),
			Context:          req.context,
			Host:             req.context.Host,
			Session:          req.context.Session,
			ToolID:           req.toolID,
			Handler:          req.handler,
			TrustedWorkspace: req.trustedWorkspace,
			invocationID:     req.invocationID,
		})
	})
}

// OnReady registers a callback that runs after initialize succeeds.
//
// Callbacks must return after ctx.Done. During shutdown, the runtime cancels
// accepted callbacks and takes one bounded drain snapshot at the shutdown
// request deadline, or at the negotiated runtime timeout when the transport
// closes without a request. Observable completion wins when completion and the
// deadline are both ready at that snapshot. Run returns callback failures in
// the snapshot; failures recorded after a deadline decision are excluded. A
// callback that outlives that decision may outlive Run. Callbacks registered
// after shutdown or runtime closure are ignored.
func (e *Extension) OnReady(callback func(context.Context, *HostAPI, ExtensionSession) error) {
	if e == nil || callback == nil {
		return
	}
	e.mu.Lock()
	if e.shutdownStarted || e.readyCallbacksClosed {
		e.mu.Unlock()
		return
	}
	e.readyCallbacks = append(e.readyCallbacks, callback)
	session := e.session
	initialized := e.initialized && session != nil
	host := e.host
	e.mu.Unlock()
	if initialized {
		e.startReadyCallback(callback, host, session)
	}
}

// Run starts the JSON-RPC transport and blocks until it closes.
func (e *Extension) Run(ctx context.Context) error {
	if e == nil {
		return NewInternalError("extension is required")
	}
	if err := e.definition.validate(); err != nil {
		return err
	}
	if describeModeRequested(os.Args) {
		return e.writeDescribe(os.Stdout)
	}
	runErr := e.transport.Run(ctx)
	return errors.Join(runErr, e.stopReadyCallbacks())
}
