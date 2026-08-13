package github

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// Canonical suite: GitHub forge credential, HTTP, and pull-request behavior.
func TestCredentialResolverIT036(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer the bound token without invoking gh", func(t *testing.T) {
		t.Parallel()
		ghCalls := 0
		resolver := credentialResolver{
			getenv: func(string) string { return " bound-token " },
			lookPath: func(string) (string, error) {
				t.Fatal("lookPath must not run when the binding is present")
				return "", nil
			},
			runGH: func(context.Context, string) ([]byte, error) {
				ghCalls++
				return nil, nil
			},
		}
		got, err := resolver.resolve(context.Background())
		if err != nil || got != (credential{token: "bound-token", source: "binding"}) || ghCalls != 0 {
			t.Fatalf("resolve() = %#v, %v, gh calls %d", got, err, ghCalls)
		}
	})

	t.Run("Should fall back to the native gh login", func(t *testing.T) {
		t.Parallel()
		resolver := credentialResolver{
			getenv:   func(string) string { return "" },
			lookPath: func(string) (string, error) { return "/usr/bin/gh", nil },
			runGH:    func(context.Context, string) ([]byte, error) { return []byte(" gh-token\n"), nil },
		}
		got, err := resolver.resolve(context.Background())
		if err != nil || got != (credential{token: "gh-token", source: "gh"}) {
			t.Fatalf("resolve() = %#v, %v", got, err)
		}
	})

	t.Run("Should report an absent credential when neither source is available", func(t *testing.T) {
		t.Parallel()
		resolver := credentialResolver{
			getenv:   func(string) string { return "" },
			lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			runGH:    func(context.Context, string) ([]byte, error) { return nil, errors.New("unexpected gh call") },
		}
		got, err := resolver.resolve(context.Background())
		if err != nil || got != (credential{}) {
			t.Fatalf("resolve() = %#v, %v", got, err)
		}
	})

	t.Run("Should preserve cancellation from native gh lookup", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		resolver := credentialResolver{
			getenv:   func(string) string { return "" },
			lookPath: func(string) (string, error) { return "/usr/bin/gh", nil },
			runGH: func(ctx context.Context, _ string) ([]byte, error) {
				return nil, ctx.Err()
			},
		}
		if _, err := resolver.resolve(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("resolve(canceled) error = %v, want context.Canceled", err)
		}
	})
}

func TestRunProviderForgeProtocolConformanceIT035(t *testing.T) {
	t.Parallel()

	runtime := newForgeRuntimeHarness(t)
	initialize := runtime.call(t, 1, "initialize", forgeInitializeParams())
	if initialize.Error != nil {
		t.Fatalf("initialize RPC error = %#v", initialize.Error)
	}
	var initializeResult struct {
		ImplementedMethods []string `json:"implemented_methods"`
	}
	if err := json.Unmarshal(initialize.Result, &initializeResult); err != nil {
		t.Fatalf("decode initialize result: %v; result=%s", err, initialize.Result)
	}
	for _, method := range []string{"forge/capabilities", "forge/status", "forge/pr_create"} {
		if !slices.Contains(initializeResult.ImplementedMethods, method) {
			t.Fatalf("implemented methods = %#v, want %q", initializeResult.ImplementedMethods, method)
		}
	}

	for _, test := range []struct {
		name   string
		method string
		params any
	}{
		{
			name: "capabilities", method: "forge/capabilities",
			params: extensioncontract.ForgeCapabilitiesRequest{RemoteURLs: []string{"https://gitlab.com/acme/repo.git"}},
		},
		{
			name: "status", method: "forge/status",
			params: extensioncontract.ForgeStatusRequest{
				RemoteURLs: []string{"https://gitlab.com/acme/repo.git"}, Branch: "feature/exit",
			},
		},
		{
			name: "pull request create", method: "forge/pr_create",
			params: extensioncontract.ForgePRCreateRequest{
				RemoteURLs: []string{"https://gitlab.com/acme/repo.git"}, Head: "feature/exit", Base: "main",
			},
		},
	} {
		t.Run("Should serve "+test.name+" through the public SDK", func(t *testing.T) {
			result := runtime.call(t, 2, test.method, test.params)
			if result.Error != nil {
				t.Fatalf("%s RPC error = %#v", test.method, result.Error)
			}
			var response struct {
				Cause string `json:"cause"`
			}
			if err := json.Unmarshal(result.Result, &response); err != nil {
				t.Fatalf("decode %s result: %v; result=%s", test.method, err, result.Result)
			}
			if response.Cause != extensioncontract.ForgeCauseUnsupportedRemote {
				t.Fatalf(
					"%s cause = %q, want %q",
					test.method,
					response.Cause,
					extensioncontract.ForgeCauseUnsupportedRemote,
				)
			}
		})
	}
}

type forgeRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type forgeRuntimeHarness struct {
	extensionInput   *io.PipeReader
	daemonWriter     *io.PipeWriter
	daemonReaderPipe *io.PipeReader
	daemonReader     *bufio.Reader
	extensionOutput  *io.PipeWriter
	cancel           context.CancelFunc
	done             chan error
	stderr           *bytes.Buffer
}

func newForgeRuntimeHarness(t testing.TB) *forgeRuntimeHarness {
	t.Helper()
	extensionInput, daemonWriter := io.Pipe()
	daemonReaderPipe, extensionOutput := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	harness := &forgeRuntimeHarness{
		extensionInput:   extensionInput,
		daemonWriter:     daemonWriter,
		daemonReaderPipe: daemonReaderPipe,
		daemonReader:     bufio.NewReader(daemonReaderPipe),
		extensionOutput:  extensionOutput,
		cancel:           cancel,
		done:             make(chan error, 1),
		stderr:           &bytes.Buffer{},
	}
	go func() {
		harness.done <- RunProvider(ctx, extensionInput, extensionOutput, harness.stderr)
	}()
	t.Cleanup(func() {
		var cleanupErr error
		cleanupErr = errors.Join(cleanupErr, daemonWriter.Close())
		select {
		case runErr := <-harness.done:
			cleanupErr = errors.Join(cleanupErr, runErr)
		case <-time.After(2 * time.Second):
			cancel()
			select {
			case runErr := <-harness.done:
				cleanupErr = errors.Join(cleanupErr, runErr)
			case <-time.After(2 * time.Second):
				cleanupErr = errors.Join(cleanupErr, errors.New("github forge runtime did not stop"))
			}
		}
		cancel()
		cleanupErr = errors.Join(
			cleanupErr,
			extensionInput.Close(),
			extensionOutput.Close(),
			daemonReaderPipe.Close(),
		)
		if cleanupErr != nil {
			t.Errorf("close forge runtime: %v; stderr=%s", cleanupErr, harness.stderr.String())
		}
	})
	return harness
}

func (h *forgeRuntimeHarness) call(t testing.TB, id int, method string, params any) forgeRPCResponse {
	t.Helper()
	request, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		t.Fatalf("marshal %s request: %v", method, err)
	}
	if _, err := h.daemonWriter.Write(append(request, '\n')); err != nil {
		t.Fatalf("write %s request: %v", method, err)
	}
	line, err := h.daemonReader.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read %s response: %v; stderr=%s", method, err, h.stderr.String())
	}
	var response forgeRPCResponse
	if err := json.Unmarshal(bytes.TrimSpace(line), &response); err != nil {
		t.Fatalf("decode %s RPC response: %v; response=%s stderr=%s", method, err, line, h.stderr.String())
	}
	if response.ID != id {
		t.Fatalf("%s response id = %d, want %d", method, response.ID, id)
	}
	return response
}

