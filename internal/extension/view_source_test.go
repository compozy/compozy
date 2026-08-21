package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestManagerViewProgramForwarding(t *testing.T) {
	t.Parallel()

	t.Run("Should open a view through the negotiated method and preserve generation", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(9201)
		process.callFn = func(_ context.Context, method string, params, result any) error {
			if got, want := method, string(extensionprotocol.ExtensionServiceMethodViewOpen); got != want {
				t.Fatalf("Call() method = %q, want %q", got, want)
			}
			request, ok := params.(cmdpalette.ViewOpenRequest)
			if !ok || request.View != "browser" || request.Workspace != "workspace-a" {
				t.Fatalf("Call() params = %#v, want view open request", params)
			}
			frame, ok := result.(*cmdpalette.ViewFrame)
			if !ok {
				t.Fatalf("Call() result type = %T, want *ViewFrame", result)
			}
			frame.ViewSession = request.ViewSession
			frame.Revision = "rev-1"
			frame.Handlers = []string{"search"}
			return nil
		}
		manager := newViewProgramManagerForTest("notes", 7, 50*time.Millisecond, process)
		frame, generation, err := manager.OpenProgram(testutil.Context(t), "notes", cmdpalette.ViewOpenRequest{
			ViewSession: "vs_test", View: "browser", Workspace: "workspace-a",
		})
		if err != nil {
			t.Fatalf("OpenProgram() error = %v", err)
		}
		if generation != 7 || frame.Revision != "rev-1" || frame.ViewSession != "vs_test" {
			t.Fatalf("OpenProgram() = %#v generation %d, want rev-1 and generation 7", frame, generation)
		}
	})

	t.Run("Should treat empty and null event payloads as acknowledgements", func(t *testing.T) {
		t.Parallel()
		for _, name := range []struct {
			name    string
			payload json.RawMessage
		}{
			{name: "empty", payload: nil},
			{name: "null", payload: json.RawMessage("null")},
			{name: "object", payload: json.RawMessage("{}")},
		} {
			t.Run("Should acknowledge "+name.name+" event payload", func(t *testing.T) {
				t.Parallel()
				process := newFakeProcess(9202)
				process.callFn = func(_ context.Context, method string, _ any, result any) error {
					if got, want := method, string(extensionprotocol.ExtensionServiceMethodViewEvent); got != want {
						t.Fatalf("Call() method = %q, want %q", got, want)
					}
					raw, ok := result.(*json.RawMessage)
					if !ok {
						t.Fatalf("Call() result type = %T, want *json.RawMessage", result)
					}
					if name.payload != nil {
						*raw = append(json.RawMessage(nil), name.payload...)
					}
					return nil
				}
				manager := newViewProgramManagerForTest("notes", 1, time.Second, process)
				frame, err := manager.HandleProgramEvent(
					testutil.Context(t),
					"workspace-a",
					"notes",
					cmdpalette.ViewEvent{
						ViewSession: "vs_test", Handler: "search", Revision: "rev-1", Seq: 1,
					},
				)
				if err != nil {
					t.Fatalf("HandleProgramEvent() error = %v", err)
				}
				if frame != nil {
					t.Fatalf("HandleProgramEvent() = %#v, want acknowledgement-only nil frame", frame)
				}
			})
		}
	})

	t.Run("Should close a view through the negotiated method", func(t *testing.T) {
		t.Parallel()
		called := false
		process := newFakeProcess(9203)
		process.callFn = func(_ context.Context, method string, params, _ any) error {
			called = true
			if got, want := method, string(extensionprotocol.ExtensionServiceMethodViewClose); got != want {
				t.Fatalf("Call() method = %q, want %q", got, want)
			}
			request, ok := params.(cmdpalette.ViewCloseRequest)
			if !ok || request.ViewSession != "vs_test" {
				t.Fatalf("Call() params = %#v, want close request", params)
			}
			return nil
		}
		manager := newViewProgramManagerForTest("notes", 1, time.Second, process)
		if err := manager.CloseProgram(testutil.Context(t), "workspace-a", "notes", cmdpalette.ViewCloseRequest{
			ViewSession: "vs_test",
		}); err != nil {
			t.Fatalf("CloseProgram() error = %v", err)
		}
		if !called {
			t.Fatal("CloseProgram() did not call view/close")
		}
	})

	t.Run("Should cancel hanging view calls with the configured timeout", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(9204)
		process.callFn = func(ctx context.Context, _ string, _ any, _ any) error {
			<-ctx.Done()
			return ctx.Err()
		}
		manager := newViewProgramManagerForTest("notes", 1, 20*time.Millisecond, process)
		_, _, err := manager.OpenProgram(testutil.Context(t), "notes", cmdpalette.ViewOpenRequest{
			ViewSession: "vs_test", View: "browser", Workspace: "workspace-a",
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("OpenProgram() error = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("Should fail closed when the view process is unavailable", func(t *testing.T) {
		t.Parallel()
		manager := newViewProgramManagerForTest("notes", 1, time.Second, nil)
		_, _, err := manager.OpenProgram(testutil.Context(t), "notes", cmdpalette.ViewOpenRequest{
			ViewSession: "vs_test", View: "browser", Workspace: "workspace-a",
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("OpenProgram() error = %v, want ErrToolUnavailable", err)
		}
	})

	t.Run("Should treat a stale generation as an unavailable process", func(t *testing.T) {
		t.Parallel()
		manager := newViewProgramManagerForTest("notes", -1, time.Second, newFakeProcess(9205))
		_, _, err := manager.OpenProgram(testutil.Context(t), "notes", cmdpalette.ViewOpenRequest{
			ViewSession: "vs_test", View: "browser", Workspace: "workspace-a",
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("OpenProgram() error = %v, want ErrToolUnavailable", err)
		}
	})

	t.Run("Should treat close as idempotent when the process is gone", func(t *testing.T) {
		t.Parallel()
		manager := newViewProgramManagerForTest("notes", 1, time.Second, nil)
		if err := manager.CloseProgram(testutil.Context(t), "workspace-a", "notes", cmdpalette.ViewCloseRequest{
			ViewSession: "vs_test",
		}); err != nil {
			t.Fatalf("CloseProgram() error = %v, want nil for unavailable process", err)
		}
	})
}

func newViewProgramManagerForTest(
	name string,
	generation int64,
	timeout time.Duration,
	process processHandle,
) *Manager {
	return &Manager{
		defaultViewTimeout: timeout,
		extensions: map[string]*managedExtension{
			name: {
				info: ExtensionInfo{
					Name: name,
					Capabilities: CapabilitiesConfig{
						Provides: []string{extensionprotocol.CapabilityProvideViewProvider},
					},
				},
				active:     true,
				generation: generation,
				process:    process,
				initialize: &subprocess.InitializeResponse{
					AcceptedCapabilities: subprocess.AcceptedCapabilities{
						Provides: []string{extensionprotocol.CapabilityProvideViewProvider},
					},
					ImplementedMethods: []string{
						string(extensionprotocol.ExtensionServiceMethodViewOpen),
						string(extensionprotocol.ExtensionServiceMethodViewEvent),
						string(extensionprotocol.ExtensionServiceMethodViewClose),
					},
				},
			},
		},
	}
}
