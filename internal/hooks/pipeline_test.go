package hooks

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

type pipelineTestPayload struct {
	Value  string `json:"value,omitempty"`
	Denied bool   `json:"denied,omitempty"`
}

type pipelineTestPatch struct {
	Append string `json:"append,omitempty"`
	Deny   bool   `json:"deny,omitempty"`
}

type pipelineTestExecutor struct {
	kind    HookExecutorKind
	execute func(context.Context, RegisteredHook, []byte) ([]byte, error)
}

func (e pipelineTestExecutor) Kind() HookExecutorKind {
	return e.kind
}

func (e pipelineTestExecutor) Execute(ctx context.Context, hook RegisteredHook, payload []byte) ([]byte, error) {
	return e.execute(ctx, hook, payload)
}

func TestPipelineExecuteSequentialComposition(t *testing.T) {
	t.Parallel()

	seen := make([]string, 0, 3)
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						seen = append(seen, payload.Value)
						return pipelineTestPatch{Append: "A"}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						seen = append(seen, payload.Value)
						return pipelineTestPatch{Append: "B"}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-3", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						seen = append(seen, payload.Value)
						return pipelineTestPatch{Append: "C"}, nil
					},
				)),
			}
		},
		apply:  applyPipelineTestPatch,
		encode: failPipelineEncode(t),
		decode: failPipelineDecode(t),
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Value != "ABC" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "ABC")
	}
	if got := strings.Join(seen, ","); got != ",A,AB" {
		t.Fatalf("payload sequence = %q, want %q", got, ",A,AB")
	}
}

func TestPipelineExecuteSortsUnorderedHooks(t *testing.T) {
	t.Parallel()

	seen := make([]string, 0, 2)
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				{
					RegisteredHook: RegisteredHook{
						Name:     "hook-b",
						Event:    HookSessionPreCreate,
						Source:   HookSourceConfig,
						Mode:     HookModeSync,
						Priority: 10,
						Executor: NewTypedNativeExecutor(
							func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
								seen = append(seen, payload.Value)
								return pipelineTestPatch{Append: "B"}, nil
							},
						),
					},
					Decl: HookDecl{
						Name:         "hook-b",
						Event:        HookSessionPreCreate,
						Source:       HookSourceConfig,
						Mode:         HookModeSync,
						Priority:     10,
						ExecutorKind: HookExecutorNative,
					},
				},
				{
					RegisteredHook: RegisteredHook{
						Name:     "hook-a",
						Event:    HookSessionPreCreate,
						Source:   HookSourceNative,
						Mode:     HookModeSync,
						Priority: 100,
						Executor: NewTypedNativeExecutor(
							func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
								seen = append(seen, payload.Value)
								return pipelineTestPatch{Append: "A"}, nil
							},
						),
					},
					Decl: HookDecl{
						Name:         "hook-a",
						Event:        HookSessionPreCreate,
						Source:       HookSourceNative,
						Mode:         HookModeSync,
						Priority:     100,
						ExecutorKind: HookExecutorNative,
					},
				},
			}
		},
		apply:  applyPipelineTestPatch,
		encode: failPipelineEncode(t),
		decode: failPipelineDecode(t),
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Value != "AB" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "AB")
	}
	if got := strings.Join(seen, ","); got != ",A" {
		t.Fatalf("payload sequence = %q, want %q", got, ",A")
	}
}

func TestPipelineExecuteShortCircuitsOnExplicitDeny(t *testing.T) {
	t.Parallel()

	var thirdExecuted bool
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ pipelineTestPayload) (pipelineTestPatch, error) {
						return pipelineTestPatch{Append: "A"}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						if payload.Value != "A" {
							t.Fatalf("hook-2 payload.Value = %q, want %q", payload.Value, "A")
						}
						return pipelineTestPatch{Append: "B", Deny: true}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-3", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ pipelineTestPayload) (pipelineTestPatch, error) {
						thirdExecuted = true
						return pipelineTestPatch{Append: "C"}, nil
					},
				)),
			}
		},
		apply:  applyPipelineTestPatch,
		encode: failPipelineEncode(t),
		decode: failPipelineDecode(t),
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if !result.Denied {
		t.Fatal("result.Denied = false, want true")
	}
	if result.Value != "AB" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "AB")
	}
	if thirdExecuted {
		t.Fatal("third hook executed after explicit deny")
	}
}

