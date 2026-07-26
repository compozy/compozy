package daytona

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	daytonaerrors "github.com/daytonaio/daytona/libs/sdk-go/pkg/errors"
)

func TestSDKClientAdapter(t *testing.T) {
	t.Run("Should use the Daytona SDK API", func(t *testing.T) {
		// not parallel: Daytona SDK client construction reads process environment.
		t.Setenv("DAYTONA_API_KEY", "test-key")

		var serverURL string
		var seenCreate bool
		var seenUpload bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodPost && r.URL.Path == "/sandbox":
				seenCreate = true
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode create body: %v", err)
				}
				if body["snapshot"] != "snap-sdk" {
					t.Errorf("create snapshot = %#v, want snap-sdk", body["snapshot"])
				}
				writeJSON(t, w, sandboxResponse(serverURL, "sandbox-sdk"))
			case r.Method == http.MethodGet && r.URL.Path == "/sandbox/sandbox-sdk":
				writeJSON(t, w, sandboxResponse(serverURL, "sandbox-sdk"))
			case r.Method == http.MethodGet && r.URL.Path == "/sandbox":
				if got, want := r.URL.Query().Get("limit"), "1"; got != want {
					t.Errorf("list limit = %q, want %q", got, want)
				}
				if labels := r.URL.Query().Get("labels"); !strings.Contains(labels, `"agh_sandbox_id":"env-sdk"`) {
					t.Errorf("list labels = %q, want agh_sandbox_id label", labels)
				}
				writeJSON(t, w, map[string]any{
					"items":      []map[string]any{sandboxResponse(serverURL, "sandbox-sdk")},
					"nextCursor": nil,
				})
			case r.Method == http.MethodPost && r.URL.Path == "/sandbox/sandbox-sdk/start":
				writeJSON(t, w, sandboxResponse(serverURL, "sandbox-sdk"))
			case r.Method == http.MethodPost && r.URL.Path == "/sandbox/sandbox-sdk/archive":
				writeJSON(t, w, sandboxResponse(serverURL, "sandbox-sdk"))
			case r.Method == http.MethodDelete && r.URL.Path == "/sandbox/sandbox-sdk":
				writeJSON(t, w, sandboxResponse(serverURL, "sandbox-sdk"))
			case r.Method == http.MethodGet && r.URL.Path == "/toolbox/sandbox-sdk/work-dir":
				writeJSON(t, w, map[string]any{"dir": "/workdir"})
			case r.Method == http.MethodGet && r.URL.Path == "/toolbox/sandbox-sdk/files/download":
				if got, want := r.URL.Query().Get("path"), "/workdir/file.txt"; got != want {
					t.Errorf("download path = %q, want %q", got, want)
				}
				if _, err := w.Write([]byte("downloaded")); err != nil {
					t.Errorf("write download response: %v", err)
				}
			case r.Method == http.MethodPost && r.URL.Path == "/toolbox/sandbox-sdk/files/upload":
				seenUpload = true
				writeJSON(t, w, map[string]any{"file": map[string]any{"ok": true}})
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()
		serverURL = server.URL

		client, err := newSDKClient(clientConfig{APIURL: server.URL})
		if err != nil {
			t.Fatalf("newSDKClient() error = %v", err)
		}
		ctx := context.Background()
		created, err := client.Create(ctx, createSandboxRequest{
			Snapshot: "snap-sdk",
			Labels:   map[string]string{"agh_sandbox_id": "env-sdk"},
			EnvVars:  map[string]string{"AGH_SESSION_ID": "sess-sdk"},
			Timeout:  time.Second,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if !seenCreate {
			t.Fatal("server did not observe create request")
		}
		if got, want := created.ID(), "sandbox-sdk"; got != want {
			t.Fatalf("created.ID() = %q, want %q", got, want)
		}
		got, err := client.Get(ctx, "sandbox-sdk")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.Name() == "" {
			t.Fatal("Get().Name() = empty")
		}
		found, err := client.FindOne(ctx, map[string]string{"agh_sandbox_id": "env-sdk"})
		if err != nil {
			t.Fatalf("FindOne() error = %v", err)
		}
		if found.ID() != "sandbox-sdk" {
			t.Fatalf("FindOne().ID() = %q, want sandbox-sdk", found.ID())
		}
		if err := created.Start(ctx); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if dir, err := created.WorkingDir(ctx); err != nil || dir != "/workdir" {
			t.Fatalf("WorkingDir() = %q, %v; want /workdir nil", dir, err)
		}
		content, err := created.ReadFile(ctx, "/workdir/file.txt")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(content) != "downloaded" {
			t.Fatalf("ReadFile() = %q, want downloaded", string(content))
		}
		if err := created.WriteFile(ctx, "/workdir/file.txt", []byte("upload")); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if !seenUpload {
			t.Fatal("server did not observe upload request")
		}
		if err := created.Archive(ctx); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		if err := created.Delete(ctx); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
	})

	t.Run("Should map an empty Daytona list to not found", func(t *testing.T) {
		// not parallel: Daytona SDK client construction reads process environment.
		t.Setenv("DAYTONA_API_KEY", "test-key")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Path != "/sandbox" {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, map[string]any{"items": []map[string]any{}, "nextCursor": nil})
		}))
		defer server.Close()
		client, err := newSDKClient(clientConfig{APIURL: server.URL})
		if err != nil {
			t.Fatalf("newSDKClient() error = %v", err)
		}
		if _, err := client.FindOne(context.Background(), map[string]string{"missing": "true"}); err == nil {
			t.Fatal("FindOne() error = nil, want not found")
		}
	})
}

func TestMapSDKNotFound(t *testing.T) {
	t.Run("Should map Daytona not found errors to the sandbox sentinel", func(t *testing.T) {
		notFound := mapSDKNotFound("get sandbox", daytonaerrors.NewDaytonaNotFoundError("missing", nil))
		if !errors.Is(notFound, errSandboxNotFound) {
			t.Fatalf("mapSDKNotFound(not found) = %v, want errSandboxNotFound", notFound)
		}
	})

	t.Run("Should preserve non-not-found errors as regular SDK failures", func(t *testing.T) {
		other := mapSDKNotFound("get sandbox", errors.New("boom"))
		if errors.Is(other, errSandboxNotFound) {
			t.Fatalf("mapSDKNotFound(other) = %v, did not want errSandboxNotFound", other)
		}
	})
}

func sandboxResponse(serverURL string, id string) map[string]any {
	return map[string]any{
		"id":                 id,
		"organizationId":     "org",
		"name":               "sandbox-name",
		"user":               "daytona",
		"env":                map[string]string{},
		"labels":             map[string]string{"agh_sandbox_id": "env-sdk"},
		"public":             false,
		"networkBlockAll":    false,
		"target":             "default",
		"cpu":                1,
		"gpu":                0,
		"memory":             1,
		"disk":               1,
		"state":              "started",
		"toolboxProxyUrl":    strings.TrimRight(serverURL, "/") + "/toolbox",
		"autoStopInterval":   0,
		"autoDeleteInterval": -1,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}
