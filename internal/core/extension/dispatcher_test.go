package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestHookDispatcherBuildsPriorityOrderedChains(t *testing.T) {
	t.Parallel()

	registry := mustRegistry(t,
		newRuntimeExtension("zeta", []Capability{CapabilityPromptMutate}, HookDeclaration{
			Event:    HookPromptPostBuild,
			Priority: 500,
		}),
		newRuntimeExtension("alpha", []Capability{CapabilityPromptMutate}, HookDeclaration{
			Event:    HookPromptPostBuild,
			Priority: 500,
		}),
		newRuntimeExtension("gamma", []Capability{CapabilityPromptMutate}, HookDeclaration{
			Event:    HookPromptPostBuild,
			Priority: 100,
		}),
	)

	dispatcher := NewHookDispatcher(registry, nil)
	entries := dispatcher.chains[HookPromptPostBuild]

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.extension.Name)
	}

	want := []string{"gamma", "alpha", "zeta"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("chain order = %v, want %v", got, want)
	}
}

func TestDispatchMutableFeedsMutatedPayloadThroughChain(t *testing.T) {
	t.Parallel()

	firstSeen := make(chan string, 1)
	secondSeen := make(chan string, 1)
	audit := &auditSpy{}

	registry := mustRegistry(t,
		newRuntimeExtensionWithCaller(
			"first",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
					promptText := promptTextFromPayload(t, request.Payload)
					firstSeen <- promptText
					return patchPromptText(t, promptText+"-one"), nil
				},
			},
			HookDeclaration{Event: HookPromptPostBuild, Priority: 100, Required: true},
		),
		newRuntimeExtensionWithCaller(
			"second",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
					promptText := promptTextFromPayload(t, request.Payload)
					secondSeen <- promptText
					return patchPromptText(t, promptText+"-two"), nil
				},
			},
			HookDeclaration{Event: HookPromptPostBuild, Priority: 200, Required: true},
		),
	)

	dispatcher := NewHookDispatcher(registry, audit)
	result, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "base",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	if got := mustReceiveString(t, firstSeen, time.Second, "first extension"); got != "base" {
		t.Fatalf("first extension saw %q, want %q", got, "base")
	}
	if got := mustReceiveString(t, secondSeen, time.Second, "second extension"); got != "base-one" {
		t.Fatalf("second extension saw %q, want %q", got, "base-one")
	}
	if got := promptTextFromPayload(t, result); got != "base-one-two" {
		t.Fatalf("final prompt_text = %q, want %q", got, "base-one-two")
	}

	entries := audit.entries()
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(entries))
	}
	for _, entry := range entries {
		if entry.Result != AuditResultOK {
			t.Fatalf("audit result = %q, want %q", entry.Result, AuditResultOK)
		}
	}
}

func TestDispatchMutableRequiredFailureAbortsChain(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("boom")
	thirdCalled := false
	audit := &auditSpy{}

	registry := mustRegistry(t,
		newPromptAppenderExtension(t, "first", 100, true, "-one"),
		newRuntimeExtensionWithCaller(
			"broken",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, _ executeHookRequest) (json.RawMessage, error) {
					return nil, sentinel
				},
			},
			HookDeclaration{Event: HookPromptPostBuild, Priority: 200, Required: true},
		),
		newRuntimeExtensionWithCaller(
			"third",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
					thirdCalled = true
					return patchPromptText(t, promptTextFromPayload(t, request.Payload)+"-three"), nil
				},
			},
			HookDeclaration{Event: HookPromptPostBuild, Priority: 300, Required: true},
		),
	)

	dispatcher := NewHookDispatcher(registry, audit)
	_, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "base",
	})
	if err == nil {
		t.Fatal("DispatchMutable() error = nil, want failure")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("DispatchMutable() error = %v, want wrapped sentinel", err)
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Fatalf("DispatchMutable() error = %q, want extension name", err.Error())
	}
	if thirdCalled {
		t.Fatal("third extension was called after required failure")
	}

	entries := audit.entries()
	if len(entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(entries))
	}
	if entries[1].Result != AuditResultError {
		t.Fatalf("failed audit result = %q, want %q", entries[1].Result, AuditResultError)
	}
}

