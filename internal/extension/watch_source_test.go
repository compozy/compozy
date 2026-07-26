package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestManagerWatchPoll(t *testing.T) {
	t.Parallel()

	t.Run("Should reject watch poll when extension lacks watch capability", func(t *testing.T) {
		t.Parallel()

		manager := newWatchSourceManagerForTest(
			"ext-watch",
			nil,
			[]string{string(extensionprotocol.ExtensionServiceMethodWatchPoll)},
			[]string{"reviews"},
			newFakeProcess(9101),
		)

		_, err := manager.WatchPoll(testutil.Context(t), "ext-watch", watchpkg.PollRequest{
			Spec: json.RawMessage(`{"kind":"reviews"}`),
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("WatchPoll() error = %v, want ErrToolUnavailable", err)
		}
	})

	t.Run("Should call watch poll when capability and method are negotiated", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(9102)
		process.callFn = func(_ context.Context, method string, params, result any) error {
			if got, want := method, string(extensionprotocol.ExtensionServiceMethodWatchPoll); got != want {
				t.Fatalf("Call() method = %q, want %q", got, want)
			}
			req, ok := params.(watchpkg.PollRequest)
			if !ok {
				t.Fatalf("Call() params type = %T, want watch PollRequest", params)
			}
			if got, want := string(req.Spec), `{"kind":"reviews"}`; got != want {
				t.Fatalf("PollRequest.Spec = %s, want %s", got, want)
			}
			if req.ExpectedStateDigest != "sha256:prev" {
				t.Fatalf("ExpectedStateDigest = %q, want sha256:prev", req.ExpectedStateDigest)
			}
			response, ok := result.(*watchpkg.PollResponse)
			if !ok {
				t.Fatalf("Call() result type = %T, want *watch.PollResponse", result)
			}
			response.Ready = true
			response.StateDigest = "sha256:next"
			response.Payload = json.RawMessage(`{"review":"r1"}`)
			return nil
		}
		manager := newWatchSourceManagerForTest(
			"ext-watch",
			[]string{extensionprotocol.CapabilityProvideWatchSource},
			[]string{string(extensionprotocol.ExtensionServiceMethodWatchPoll)},
			[]string{"reviews"},
			process,
		)

		response, err := manager.WatchPoll(testutil.Context(t), "ext-watch", watchpkg.PollRequest{
			Spec:                json.RawMessage(`{"kind":"reviews"}`),
			ExpectedStateDigest: "sha256:prev",
		})
		if err != nil {
			t.Fatalf("WatchPoll() error = %v", err)
		}
		if !response.Ready {
			t.Fatal("response.Ready = false, want true")
		}
		if got, want := string(response.Payload), `{"review":"r1"}`; got != want {
			t.Fatalf("response.Payload = %s, want %s", got, want)
		}
		snapshot, err := manager.Get("ext-watch")
		if err != nil {
			t.Fatalf("Get(ext-watch) error = %v", err)
		}
		if got, want := snapshot.InitializeResult.WatchSourceKinds[0], "reviews"; got != want {
			t.Fatalf("WatchSourceKinds[0] = %q, want %q", got, want)
		}
	})

	t.Run("Should resolve poll by watch source kind", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(9103)
		process.callFn = func(context.Context, string, any, any) error {
			return nil
		}
		manager := newWatchSourceManagerForTest(
			"ext-watch",
			[]string{extensionprotocol.CapabilityProvideWatchSource},
			[]string{string(extensionprotocol.ExtensionServiceMethodWatchPoll)},
			[]string{"reviews"},
			process,
		)

		if _, err := manager.Poll(testutil.Context(t), watchpkg.PollRequest{
			Spec: json.RawMessage(`{"kind":"reviews"}`),
		}); err != nil {
			t.Fatalf("Poll() error = %v", err)
		}
	})
}

func newWatchSourceManagerForTest(
	name string,
	provides []string,
	methods []string,
	kinds []string,
	process processHandle,
) *Manager {
	return &Manager{
		extensions: map[string]*managedExtension{
			name: {
				info: ExtensionInfo{
					Name: name,
					Capabilities: CapabilitiesConfig{
						Provides: append([]string(nil), provides...),
					},
				},
				active:  true,
				process: process,
				initialize: &subprocess.InitializeResponse{
					AcceptedCapabilities: subprocess.AcceptedCapabilities{
						Provides: append([]string(nil), provides...),
					},
					ImplementedMethods: append([]string(nil), methods...),
					WatchSourceKinds:   append([]string(nil), kinds...),
				},
			},
		},
	}
}
