package cmdpalette

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var testProfileLens = ScopedProfileLens(DefaultProfileLensID, "default")

func testCatalogRequest(workspaceID WorkspaceID, clientID ClientID) CatalogRequest {
	return CatalogRequest{ProfileLens: testProfileLens, WorkspaceID: workspaceID, ClientID: clientID}
}

type staticTestProvider struct {
	commands []Descriptor
	err      error
}

func (p staticTestProvider) ProvideCommands(context.Context, CatalogRequest) ([]Descriptor, error) {
	if p.err != nil {
		return nil, p.err
	}
	return cloneDescriptors(p.commands), nil
}

func (p staticTestProvider) StaticCommands() []Descriptor { return cloneDescriptors(p.commands) }

type dynamicTestProvider struct {
	commands []Descriptor
	err      error
}

func (p dynamicTestProvider) ProvideCommands(context.Context, CatalogRequest) ([]Descriptor, error) {
	if p.err != nil {
		return nil, p.err
	}
	return cloneDescriptors(p.commands), nil
}

type testBindings struct {
	bindings       map[CommandID][]string
	aliases        map[CommandID]string
	globalBindings map[CommandID]string
	err            error
}

func (b *testBindings) GlobalBindingsForCatalogSnapshot(
	context.Context,
	ProfileLens,
	WorkspaceID,
	[]CommandID,
) (map[CommandID]string, error) {
	return b.globalBindings, b.err
}

func (b *testBindings) Bindings(
	context.Context,
	ProfileLens,
	WorkspaceID,
) (map[CommandID][]string, map[CommandID]string, error) {
	return b.bindings, b.aliases, b.err
}

type testClientDirectory struct {
	mu             sync.Mutex
	clients        []Client
	contexts       map[ClientID]ContextSnapshot
	tokens         map[ClientID]string
	globalStatuses map[ClientID]map[CommandID]GlobalShortcut
}

func (d *testClientDirectory) GlobalShortcutStatuses(
	_ context.Context,
	_ ProfileLens,
	_ WorkspaceID,
	clientID ClientID,
) (map[CommandID]GlobalShortcut, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.globalStatuses[clientID], nil
}

func (d *testClientDirectory) Clients(context.Context, WorkspaceID) ([]Client, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Client(nil), d.clients...), nil
}

func (d *testClientDirectory) Context(
	_ context.Context,
	_ WorkspaceID,
	clientID ClientID,
) (ContextSnapshot, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	snapshot, exists := d.contexts[clientID]
	if !exists {
		return ContextSnapshot{}, errors.New("test client not found")
	}
	return snapshot, nil
}

func (d *testClientDirectory) Authorize(
	_ context.Context,
	_ WorkspaceID,
	clientID ClientID,
	token string,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.tokens[clientID] != token {
		return errors.New("test token mismatch")
	}
	return nil
}

type testExecutor struct {
	mu       sync.Mutex
	calls    []ExecutionRequest
	result   ExecutionResult
	err      error
	started  chan struct{}
	release  <-chan struct{}
	approval bool
}

type recordingEventRecorder struct {
	mu     sync.Mutex
	events []Event
	wake   chan struct{}
}

func (r *recordingEventRecorder) RecordCmdPaletteEvent(_ context.Context, event Event) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	if r.wake != nil {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
}

func (r *recordingEventRecorder) recorded() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}

type approvalEventExecutor struct {
	*testExecutor
	outcome string
}

func (e *approvalEventExecutor) ApprovalCompletionStatus(context.Context, ProfileLens, string) (string, error) {
	return e.outcome, nil
}

func (e *testExecutor) ExecuteAction(
	ctx context.Context,
	request ExecutionRequest,
) (ExecutionResult, error) {
	e.mu.Lock()
	e.calls = append(e.calls, request)
	started := e.started
	release := e.release
	result := e.result
	err := e.err
	e.mu.Unlock()
	if started != nil {
		select {
		case started <- struct{}{}:
		default:
		}
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return ExecutionResult{}, ctx.Err()
		}
	}
	return result, err
}

func (e *testExecutor) ApprovalRequired(context.Context, ExecutionRequest) (bool, error) {
	return e.approval, nil
}

func (e *testExecutor) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func testDescriptor(id CommandID) Descriptor {
	return Descriptor{
		ID: id, Title: "Test command", Section: "Test", Icon: "terminal",
		Source:    Source{Kind: SourceKindCore},
		Action:    Action{Kind: ActionKindTool, Tool: "compozy__test"},
		Arguments: []Argument{},
		Policy:    ExecutionPolicy{RetrySafe: true},
	}
}

func testRegistry(
	testingProvider Provider,
	clients ClientDirectory,
	bindings BindingsResolver,
	executor ActionExecutor,
) *Service {
	return testRegistryWithOptions(testingProvider, clients, bindings, executor)
}

func testRegistryWithOptions(
	testingProvider Provider,
	clients ClientDirectory,
	bindings BindingsResolver,
	executor ActionExecutor,
	options ...Option,
) *Service {
	options = append([]Option{WithInvocationIDGenerator(func() string { return "inv_test" })}, options...)
	service, err := NewRegistry(
		[]ProviderRegistration{{Source: Source{Kind: SourceKindCore}, Provider: testingProvider}},
		clients,
		bindings,
		executor,
		options...,
	)
	if err != nil {
		panic(err)
	}
	return service
}

func cloneDescriptors(source []Descriptor) []Descriptor {
	cloned := make([]Descriptor, 0, len(source))
	for _, descriptor := range source {
		cloned = append(cloned, cloneDescriptor(descriptor))
	}
	return cloned
}

func rawResult(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}