func TestDispatchMutableOptionalFailureContinuesWithPreviousValue(t *testing.T) {
	t.Parallel()

	audit := &auditSpy{}

	registry := mustRegistry(t,
		newPromptAppenderExtension(t, "first", 100, true, "-one"),
		newRuntimeExtensionWithCaller(
			"broken",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, _ executeHookRequest) (json.RawMessage, error) {
					return nil, errors.New("optional failure")
				},
			},
			HookDeclaration{Event: HookPromptPostBuild, Priority: 200, Required: false},
		),
		newPromptAppenderExtension(t, "third", 300, true, "-three"),
	)

	dispatcher := NewHookDispatcher(registry, audit)
	result, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "base",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	if got := promptTextFromPayload(t, result); got != "base-one-three" {
		t.Fatalf("final prompt_text = %q, want %q", got, "base-one-three")
	}

	entries := audit.entries()
	if len(entries) != 3 {
		t.Fatalf("audit entries = %d, want 3", len(entries))
	}
	if entries[1].Result != AuditResultError {
		t.Fatalf("optional failure audit result = %q, want %q", entries[1].Result, AuditResultError)
	}
}

func TestDispatchMutableTimeoutWrapsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	registry := mustRegistry(t,
		newRuntimeExtensionWithCaller(
			"slow",
			[]Capability{CapabilityPromptMutate},
			&fakeExtensionCaller{
				handler: func(ctx context.Context, _ executeHookRequest) (json.RawMessage, error) {
					<-ctx.Done()
					return nil, ctx.Err()
				},
			},
			HookDeclaration{
				Event:    HookPromptPostBuild,
				Priority: 100,
				Required: true,
				Timeout:  20 * time.Millisecond,
			},
		),
	)

	dispatcher := NewHookDispatcher(registry, &auditSpy{})
	_, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "base",
	})
	if err == nil {
		t.Fatal("DispatchMutable() error = nil, want timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("DispatchMutable() error = %v, want deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "slow") {
		t.Fatalf("DispatchMutable() error = %q, want extension name", err.Error())
	}
}

func TestDispatchObserverFansOutConcurrently(t *testing.T) {
	t.Parallel()

	fastCalled := make(chan struct{}, 1)
	slowStarted := make(chan struct{}, 1)
	releaseSlow := make(chan struct{})
	audit := &auditSpy{}

	registry := mustRegistry(t,
		newRuntimeExtensionWithCaller(
			"fast",
			[]Capability{CapabilityAgentMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
					if request.Hook.Mutable {
						t.Fatalf("observer hook mutable = true, want false")
					}
					fastCalled <- struct{}{}
					return nil, nil
				},
			},
			HookDeclaration{Event: HookAgentPostSessionCreate, Priority: 100, Required: false},
		),
		newRuntimeExtensionWithCaller(
			"slow",
			[]Capability{CapabilityAgentMutate},
			&fakeExtensionCaller{
				handler: func(ctx context.Context, request executeHookRequest) (json.RawMessage, error) {
					if request.Hook.Mutable {
						t.Fatalf("observer hook mutable = true, want false")
					}
					slowStarted <- struct{}{}
					select {
					case <-releaseSlow:
						return nil, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				},
			},
			HookDeclaration{
				Event:    HookAgentPostSessionCreate,
				Priority: 200,
				Required: false,
				Timeout:  time.Second,
			},
		),
	)

	dispatcher := NewHookDispatcher(registry, audit)

	returned := make(chan struct{})
	go func() {
		dispatcher.DispatchObserver(context.Background(), HookAgentPostSessionCreate, map[string]any{
			"session_id": "sess-1",
		})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("DispatchObserver() blocked waiting for observers")
	}

	select {
	case <-fastCalled:
	case <-time.After(time.Second):
		t.Fatal("fast observer was not called")
	}

	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow observer was not started")
	}

	close(releaseSlow)
	dispatcher.waitForObservers()

	if got := len(audit.entries()); got != 2 {
		t.Fatalf("audit entries = %d, want 2", got)
	}
}

