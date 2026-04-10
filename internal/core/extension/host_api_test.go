package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/subprocess"
)

func TestHostAPIRouterHandleMethodNotFoundForUnregisteredNamespace(t *testing.T) {
	t.Parallel()

	router := NewHostAPIRouter(
		mustRegistry(t, newHostAPIExtension("ext", []Capability{CapabilityTasksCreate}, ExtensionStateReady)),
		&auditSpy{},
	)

	_, err := router.Handle(context.Background(), "ext", "host.tasks.create", nil)
	assertRequestErrorCode(t, err, -32601)
}

func TestHostAPIRouterHandleCapabilityDenied(t *testing.T) {
	t.Parallel()

	router := NewHostAPIRouter(
		mustRegistry(t, newHostAPIExtension("ext", []Capability{CapabilityTasksRead}, ExtensionStateReady)),
		&auditSpy{},
	)
	if err := router.RegisterService("host.tasks", HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return map[string]any{"ok": true}, nil
	})); err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	_, err := router.Handle(context.Background(), "ext", "host.tasks.create", nil)
	assertRequestErrorCode(t, err, capabilityDeniedCode)
}

func TestHostAPIRouterHandleNotInitialized(t *testing.T) {
	t.Parallel()

	router := NewHostAPIRouter(
		mustRegistry(t, newHostAPIExtension("ext", []Capability{CapabilityTasksRead}, ExtensionStateLoaded)),
		&auditSpy{},
	)
	if err := router.RegisterService("tasks", HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return nil, nil
	})); err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	_, err := router.Handle(context.Background(), "ext", "host.tasks.list", nil)
	assertRequestErrorCode(t, err, notInitializedCode)
}

func TestHostAPIRouterHandleShutdownInProgress(t *testing.T) {
	t.Parallel()

	extension := newHostAPIExtension("ext", []Capability{CapabilityTasksRead}, ExtensionStateDraining)
	extension.SetShutdownDeadline(10 * time.Second)
	router := NewHostAPIRouter(mustRegistry(t, extension), &auditSpy{})
	if err := router.RegisterService("tasks", HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return nil, nil
	})); err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	_, err := router.Handle(context.Background(), "ext", "host.tasks.list", nil)
	assertRequestErrorCode(t, err, shutdownInProgressCode)
}

func TestHostAPIRouterHandleSuccessAndAudits(t *testing.T) {
	t.Parallel()

	audit := &auditSpy{}
	router := NewHostAPIRouter(
		mustRegistry(t, newHostAPIExtension("ext", []Capability{CapabilityTasksRead}, ExtensionStateReady)),
		audit,
	)

	var gotVerb string
	var gotParams map[string]any
	if err := router.RegisterService("host.tasks", HostAPIServiceFunc(func(
		_ context.Context,
		extension *RuntimeExtension,
		verb string,
		params json.RawMessage,
	) (any, error) {
		if extension.Name != "ext" {
			t.Fatalf("extension.Name = %q, want %q", extension.Name, "ext")
		}
		gotVerb = verb
		if err := json.Unmarshal(params, &gotParams); err != nil {
			t.Fatalf("json.Unmarshal(params) error = %v", err)
		}
		return map[string]any{"count": 3}, nil
	})); err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	result, err := router.Handle(
		context.Background(),
		"ext",
		"host.tasks.list",
		json.RawMessage(`{"workflow":"extensibility"}`),
	)
	if err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	decoded, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Handle() result type = %T, want map[string]any", result)
	}
	if gotVerb != "list" {
		t.Fatalf("verb = %q, want %q", gotVerb, "list")
	}
	if got := gotParams["workflow"]; got != "extensibility" {
		t.Fatalf("workflow param = %#v, want %q", got, "extensibility")
	}
	if got := decoded["count"]; got != 3 {
		t.Fatalf("result count = %#v, want 3", got)
	}

	entries := audit.entries()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Result != AuditResultOK {
		t.Fatalf("audit result = %q, want %q", entries[0].Result, AuditResultOK)
	}
}

func TestHostAPIRouterHandleAuditsServiceErrors(t *testing.T) {
	t.Parallel()

	audit := &auditSpy{}
	router := NewHostAPIRouter(
		mustRegistry(t, newHostAPIExtension("ext", []Capability{CapabilityTasksRead}, ExtensionStateReady)),
		audit,
	)

	if err := router.RegisterService("tasks", HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return nil, errors.New("service failed")
	})); err != nil {
		t.Fatalf("RegisterService() error = %v", err)
	}

	_, err := router.Handle(context.Background(), "ext", "host.tasks.list", nil)
	if err == nil {
		t.Fatal("Handle() error = nil, want failure")
	}

	entries := audit.entries()
	if len(entries) != 1 {
		t.Fatalf("audit entries = %d, want 1", len(entries))
	}
	if entries[0].Result != AuditResultError {
		t.Fatalf("audit result = %q, want %q", entries[0].Result, AuditResultError)
	}
}

func TestHostAPIRouterRegisterServiceRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	router := NewHostAPIRouter(mustRegistry(t), &auditSpy{})
	err := router.RegisterService("host.tasks.create", HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return nil, nil
	}))
	if err == nil {
		t.Fatal("RegisterService() error = nil, want invalid namespace")
	}
}

func TestHostAPIRouterRegisterServiceRejectsNilAndDuplicateHandlers(t *testing.T) {
	t.Parallel()

	router := NewHostAPIRouter(mustRegistry(t), &auditSpy{})
	if err := router.RegisterService("tasks", nil); err == nil {
		t.Fatal("RegisterService(nil) error = nil, want failure")
	}

	handler := HostAPIServiceFunc(func(
		_ context.Context,
		_ *RuntimeExtension,
		_ string,
		_ json.RawMessage,
	) (any, error) {
		return nil, nil
	})

	if err := router.RegisterService("tasks", handler); err != nil {
		t.Fatalf("first RegisterService() error = %v", err)
	}
	if err := router.RegisterService("host.tasks", handler); err == nil {
		t.Fatal("duplicate RegisterService() error = nil, want failure")
	}
}

func TestToRequestErrorConvertsUnknownCapabilityTarget(t *testing.T) {
	t.Parallel()

	err := toRequestError(&UnknownCapabilityTargetError{Kind: "method", Name: "host.unknown.call"}, "host.unknown.call")
	assertRequestErrorCode(t, err, -32601)
}

func newHostAPIExtension(name string, capabilities []Capability, state ExtensionState) *RuntimeExtension {
	extension := &RuntimeExtension{
		Name:         name,
		Capabilities: NewCapabilityChecker(capabilities),
		Manifest:     &Manifest{Extension: ExtensionInfo{Name: name}},
	}
	extension.SetState(state)
	return extension
}

func assertRequestErrorCode(t *testing.T, err error, want int) {
	t.Helper()

	var requestErr *subprocess.RequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("error type = %T, want *subprocess.RequestError", err)
	}
	if requestErr.Code != want {
		t.Fatalf("request error code = %d, want %d", requestErr.Code, want)
	}
}