func forgeInitializeParams() map[string]any {
	return map[string]any{
		"protocol_version":            "1",
		"supported_protocol_versions": []string{"1"},
		"compozy_version":             "test",
		"session_nonce":               "nonce",
		"extension": map[string]any{
			"name": Name, "version": "0.1.0", "source_tier": "bundled",
		},
		"capabilities": map[string]any{
			"provides":                []string{"forge.provider"},
			"granted_permissions":     []string{},
			"granted_resource_kinds":  []string{},
			"granted_resource_scopes": []string{},
		},
		"methods": map[string]any{
			"daemon_requests":    []string{"health_check", "shutdown"},
			"extension_services": []string{"forge/capabilities", "forge/status", "forge/pr_create"},
		},
		"runtime": map[string]any{
			"health_check_interval_ms": 30000,
			"health_check_timeout_ms":  5000,
			"shutdown_timeout_ms":      10000,
			"default_hook_timeout_ms":  5000,
		},
	}
}

func TestProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should expose authenticated GitHub capabilities and the default branch", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.Method != http.MethodGet || r.URL.Path != "/repos/acme/repo" ||
				r.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("request = %s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
			}
			writeJSON(t, w, http.StatusOK, repositoryPayload{DefaultBranch: "trunk"})
		})
		defer closeServer()

		got, err := provider.Capabilities(context.Background(), extensioncontract.ForgeCapabilitiesRequest{
			RemoteURLs: []string{"git@github.com:acme/repo.git"},
		})
		if err != nil || !got.Served || !got.Available || got.Provider != "github" ||
			got.CredentialSource != "binding" || got.DefaultBranch != "trunk" || calls.Load() != 1 {
			t.Fatalf("Capabilities() = %#v, %v; calls=%d", got, err, calls.Load())
		}
	})

	for _, testCase := range []struct {
		name       string
		statusCode int
		cause      string
	}{
		{name: "expired credentials", statusCode: http.StatusUnauthorized, cause: extensioncontract.ForgeCauseCredentialExpired},
		{name: "rate limiting", statusCode: http.StatusForbidden, cause: extensioncontract.ForgeCauseRateLimited},
	} {
		t.Run("Should classify "+testCase.name+" without retrying", func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				writeJSON(t, w, testCase.statusCode, map[string]string{"message": testCase.name})
			})
			defer closeServer()
			got, err := provider.Capabilities(context.Background(), extensioncontract.ForgeCapabilitiesRequest{
				RemoteURLs: []string{"https://github.com/acme/repo.git"},
			})
			if err != nil || got.Cause != testCase.cause || calls.Load() != 1 {
				t.Fatalf("Capabilities() = %#v, %v; calls=%d", got, err, calls.Load())
			}
		})
	}

	t.Run("Should return an existing open pull request without creating another", func(t *testing.T) {
		t.Parallel()
		var getCalls, postCalls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				postCalls.Add(1)
				t.Fatal("CreatePR must not post when an open pull request already exists")
			}
			getCalls.Add(1)
			if got := r.URL.Query().Get("head"); got != "acme:feature/exit" {
				t.Fatalf("head query = %q", got)
			}
			writeJSON(
				t,
				w,
				http.StatusOK,
				[]pullPayload{{Number: 41, HTMLURL: "https://github.com/acme/repo/pull/41", State: "open"}},
			)
		})
		defer closeServer()
		got, err := provider.CreatePR(context.Background(), createRequest())
		if err != nil || got.Status != "opened_existing" || got.Number != 41 || getCalls.Load() != 1 ||
			postCalls.Load() != 0 {
			t.Fatalf("CreatePR() = %#v, %v; GET=%d POST=%d", got, err, getCalls.Load(), postCalls.Load())
		}
	})

	t.Run("Should create one pull request after confirming none exists", func(t *testing.T) {
		t.Parallel()
		var getCalls, postCalls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				getCalls.Add(1)
				writeJSON(t, w, http.StatusOK, []pullPayload{})
			case http.MethodPost:
				postCalls.Add(1)
				var body extensioncontract.ForgePRCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("decode create body: %v", err)
				}
				if body.Head != "feature/exit" || body.Base != "main" || body.Title != "Exit worktree" || !body.Draft {
					t.Fatalf("create body = %#v", body)
				}
				writeJSON(
					t,
					w,
					http.StatusCreated,
					pullPayload{Number: 42, HTMLURL: "https://github.com/acme/repo/pull/42", State: "open"},
				)
			default:
				t.Fatalf("unexpected method %s", r.Method)
			}
		})
		defer closeServer()
		got, err := provider.CreatePR(context.Background(), createRequest())
		if err != nil || got.Status != "created" || got.Number != 42 || getCalls.Load() != 1 || postCalls.Load() != 1 {
			t.Fatalf("CreatePR() = %#v, %v; GET=%d POST=%d", got, err, getCalls.Load(), postCalls.Load())
		}
	})

	t.Run("Should reject an empty successful pull request response", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				writeJSON(t, w, http.StatusOK, []pullPayload{})
				return
			}
			w.WriteHeader(http.StatusCreated)
		})
		defer closeServer()
		if got, err := provider.CreatePR(context.Background(), createRequest()); err == nil {
			t.Fatalf("CreatePR(empty success) = %#v, nil error", got)
		}
	})

	t.Run("Should report merged pull request status", func(t *testing.T) {
		t.Parallel()
		mergedAt := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, []pullPayload{{
				Number: 43, HTMLURL: "https://github.com/acme/repo/pull/43", State: "closed", MergedAt: &mergedAt,
			}})
		})
		defer closeServer()
		got, err := provider.Status(context.Background(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
		})
		if err != nil || got.PRState == nil || *got.PRState != "merged" || got.Merged == nil || !*got.Merged {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})
}