func TestDispatchObserverUsesBoundedWorkersAndSharedRawPayload(t *testing.T) {
	t.Parallel()

	workerLimit := max(runtime.GOMAXPROCS(0), 1)
	totalObservers := workerLimit + 4
	payload := map[string]any{
		"session_id": "sess-1",
		"status":     "running",
	}
	expected, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	release := make(chan struct{})
	started := make(chan struct{}, totalObservers)
	errs := make(chan error, totalObservers)
	var (
		active         atomic.Int32
		maxActive      atomic.Int32
		sharedPayload  atomic.Uintptr
		observerStarts atomic.Int32
	)

	extensions := make([]*RuntimeExtension, 0, totalObservers)
	for i := 0; i < totalObservers; i++ {
		extensions = append(extensions, newRuntimeExtensionWithCaller(
			observerExtensionName(i),
			[]Capability{CapabilityAgentMutate},
			&fakeExtensionCaller{
				handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
					raw, ok := request.Payload.(json.RawMessage)
					if !ok {
						errs <- errors.New("observer payload is not json.RawMessage")
						return nil, nil
					}
					if !bytes.Equal(raw, expected) {
						errs <- errors.New("observer payload bytes changed")
						return nil, nil
					}
					if len(raw) > 0 {
						ptr := uintptr(unsafe.Pointer(&raw[0]))
						if prior := sharedPayload.Load(); prior == 0 {
							sharedPayload.CompareAndSwap(0, ptr)
						} else if prior != ptr {
							errs <- errors.New("observer payload did not reuse the same raw bytes")
							return nil, nil
						}
					}

					current := active.Add(1)
					observerStarts.Add(1)
					for {
						seen := maxActive.Load()
						if current <= seen || maxActive.CompareAndSwap(seen, current) {
							break
						}
					}
					started <- struct{}{}
					<-release
					active.Add(-1)
					return nil, nil
				},
			},
			HookDeclaration{
				Event:    HookAgentPostSessionCreate,
				Priority: 100 + i,
				Required: false,
				Timeout:  time.Second,
			},
		))
	}

	dispatcher := NewHookDispatcher(mustRegistry(t, extensions...), &auditSpy{})
	dispatcher.DispatchObserver(context.Background(), HookAgentPostSessionCreate, payload)

	for range workerLimit {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("expected %d observer workers to start", workerLimit)
		}
	}

	close(release)
	dispatcher.waitForObservers()

	if got := int(maxActive.Load()); got > workerLimit {
		t.Fatalf("max concurrent observer calls = %d, want <= %d", got, workerLimit)
	}
	if got := int(observerStarts.Load()); got != totalObservers {
		t.Fatalf("observer starts = %d, want %d", got, totalObservers)
	}

	select {
	case err := <-errs:
		t.Fatal(err)
	default:
	}
}

func TestDispatchMutableSendsEffectiveDefaultTimeoutToExtension(t *testing.T) {
	t.Parallel()

	timeoutSeen := make(chan int64, 1)
	extension := newRuntimeExtensionWithCaller(
		"timed",
		[]Capability{CapabilityPromptMutate},
		&fakeExtensionCaller{
			handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
				timeoutSeen <- request.Hook.TimeoutMS
				return patchPromptText(t, promptTextFromPayload(t, request.Payload)+"-timed"), nil
			},
		},
		HookDeclaration{Event: HookPromptPostBuild, Priority: 100, Required: true},
	)
	extension.SetDefaultHookTimeout(750 * time.Millisecond)

	dispatcher := NewHookDispatcher(mustRegistry(t, extension), nil)
	_, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "base",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	select {
	case got := <-timeoutSeen:
		if got != 750 {
			t.Fatalf("request timeout_ms = %d, want 750", got)
		}
	case <-time.After(time.Second):
		t.Fatal("extension caller did not observe timeout")
	}
}

func TestApplyHookPatchPreservesConcreteStructType(t *testing.T) {
	t.Parallel()

	type promptPayload struct {
		PromptText string `json:"prompt_text"`
	}

	current := promptPayload{PromptText: "base"}
	next, err := applyHookPatch(current, patchPromptText(t, "patched"))
	if err != nil {
		t.Fatalf("applyHookPatch() error = %v", err)
	}

	payload, ok := next.(promptPayload)
	if !ok {
		t.Fatalf("applyHookPatch() type = %T, want %T", next, current)
	}
	if payload.PromptText != "patched" {
		t.Fatalf("PromptText = %q, want %q", payload.PromptText, "patched")
	}
}