func TestPipelineExecuteRequiredHookTimeoutStopsPipeline(t *testing.T) {
	t.Parallel()

	var thirdExecuted bool
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ pipelineTestPayload) (pipelineTestPatch, error) {
						return pipelineTestPatch{Append: "A"}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-2", true, 20*time.Millisecond, NewTypedNativeExecutor(
					func(ctx context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						if payload.Value != "A" {
							t.Fatalf("hook-2 payload.Value = %q, want %q", payload.Value, "A")
						}
						<-ctx.Done()
						return pipelineTestPatch{}, ctx.Err()
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-3", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ pipelineTestPayload) (pipelineTestPatch, error) {
						thirdExecuted = true
						return pipelineTestPatch{Append: "C"}, nil
					},
				)),
			}
		},
		apply:  applyPipelineTestPatch,
		encode: failPipelineEncode(t),
		decode: failPipelineDecode(t),
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("execute() error = %v, want context deadline exceeded", err)
	}
	if result.Value != "A" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "A")
	}
	if thirdExecuted {
		t.Fatal("third hook executed after required hook timeout")
	}
}

func TestPipelineExecuteSkipsNonRequiredHookFailure(t *testing.T) {
	t.Parallel()

	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ pipelineTestPayload) (pipelineTestPatch, error) {
						return pipelineTestPatch{Append: "A"}, nil
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						if payload.Value != "A" {
							t.Fatalf("hook-2 payload.Value = %q, want %q", payload.Value, "A")
						}
						return pipelineTestPatch{}, errors.New("boom")
					},
				)),
				testPipelineHook(HookSessionPreCreate, "hook-3", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						if payload.Value != "A" {
							t.Fatalf("hook-3 payload.Value = %q, want %q", payload.Value, "A")
						}
						return pipelineTestPatch{Append: "B"}, nil
					},
				)),
			}
		},
		apply:  applyPipelineTestPatch,
		encode: failPipelineEncode(t),
		decode: failPipelineDecode(t),
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Value != "AB" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "AB")
	}
}

func TestPipelineExecuteWithNoMatchingHooksReturnsOriginalPayload(t *testing.T) {
	t.Parallel()

	var encodeCalled bool
	var decodeCalled bool
	original := pipelineTestPayload{Value: "original"}
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return nil
		},
		apply: applyPipelineTestPatch,
		encode: func(payload pipelineTestPayload) ([]byte, error) {
			encodeCalled = true
			return encodeJSON(payload)
		},
		decode: func(payload []byte) (pipelineTestPatch, error) {
			decodeCalled = true
			return decodeJSON[pipelineTestPatch](payload)
		},
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), original)
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result != original {
		t.Fatalf("result = %#v, want %#v", result, original)
	}
	if encodeCalled {
		t.Fatal("encode called with no matching hooks")
	}
	if decodeCalled {
		t.Fatal("decode called with no matching hooks")
	}
}

func TestEnterDispatchDepthOneSucceeds(t *testing.T) {
	t.Parallel()

	ctx, depth, err := enterDispatch(t.Context(), HookSessionPreCreate)
	if err != nil {
		t.Fatalf("enterDispatch() error = %v, want nil", err)
	}
	if depth != 1 {
		t.Fatalf("depth = %d, want 1", depth)
	}
	if got := currentDispatchDepth(ctx); got != 1 {
		t.Fatalf("currentDispatchDepth() = %d, want 1", got)
	}
}

func TestEnterDispatchDepthThreeSucceeds(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	for i := 1; i <= 3; i++ {
		var depth int
		var err error
		ctx, depth, err = enterDispatch(ctx, HookSessionPreCreate)
		if err != nil {
			t.Fatalf("enterDispatch() iteration %d error = %v, want nil", i, err)
		}
		if depth != i {
			t.Fatalf("depth = %d, want %d", depth, i)
		}
	}
}

func TestEnterDispatchDepthFourFails(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	for range 3 {
		var err error
		ctx, _, err = enterDispatch(ctx, HookSessionPreCreate)
		if err != nil {
			t.Fatalf("enterDispatch() setup error = %v, want nil", err)
		}
	}

	_, depth, err := enterDispatch(ctx, HookSessionPreCreate)
	if !errors.Is(err, ErrDispatchDepthExceeded) {
		t.Fatalf("enterDispatch() error = %v, want ErrDispatchDepthExceeded", err)
	}
	if depth != 3 {
		t.Fatalf("depth = %d, want 3", depth)
	}
}

func TestEnterDispatchNestedIncrementsFromParentContext(t *testing.T) {
	t.Parallel()

	parent, _, err := enterDispatch(t.Context(), HookSessionPreCreate)
	if err != nil {
		t.Fatalf("enterDispatch() parent error = %v, want nil", err)
	}

	child, depth, err := enterDispatch(parent, HookToolPreCall)
	if err != nil {
		t.Fatalf("enterDispatch() child error = %v, want nil", err)
	}
	if depth != 2 {
		t.Fatalf("depth = %d, want 2", depth)
	}
	if got := currentDispatchDepth(child); got != 2 {
		t.Fatalf("currentDispatchDepth() = %d, want 2", got)
	}
}