func TestParseRepository(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name string
		url  string
		ok   bool
	}{
		{name: "HTTPS", url: "https://github.com/acme/repo.git", ok: true},
		{name: "SSH", url: "git@github.com:acme/repo.git", ok: true},
		{name: "uppercase HTTPS host", url: "https://GITHUB.COM/acme/repo.git", ok: true},
		{name: "uppercase SSH host", url: "git@GITHUB.COM:acme/repo.git", ok: true},
		{name: "SSH URL", url: "ssh://git@github.com/acme/repo.git", ok: true},
		{name: "file URL", url: "file://github.com/acme/repo.git", ok: false},
		{name: "dot owner", url: "https://github.com/../repo.git", ok: false},
		{name: "encoded path", url: "https://github.com/acme%2Frepo.git", ok: false},
		{name: "enterprise host", url: "https://github.mycompany.com/acme/repo.git", ok: false},
		{name: "non GitHub remote", url: "https://gitlab.com/acme/repo.git", ok: false},
	} {
		t.Run("Should classify "+testCase.name, func(t *testing.T) {
			t.Parallel()
			repository, ok := parseRepository(testCase.url)
			if ok != testCase.ok {
				t.Fatalf("parseRepository(%q) = %#v, %t", testCase.url, repository, ok)
			}
			if ok && (repository.owner != "acme" || repository.name != "repo" ||
				repository.url != "https://github.com/acme/repo") {
				t.Fatalf("parseRepository(%q) = %#v", testCase.url, repository)
			}
		})
	}
}

func testProvider(t *testing.T, handler http.HandlerFunc) (*Provider, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	provider := &Provider{
		credentials: credentialResolver{
			getenv:   func(string) string { return "secret" },
			lookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			runGH:    func(context.Context, string) ([]byte, error) { return nil, errors.New("unexpected gh call") },
		},
		api: githubClient{baseURL: server.URL, client: server.Client()},
		now: func() time.Time { return time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC) },
	}
	return provider, server.Close
}

func createRequest() extensioncontract.ForgePRCreateRequest {
	return extensioncontract.ForgePRCreateRequest{
		RemoteURLs: []string{"https://github.com/acme/repo.git"}, Head: "feature/exit", Base: "main",
		Title: "Exit worktree", Body: "Ready", Draft: true,
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !strings.Contains(err.Error(), "closed") {
		t.Errorf("encode response: %v", err)
	}
}