func TestApplyHookPatchRejectsInvalidPatchShape(t *testing.T) {
	t.Parallel()

	_, err := applyHookPatch(map[string]any{"prompt_text": "base"}, json.RawMessage(`"bad"`))
	if err == nil {
		t.Fatal("applyHookPatch() error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "decode hook patch") {
		t.Fatalf("applyHookPatch() error = %q, want decode hook patch context", err.Error())
	}

	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("applyHookPatch() error = %T, want wrapped *json.UnmarshalTypeError", err)
	}
	if typeErr.Value != "string" {
		t.Fatalf("unmarshal error value = %q, want %q", typeErr.Value, "string")
	}
	if got := typeErr.Type.String(); got != "map[string]json.RawMessage" {
		t.Fatalf("unmarshal error type = %q, want %q", got, "map[string]json.RawMessage")
	}
}

func TestDecodeJSONLikeHandlesPointerAndInterfaceTemplates(t *testing.T) {
	t.Parallel()

	type promptPayload struct {
		PromptText string `json:"prompt_text"`
	}

	pointerTemplate := &promptPayload{}
	pointerResult, err := decodeJSONLike(pointerTemplate, []byte(`{"prompt_text":"pointer"}`))
	if err != nil {
		t.Fatalf("decodeJSONLike(pointer) error = %v", err)
	}

	decodedPointer, ok := pointerResult.(*promptPayload)
	if !ok {
		t.Fatalf("decodeJSONLike(pointer) type = %T, want *promptPayload", pointerResult)
	}
	if decodedPointer.PromptText != "pointer" {
		t.Fatalf("pointer PromptText = %q, want %q", decodedPointer.PromptText, "pointer")
	}

	var interfaceTemplate any
	interfaceResult, err := decodeJSONLike(interfaceTemplate, []byte(`{"prompt_text":"iface"}`))
	if err != nil {
		t.Fatalf("decodeJSONLike(interface) error = %v", err)
	}

	if got := promptTextFromPayload(t, interfaceResult); got != "iface" {
		t.Fatalf("interface prompt_text = %q, want %q", got, "iface")
	}
}

func TestRegistryAddRejectsDuplicateNames(t *testing.T) {
	t.Parallel()

	registry := mustRegistry(t, newRuntimeExtension("alpha", []Capability{CapabilityPromptMutate}))
	err := registry.Add(newRuntimeExtension("alpha", []Capability{CapabilityPromptMutate}))
	if err == nil {
		t.Fatal("Add() error = nil, want duplicate failure")
	}
}

func TestRuntimeExtensionShutdownDeadlineFallsBackToManifest(t *testing.T) {
	t.Parallel()

	extension := &RuntimeExtension{
		Ref: Ref{Name: "ref-name"},
		Manifest: &Manifest{
			Extension:  ExtensionInfo{Name: "manifest-name"},
			Subprocess: &SubprocessConfig{ShutdownTimeout: 7 * time.Second},
		},
	}

	if got := extension.ShutdownDeadline(); got != 7*time.Second {
		t.Fatalf("ShutdownDeadline() = %v, want %v", got, 7*time.Second)
	}
	if got := extension.normalizedName(); got != "ref-name" {
		t.Fatalf("normalizedName() = %q, want %q", got, "ref-name")
	}

	extension.SetShutdownDeadline(3 * time.Second)
	if got := extension.ShutdownDeadline(); got != 3*time.Second {
		t.Fatalf("ShutdownDeadline() after override = %v, want %v", got, 3*time.Second)
	}
}

func TestDispatchMutableIntegrationPriorityChain(t *testing.T) {
	t.Parallel()

	dispatcher := NewHookDispatcher(
		mustRegistry(t,
			newPromptAppenderExtension(t, "first", 100, true, "-A"),
			newPromptAppenderExtension(t, "second", 500, true, "-B"),
			newPromptAppenderExtension(t, "third", 900, true, "-C"),
		),
		&auditSpy{},
	)

	result, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "prompt",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	if got := promptTextFromPayload(t, result); got != "prompt-A-B-C" {
		t.Fatalf("final prompt_text = %q, want %q", got, "prompt-A-B-C")
	}
}