func TestPermissionPipelineKeepDenyAllowed(t *testing.T) {
	t.Parallel()

	var secondExecuted bool
	pipe := permissionPipeline(
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		func(PermissionRequestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookPermissionRequest, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ PermissionRequestPayload) (PermissionRequestPatch, error) {
						return PermissionRequestPatch{ControlPatch: ControlPatch{Deny: true}}, nil
					},
				)),
				testPipelineHook(HookPermissionRequest, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ PermissionRequestPayload) (PermissionRequestPatch, error) {
						secondExecuted = true
						return PermissionRequestPatch{}, nil
					},
				)),
			}
		},
	)

	result, err := pipe.execute(t.Context(), PermissionRequestPayload{Decision: "deny"})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Decision != "deny" {
		t.Fatalf("result.Decision = %q, want %q", result.Decision, "deny")
	}
	if secondExecuted {
		t.Fatal("second hook executed after deny")
	}
}

func TestPermissionPipelineRejectsDenyToAllowAndLogs(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	var secondHookSawDecision string
	pipe := permissionPipeline(
		logger,
		func(PermissionRequestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookPermissionRequest, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ PermissionRequestPayload) (PermissionRequestPatch, error) {
						allow := "allow"
						return PermissionRequestPatch{Decision: &allow}, nil
					},
				)),
				testPipelineHook(HookPermissionRequest, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload PermissionRequestPayload) (PermissionRequestPatch, error) {
						secondHookSawDecision = payload.Decision
						return PermissionRequestPatch{}, nil
					},
				)),
			}
		},
	)

	result, err := pipe.execute(t.Context(), PermissionRequestPayload{Decision: "deny"})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Decision != "deny" {
		t.Fatalf("result.Decision = %q, want %q", result.Decision, "deny")
	}
	if secondHookSawDecision != "deny" {
		t.Fatalf("second hook saw decision %q, want %q", secondHookSawDecision, "deny")
	}
	if !strings.Contains(logs.String(), "hook.dispatch.permission_escalation_blocked") {
		t.Fatalf("logs = %q, want escalation-blocked entry", logs.String())
	}
}

func TestPermissionPipelineAllowToDenyIsAllowed(t *testing.T) {
	t.Parallel()

	var secondExecuted bool
	pipe := permissionPipeline(
		slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
		func(PermissionRequestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookPermissionRequest, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ PermissionRequestPayload) (PermissionRequestPatch, error) {
						deny := "deny"
						return PermissionRequestPatch{Decision: &deny}, nil
					},
				)),
				testPipelineHook(HookPermissionRequest, "hook-2", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, _ PermissionRequestPayload) (PermissionRequestPatch, error) {
						secondExecuted = true
						return PermissionRequestPatch{}, nil
					},
				)),
			}
		},
	)

	result, err := pipe.execute(t.Context(), PermissionRequestPayload{Decision: "allow"})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Decision != "deny" {
		t.Fatalf("result.Decision = %q, want %q", result.Decision, "deny")
	}
	if secondExecuted {
		t.Fatal("second hook executed after allow-to-deny patch")
	}
}

func TestPipelineNativeExecutorSkipsSerialization(t *testing.T) {
	t.Parallel()

	var encodeCalls int
	var decodeCalls int
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, NewTypedNativeExecutor(
					func(_ context.Context, _ RegisteredHook, payload pipelineTestPayload) (pipelineTestPatch, error) {
						if payload.Value != "seed" {
							t.Fatalf("payload.Value = %q, want %q", payload.Value, "seed")
						}
						return pipelineTestPatch{Append: "-native"}, nil
					},
				)),
			}
		},
		apply: applyPipelineTestPatch,
		encode: func(payload pipelineTestPayload) ([]byte, error) {
			encodeCalls++
			return encodeJSON(payload)
		},
		decode: func(payload []byte) (pipelineTestPatch, error) {
			decodeCalls++
			return decodeJSON[pipelineTestPatch](payload)
		},
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{Value: "seed"})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Value != "seed-native" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "seed-native")
	}
	if encodeCalls != 0 {
		t.Fatalf("encodeCalls = %d, want 0", encodeCalls)
	}
	if decodeCalls != 0 {
		t.Fatalf("decodeCalls = %d, want 0", decodeCalls)
	}
}

