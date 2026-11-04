package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/heartbeat/source"
	watchpkg "github.com/compozy/compozy/internal/loop/watch"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
			response.EventKey = " reviews:r1 "
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
		if got, want := response.EventKey, "reviews:r1"; got != want {
			t.Fatalf("response.EventKey = %q, want %q", got, want)
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
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			response, ok := result.(*watchpkg.PollResponse)
			if !ok {
				t.Fatalf("Call() result type = %T, want *watch.PollResponse", result)
			}
			response.EventKey = "reviews:poll"
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

	t.Run("Should fail closed when extension omits event identity", func(t *testing.T) {
		t.Parallel()

		process := newFakeProcess(9104)
		process.callFn = func(context.Context, string, any, any) error { return nil }
		manager := newWatchSourceManagerForTest(
			"ext-watch",
			[]string{extensionprotocol.CapabilityProvideWatchSource},
			[]string{string(extensionprotocol.ExtensionServiceMethodWatchPoll)},
			[]string{"reviews"},
			process,
		)
		_, err := manager.WatchPoll(testutil.Context(t), "ext-watch", watchpkg.PollRequest{
			Spec: json.RawMessage(`{"kind":"reviews"}`),
		})
		if err == nil || !strings.Contains(err.Error(), "event_key") {
			t.Fatalf("WatchPoll() error = %v, want event_key validation", err)
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

func TestManagerHeartbeatSource(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, workspace, kind, params, code string
		fail                                error
	}{
		{name: "Should poll a namespaced kind", kind: "ext/issues", params: `{"repo":"a/b"}`},
		{name: "Should reject invalid params before polling", kind: "ext/issues", params: `{"repo":3}`, code: "source_params_invalid"},
		{name: "Should reject unregistered kinds", kind: "other/issues", params: `{}`, code: "source_kind_unavailable"},
		{name: "Should map process crashes", kind: "ext/issues", params: `{"repo":"a/b"}`, fail: errors.New("process exited"), code: "source_extension_failed"},
		{name: "Should map typed rate limits", kind: "ext/issues", params: `{"repo":"a/b"}`, fail: &subprocess.RPCError{Code: -32000, Data: json.RawMessage(`{"code":"rate_limited"}`)}, code: "source_rate_limited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			process := newFakeProcess(9201)
			process.callFn = func(_ context.Context, method string, params, result any) error {
				if method != "heartbeat/source_poll" {
					t.Fatalf("method=%s", method)
				}
				req := params.(source.Request)
				if req.Kind != "issues" {
					t.Fatalf("kind=%s", req.Kind)
				}
				if tc.fail != nil {
					return tc.fail
				}
				*result.(*source.Result) = source.Result{Items: []source.Item{{Key: "1", Title: "New issue"}}}
				return nil
			}
			m := newWatchSourceManagerForTest(
				"ext",
				[]string{"heartbeat.source"},
				[]string{"heartbeat/source_poll"},
				nil,
				process,
			)
			m.extensions["ext"].initialize.HeartbeatSourceKinds = []extensionprotocol.HeartbeatSourceKind{
				{
					Kind:  "issues",
					Title: "Issues",
					ParamsSchema: json.RawMessage(
						`{"type":"object","required":["repo"],"properties":{"repo":{"type":"string"}}}`,
					),
				},
			}
			var poller source.ExtensionPoller = m
			kinds, err := poller.SourceKinds(t.Context(), source.CatalogScope{})
			if err != nil || len(kinds) != 1 || kinds[0].Kind != "ext/issues" {
				t.Fatalf("kinds=%+v err=%v", kinds, err)
			}
			result, err := poller.PollSource(
				t.Context(),
				source.CatalogScope{},
				tc.kind,
				source.Request{Params: []byte(tc.params), Limits: source.Limits{MaxItems: 20, MaxStateBytes: 16384}},
			)
			if tc.code != "" {
				failure, ok := errors.AsType[*source.Error](err)
				if !ok || string(failure.Code) != tc.code {
					t.Fatalf("error=%v want %s", err, tc.code)
				}
				return
			}
			if err != nil || len(result.Items) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
		})
	}
	t.Run("Should distinguish equal local kinds and exclude disabled extensions", func(t *testing.T) {
		t.Parallel()
		m := newWatchSourceManagerForTest(
			"one",
			[]string{"heartbeat.source"},
			[]string{"heartbeat/source_poll"},
			nil,
			newFakeProcess(9204),
		)
		second := newWatchSourceManagerForTest(
			"two",
			[]string{"heartbeat.source"},
			[]string{"heartbeat/source_poll"},
			nil,
			newFakeProcess(9205),
		)
		m.extensions["two"] = second.extensions["two"]
		for _, ext := range m.extensions {
			ext.initialize.HeartbeatSourceKinds = []extensionprotocol.HeartbeatSourceKind{{Kind: "issues"}}
		}
		kinds, err := m.SourceKinds(t.Context(), source.CatalogScope{})
		if err != nil || len(kinds) != 2 || kinds[0].Kind != "one/issues" || kinds[1].Kind != "two/issues" {
			t.Fatalf("kinds=%v err=%v", kinds, err)
		}
		m.extensions["two"].active = false
		kinds, err = m.SourceKinds(t.Context(), source.CatalogScope{})
		if err != nil || len(kinds) != 1 {
			t.Fatalf("kinds=%v err=%v", kinds, err)
		}
		_, err = m.PollSource(t.Context(), source.CatalogScope{}, "two/issues", source.Request{})
		if err == nil || !strings.Contains(err.Error(), "source_kind_unavailable") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("Should resolve the profile-global instance in User scope", func(t *testing.T) {
		t.Parallel()
		m := newWatchSourceManagerForTest(
			"ext",
			[]string{"heartbeat.source"},
			[]string{"heartbeat/source_poll"},
			nil,
			newFakeProcess(9203),
		)
		key := ProfileInstanceKey("ext", "profile-a", "")
		ext := m.extensions["ext"]
		ext.key = key
		ext.initialize.HeartbeatSourceKinds = []extensionprotocol.HeartbeatSourceKind{{Kind: "issues"}}
		m.scopedExtensions = map[InstanceKey]*managedExtension{key: ext}
		m.profileCoordinators = map[InstanceKey]*sync.Mutex{}
		m.profileNames = fixedProfileNameResolver{"profile-a": "profile-a"}
		kinds, err := m.SourceKinds(t.Context(), source.CatalogScope{ProfileID: "profile-a"})
		if err != nil || len(kinds) != 1 || kinds[0].Kind != "ext/issues" {
			t.Fatalf("kinds=%v err=%v", kinds, err)
		}
	})

	t.Run("Should keep workspace kinds out of User scope", func(t *testing.T) {
		t.Parallel()
		key := InstanceKey{Name: "ext", WorkspaceID: "ws-a"}
		m := newWatchSourceManagerForTest(
			"ext",
			[]string{"heartbeat.source"},
			[]string{"heartbeat/source_poll"},
			nil,
			newFakeProcess(9202),
		)
		ext := m.extensions["ext"]
		ext.key = key
		ext.initialize.HeartbeatSourceKinds = []extensionprotocol.HeartbeatSourceKind{{Kind: "issues"}}
		delete(m.extensions, "ext")
		m.devExtensions = map[InstanceKey]*managedExtension{key: ext}
		m.devCoordinators = map[InstanceKey]*sync.Mutex{}
		kinds, err := m.SourceKinds(t.Context(), source.CatalogScope{WorkspaceID: "ws-a"})
		if err != nil || len(kinds) != 1 {
			t.Fatalf("kinds=%v err=%v", kinds, err)
		}
		for _, scope := range []source.CatalogScope{{}, {WorkspaceID: "ws-b"}} {
			kinds, err = m.SourceKinds(t.Context(), scope)
			if err != nil || len(kinds) != 0 {
				t.Fatalf("leaked kinds=%v err=%v", kinds, err)
			}
		}
	})
}