func TestDispatchMutableIntegrationOptionalMiddleFailure(t *testing.T) {
	t.Parallel()

	dispatcher := NewHookDispatcher(
		mustRegistry(t,
			newPromptAppenderExtension(t, "first", 100, true, "-A"),
			newRuntimeExtensionWithCaller(
				"second",
				[]Capability{CapabilityPromptMutate},
				&fakeExtensionCaller{
					handler: func(_ context.Context, _ executeHookRequest) (json.RawMessage, error) {
						return nil, errors.New("boom")
					},
				},
				HookDeclaration{Event: HookPromptPostBuild, Priority: 500, Required: false},
			),
			newPromptAppenderExtension(t, "third", 900, true, "-C"),
		),
		&auditSpy{},
	)

	result, err := dispatcher.DispatchMutable(context.Background(), HookPromptPostBuild, map[string]any{
		"prompt_text": "prompt",
	})
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	if got := promptTextFromPayload(t, result); got != "prompt-A-C" {
		t.Fatalf("final prompt_text = %q, want %q", got, "prompt-A-C")
	}
}

type fakeExtensionCaller struct {
	handler func(ctx context.Context, request executeHookRequest) (json.RawMessage, error)
}

func mustReceiveString(t *testing.T, ch <-chan string, timeout time.Duration, label string) string {
	t.Helper()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case got := <-ch:
		return got
	case <-timer.C:
		t.Fatalf("%s was not called", label)
		return ""
	}
}

func (f *fakeExtensionCaller) Call(ctx context.Context, method string, params any, result any) error {
	if method != executeHookMethod {
		return errors.New("unexpected method")
	}

	request, ok := params.(executeHookRequest)
	if !ok {
		return errors.New("unexpected params")
	}

	var patch json.RawMessage
	var err error
	if f != nil && f.handler != nil {
		patch, err = f.handler(ctx, request)
		if err != nil {
			return err
		}
	}

	response, ok := result.(*executeHookResponse)
	if !ok {
		return errors.New("unexpected result")
	}
	response.Patch = patch
	return nil
}

type auditSpy struct {
	mu      sync.Mutex
	records []AuditEntry
}

func (s *auditSpy) Record(entry AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records = append(s.records, entry)
	return nil
}

func (s *auditSpy) entries() []AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]AuditEntry(nil), s.records...)
}

func mustRegistry(t *testing.T, extensions ...*RuntimeExtension) *Registry {
	t.Helper()

	registry, err := NewRegistry(extensions...)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func newRuntimeExtension(name string, capabilities []Capability, hooks ...HookDeclaration) *RuntimeExtension {
	return newRuntimeExtensionWithCaller(name, capabilities, &fakeExtensionCaller{}, hooks...)
}

func newRuntimeExtensionWithCaller(
	name string,
	capabilities []Capability,
	caller ExtensionCaller,
	hooks ...HookDeclaration,
) *RuntimeExtension {
	extension := &RuntimeExtension{
		Name:         name,
		Caller:       caller,
		Capabilities: NewCapabilityChecker(capabilities),
		Manifest: &Manifest{
			Extension: ExtensionInfo{Name: name},
			Hooks:     append([]HookDeclaration(nil), hooks...),
		},
	}
	extension.SetState(ExtensionStateReady)
	return extension
}

func newPromptAppenderExtension(
	t *testing.T,
	name string,
	priority int,
	required bool,
	suffix string,
) *RuntimeExtension {
	t.Helper()

	return newRuntimeExtensionWithCaller(
		name,
		[]Capability{CapabilityPromptMutate},
		&fakeExtensionCaller{
			handler: func(_ context.Context, request executeHookRequest) (json.RawMessage, error) {
				return patchPromptText(t, promptTextFromPayload(t, request.Payload)+suffix), nil
			},
		},
		HookDeclaration{
			Event:    HookPromptPostBuild,
			Priority: priority,
			Required: required,
		},
	)
}

func promptTextFromPayload(t *testing.T, payload any) string {
	t.Helper()

	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}

	value, ok := decoded["prompt_text"].(string)
	if !ok {
		t.Fatalf("payload prompt_text missing or not a string: %#v", decoded)
	}
	return value
}

func patchPromptText(t *testing.T, promptText string) json.RawMessage {
	t.Helper()

	patch, err := json.Marshal(map[string]any{"prompt_text": promptText})
	if err != nil {
		t.Fatalf("json.Marshal(patch) error = %v", err)
	}
	return patch
}

func observerExtensionName(index int) string {
	return fmt.Sprintf("observer-%02d", index)
}