func TestPipelineSubprocessExecutorUsesEncodeDecode(t *testing.T) {
	t.Parallel()

	var encodeCalls int
	var decodeCalls int
	var rawPayload string
	pipe := pipeline[pipelineTestPayload, pipelineTestPatch]{
		event: HookSessionPreCreate,
		hooks: func(pipelineTestPayload) []*ResolvedHook {
			return []*ResolvedHook{
				testPipelineHook(HookSessionPreCreate, "hook-1", false, 0, pipelineTestExecutor{
					kind: HookExecutorSubprocess,
					execute: func(_ context.Context, _ RegisteredHook, payload []byte) ([]byte, error) {
						rawPayload = string(payload)
						return []byte(`{"append":"-subprocess"}`), nil
					},
				}),
			}
		},
		apply: applyPipelineTestPatch,
		encode: func(payload pipelineTestPayload) ([]byte, error) {
			encodeCalls++
			return encodeJSON(payload)
		},
		decode: func(payload []byte) (pipelineTestPatch, error) {
			decodeCalls++
			return decodeJSON[pipelineTestPatch](payload)
		},
		denied: func(patch pipelineTestPatch) bool { return patch.Deny },
	}

	result, err := pipe.execute(t.Context(), pipelineTestPayload{Value: "seed"})
	if err != nil {
		t.Fatalf("execute() error = %v, want nil", err)
	}
	if result.Value != "seed-subprocess" {
		t.Fatalf("result.Value = %q, want %q", result.Value, "seed-subprocess")
	}
	if encodeCalls != 1 {
		t.Fatalf("encodeCalls = %d, want 1", encodeCalls)
	}
	if decodeCalls != 1 {
		t.Fatalf("decodeCalls = %d, want 1", decodeCalls)
	}
	if !strings.Contains(rawPayload, `"value":"seed"`) {
		t.Fatalf("rawPayload = %q, want encoded JSON payload", rawPayload)
	}
}

func applyPipelineTestPatch(payload pipelineTestPayload, patch pipelineTestPatch) pipelineTestPayload {
	payload.Value += patch.Append
	if patch.Deny {
		payload.Denied = true
	}
	return payload
}

func applyPermissionRequestPatch(
	payload PermissionRequestPayload,
	patch PermissionRequestPatch,
) PermissionRequestPayload {
	if patch.Decision != nil {
		payload.Decision = *patch.Decision
	}
	if patch.Deny {
		payload.Decision = "deny"
	}
	if patch.DecisionClass != nil {
		payload.DecisionClass = *patch.DecisionClass
	}
	return payload
}

func permissionPipeline(
	logger *slog.Logger,
	hooks func(PermissionRequestPayload) []*ResolvedHook,
) pipeline[PermissionRequestPayload, PermissionRequestPatch] {
	return pipeline[PermissionRequestPayload, PermissionRequestPatch]{
		event:  HookPermissionRequest,
		hooks:  hooks,
		apply:  applyPermissionRequestPatch,
		encode: failPermissionEncode,
		decode: failPermissionDecode,
		denied: permissionPatchDenies,
		guard:  newPermissionRequestGuard(logger, nil),
	}
}

func testPipelineHook(
	event HookEvent,
	name string,
	required bool,
	timeout time.Duration,
	executor Executor,
) *ResolvedHook {
	source := HookSourceConfig
	command := "hook"
	if executor.Kind() == HookExecutorNative {
		source = HookSourceNative
		command = ""
	}

	return &ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:     name,
			Event:    event,
			Source:   source,
			Mode:     HookModeSync,
			Required: required,
			Timeout:  timeout,
			Executor: executor,
		},
		Decl: HookDecl{
			Name:         name,
			Event:        event,
			Source:       source,
			Mode:         HookModeSync,
			Required:     required,
			Timeout:      timeout,
			ExecutorKind: executor.Kind(),
			Command:      command,
		},
	}
}

func failPipelineEncode(t *testing.T) func(pipelineTestPayload) ([]byte, error) {
	t.Helper()

	return func(pipelineTestPayload) ([]byte, error) {
		t.Fatal("encode should not be called")
		return nil, nil
	}
}

func failPipelineDecode(t *testing.T) func([]byte) (pipelineTestPatch, error) {
	t.Helper()

	return func([]byte) (pipelineTestPatch, error) {
		t.Fatal("decode should not be called")
		return pipelineTestPatch{}, nil
	}
}

func failPermissionEncode(PermissionRequestPayload) ([]byte, error) {
	return nil, errors.New("permission pipeline should not serialize native hooks in tests")
}

func failPermissionDecode([]byte) (PermissionRequestPatch, error) {
	return PermissionRequestPatch{}, errors.New("permission pipeline should not deserialize native hooks in tests")
}
