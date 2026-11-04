package github

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/heartbeat/source"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	compozysdk "github.com/compozy/compozy/sdk/go"
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
		got, err := resolver.resolve(t.Context())
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
		got, err := resolver.resolve(t.Context())
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
		got, err := resolver.resolve(t.Context())
		if err != nil || got != (credential{}) {
			t.Fatalf("resolve() = %#v, %v", got, err)
		}
	})

	t.Run("Should preserve cancellation from native gh lookup", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
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
		ImplementedMethods []string                         `json:"implemented_methods"`
		SourceKinds        []compozysdk.HeartbeatSourceKind `json:"heartbeat_source_kinds"`
	}
	if err := json.Unmarshal(initialize.Result, &initializeResult); err != nil {
		t.Fatalf("decode initialize result: %v; result=%s", err, initialize.Result)
	}
	for _, method := range []string{"forge/capabilities", "forge/status", "forge/pr_create", "heartbeat/source_poll"} {
		if !slices.Contains(initializeResult.ImplementedMethods, method) {
			t.Fatalf("implemented methods = %#v, want %q", initializeResult.ImplementedMethods, method)
		}
	}

	if len(initializeResult.SourceKinds) != 1 || initializeResult.SourceKinds[0].Kind != "issues" ||
		initializeResult.SourceKinds[0].Title != "New GitHub issues" {
		t.Fatalf("source descriptors = %+v", initializeResult.SourceKinds)
	}

	sourceResponse := runtime.call(t, 3, "heartbeat/source_poll", map[string]any{
		"kind": "issues", "params": map[string]string{"repo": "invalid"},
	})
	if sourceResponse.Error == nil || !strings.Contains(string(sourceResponse.Error.Data), "source_params_invalid") {
		t.Fatalf("source RPC response = %+v", sourceResponse)
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
				Title: "Exit feature worktree",
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
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
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
			"provides":                []string{"forge.provider", "heartbeat.source"},
			"granted_permissions":     []string{},
			"granted_resource_kinds":  []string{},
			"granted_resource_scopes": []string{},
		},
		"methods": map[string]any{
			"daemon_requests": []string{"health_check", "shutdown"},
			"extension_services": []string{
				"forge/capabilities",
				"forge/status",
				"forge/pr_create",
				"heartbeat/source_poll",
			},
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
	// Invariant: discovery scopes search to the repository and preserves PR number, title and branch.
	// Owner: forge provider I/O; canonical provider suite.
	t.Run("Should discover a bounded page and resolve PR identity", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/graphql":
				var body struct {
					Variables struct {
						Query string
						First int
						After string
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body.Variables.Query != `repo:acme/repo is:pr "review" sort:updated-desc` ||
					body.Variables.First != 1 ||
					body.Variables.After != "Y3Vyc29yOjE=" {
					t.Errorf("variables=%+v", body.Variables)
				}
				writeJSON(
					t,
					w,
					http.StatusOK,
					map[string]any{
						"data": map[string]any{
							"search": map[string]any{
								"nodes": []any{
									nil,
									map[string]any{
										"number":         42,
										"title":          "Linked work",
										"url":            "https://github.com/acme/repo/pull/42",
										"state":          "OPEN",
										"headRefName":    "topic",
										"headRepository": map[string]any{"nameWithOwner": "contributor/repo"},
									},
								},
								"pageInfo": map[string]any{"hasNextPage": true},
							},
						},
						"errors": []any{map[string]any{"type": "NOT_FOUND"}},
					},
				)
			default:
				t.Errorf("unexpected path=%s", r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}
		})
		defer closeServer()
		result, err := provider.SearchPRs(
			t.Context(),
			extensioncontract.ForgePRSearchRequest{
				RemoteURLs: []string{"https://github.com/acme/repo.git"},
				Query:      "review",
				Page:       2,
				Limit:      1,
			},
		)
		if err != nil || len(result.Items) != 1 || !result.HasMore {
			t.Fatalf("result=%+v error=%v", result, err)
		}
		item := result.Items[0]
		if item.Number != 42 || item.Title != "Linked work" || item.Branch != "topic" ||
			item.HeadRepository != "github.com/contributor/repo" ||
			item.URL != "https://github.com/acme/repo/pull/42" || item.State != "open" {
			t.Fatalf("candidate=%+v", item)
		}
	})

	// Invariant: a rate-limited discovery or exact delivery read reports the cause and reset the daemon
	// needs to hold every forge read, not a bare request error or a cause without its reset.
	t.Run("Should report discovery and delivery rate limits with their reset", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1800000000")
			writeJSON(t, w, http.StatusForbidden, map[string]string{"message": "API rate limit exceeded"})
		})
		defer closeServer()
		remotes := []string{"https://github.com/acme/repo.git"}
		_, searchErr := provider.SearchPRs(t.Context(), extensioncontract.ForgePRSearchRequest{RemoteURLs: remotes})
		_, readErr := provider.ReadPR(t.Context(), extensioncontract.ForgePRReadRequest{RemoteURLs: remotes, Number: 7})
		for _, err := range []error{searchErr, readErr} {
			rpc, ok := errors.AsType[*compozysdk.RPCError](err)
			if !ok || !strings.Contains(string(rpc.Data), `"code":"rate_limited"`) ||
				!strings.Contains(string(rpc.Data), `"retry_after":"2027-01-15T08:00:00Z"`) {
				t.Fatalf("discovery error = %#v", err)
			}
		}
		got, err := provider.Status(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: remotes, Branch: "topic", Base: "main", HeadSHA: "abc",
		})
		if err != nil || got.Cause != extensioncontract.ForgeCauseRateLimited || got.RetryAfter == nil ||
			got.RetryAfter.Unix() != 1800000000 {
			t.Fatalf("delivery status = %#v, %v", got, err)
		}
	})

	for _, scenario := range []string{"open first", "terminal newest", "incomplete", "bounded"} {
		t.Run("Should preserve REST display selection for "+scenario, func(t *testing.T) {
			t.Parallel()
			provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/graphql" {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if r.URL.Query().Get("sort") != "updated" || r.URL.Query().Get("direction") != "desc" {
					t.Errorf("selection query=%s", r.URL.RawQuery)
				}
				if r.URL.Query().Get("state") == "open" && scenario == "terminal newest" {
					writeJSON(t, w, http.StatusOK, []pullPayload{})
					return
				}
				state := r.URL.Query().Get("state")
				if scenario == "open first" && state != "open" {
					t.Error("terminal population requested despite matching open PR")
				}
				pulls := make([]pullPayload, 0, 3)
				for i, owner := range []string{"acme/repo", "fork/repo", "acme/repo"} {
					pull := pullPayload{
						Number:    i + 1,
						State:     state,
						HTMLURL:   fmt.Sprintf("https://github.com/acme/repo/pull/%d", i+1),
						UpdatedAt: time.Unix(int64(i+1), 0),
						Head:      pullRef{Ref: "topic", SHA: "abc"},
					}
					pull.Head.Repo = &struct {
						FullName string `json:"full_name"`
					}{owner}
					if owner == "fork/repo" {
						pull.UpdatedAt = time.Unix(100, 0)
					}
					pulls = append(pulls, pull)
				}
				if scenario == "incomplete" {
					pulls[0].Head.Repo = nil
				}
				if scenario == "bounded" {
					fork := pulls[1]
					pulls = make([]pullPayload, 100)
					for i := range pulls {
						pulls[i] = fork
					}
				}
				writeJSON(t, w, http.StatusOK, pulls)
			})
			defer closeServer()
			got, err := provider.Status(
				t.Context(),
				extensioncontract.ForgeStatusRequest{
					RemoteURLs: []string{"https://github.com/acme/repo"},
					Branch:     "topic",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "incomplete" || scenario == "bounded" {
				if got.Selection != "incomplete" || got.PRNumber != nil {
					t.Fatalf("incomplete=%#v", got)
				}
				return
			}
			if got.PRNumber == nil || *got.PRNumber != 3 || got.Selection != "complete" {
				t.Fatalf("selection=%#v", got)
			}
		})
	}

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

		got, err := provider.Capabilities(t.Context(), extensioncontract.ForgeCapabilitiesRequest{
			RemoteURLs: []string{"git@github.com:acme/repo.git"},
		})
		if err != nil || !got.Served || !got.Available || got.Provider != "github" ||
			got.CredentialSource != "binding" || got.DefaultBranch != "trunk" || calls.Load() != 1 {
			t.Fatalf("Capabilities() = %#v, %v; calls=%d", got, err, calls.Load())
		}
	})

	// Invariant: only a refusal GitHub marks as a rate limit is classified as one, and its reset
	// travels with the capability answer so the daemon can hold every read until then.
	for _, testCase := range []struct {
		name       string
		statusCode int
		header     map[string]string
		message    string
		cause      string
		retryAfter int64
	}{
		{name: "expired credentials", statusCode: http.StatusUnauthorized, cause: extensioncontract.ForgeCauseCredentialExpired},
		{
			name: "an exhausted primary rate limit", statusCode: http.StatusForbidden,
			header: map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": "1800000000"},
			cause:  extensioncontract.ForgeCauseRateLimited, retryAfter: 1800000000,
		},
		{
			name: "a secondary rate limit message", statusCode: http.StatusForbidden,
			message: "You have exceeded a secondary rate limit", cause: extensioncontract.ForgeCauseRateLimited,
		},
		{
			name: "a secondary rate limit with Retry-After", statusCode: http.StatusTooManyRequests,
			header: map[string]string{"Retry-After": "Fri, 15 Jan 2027 08:00:00 GMT", "X-RateLimit-Reset": "1"},
			cause:  extensioncontract.ForgeCauseRateLimited, retryAfter: 1800000000,
		},
	} {
		t.Run("Should classify "+testCase.name+" without retrying", func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				for key, value := range testCase.header {
					w.Header().Set(key, value)
				}
				writeJSON(t, w, testCase.statusCode, map[string]string{"message": testCase.message})
			})
			defer closeServer()
			got, err := provider.Capabilities(t.Context(), extensioncontract.ForgeCapabilitiesRequest{
				RemoteURLs: []string{"https://github.com/acme/repo.git"},
			})
			if err != nil || got.Cause != testCase.cause || calls.Load() != 1 {
				t.Fatalf("Capabilities() = %#v, %v; calls=%d", got, err, calls.Load())
			}
			if (testCase.retryAfter == 0) != (got.RetryAfter == nil) ||
				(got.RetryAfter != nil && got.RetryAfter.Unix() != testCase.retryAfter) {
				t.Fatalf("RetryAfter = %v, want unix %d", got.RetryAfter, testCase.retryAfter)
			}
		})
	}
	t.Run("Should not mistake a permission refusal for a rate limit", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "4000")
			writeJSON(
				t,
				w,
				http.StatusForbidden,
				map[string]string{"message": "Resource not accessible by integration"},
			)
		})
		defer closeServer()
		got, err := provider.Capabilities(t.Context(), extensioncontract.ForgeCapabilitiesRequest{
			RemoteURLs: []string{"https://github.com/acme/repo.git"},
		})
		if err == nil || got.Cause != "" {
			t.Fatalf("Capabilities() = %#v, %v; want an unclassified failure", got, err)
		}
	})
	t.Run("Should choose the reset that GitHub's headers imply", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, time.November, 4, 15, 0, 0, 0, time.UTC)
		primary := strconv.FormatInt(now.Add(30*time.Minute).Unix(), 10)
		for _, test := range []struct {
			name   string
			header http.Header
			want   *time.Time
		}{
			{"Retry-After seconds", http.Header{"Retry-After": {"90"}}, new(now.Add(90 * time.Second))},
			{
				"a later primary reset over a shorter Retry-After",
				http.Header{"Retry-After": {"90"}, "X-Ratelimit-Remaining": {"0"}, "X-Ratelimit-Reset": {primary}},
				new(now.Add(30 * time.Minute)),
			},
			{
				"the one-minute fallback for a secondary limit with quota left",
				http.Header{"X-Ratelimit-Remaining": {"12"}, "X-Ratelimit-Reset": {primary}},
				nil,
			},
		} {
			failure := classifyGitHubResponse(
				http.StatusForbidden, test.header, []byte("secondary rate limit"), now,
			).(*githubAPIError)
			if failure.cause != extensioncontract.ForgeCauseRateLimited ||
				(test.want == nil) != (failure.retryAfter == nil) ||
				(test.want != nil && !failure.retryAfter.Equal(*test.want)) {
				t.Fatalf("%s: classified = %#v", test.name, failure)
			}
		}
	})

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
		got, err := provider.CreatePR(t.Context(), createRequest())
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
		got, err := provider.CreatePR(t.Context(), createRequest())
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
		if got, err := provider.CreatePR(t.Context(), createRequest()); err == nil ||
			!strings.Contains(err.Error(), "invalid created pull request") {
			t.Fatalf("CreatePR(empty success) = %#v, %v; want invalid created pull request", got, err)
		}
	})

	t.Run("Should reconcile only the exact draft delivery candidate", func(t *testing.T) {
		t.Parallel()
		draft := true
		candidate := pullPayload{Number: 44, HTMLURL: "https://github.com/acme/repo/pull/44", State: "open",
			Head: pullRef{Ref: "feature/exit", SHA: "reviewed-sha"}, Base: pullRef{Ref: "main"}, Draft: &draft}
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Query().Get("state") != "open" {
				t.Errorf("unexpected candidate lookup %s %s", r.Method, r.URL)
			}
			writeJSON(t, w, http.StatusOK, []pullPayload{candidate})
		})
		defer closeServer()
		status, err := provider.Status(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{
				"https://github.com/acme/repo",
			},
			Branch:  "feature/exit",
			Base:    "main",
			HeadSHA: "reviewed-sha",
		})
		if err != nil || status.Head != "feature/exit" || status.Base != "main" || status.HeadSHA != "reviewed-sha" ||
			status.Draft == nil || !*status.Draft {
			t.Fatalf("exact candidate status = %#v, %v", status, err)
		}
		request := createRequest()
		request.HeadSHA = "reviewed-sha"
		created, err := provider.CreatePR(t.Context(), request)
		if err != nil || created.Status != "opened_existing" || created.Number != candidate.Number {
			t.Fatalf("reconciled create = %#v, %v", created, err)
		}
	})

	for _, mismatch := range []string{"base", "head", "sha", "draft", "unknown draft", "ambiguous"} {
		t.Run("Should reject mismatched delivery "+mismatch+" before publishing", func(t *testing.T) {
			t.Parallel()
			draft := true
			candidate := pullPayload{Number: 44, HTMLURL: "https://github.com/acme/repo/pull/44", State: "open",
				Head: pullRef{Ref: "feature/exit", SHA: "reviewed-sha"}, Base: pullRef{Ref: "main"}, Draft: &draft}
			switch mismatch {
			case "base":
				candidate.Base.Ref = "other"
			case "head":
				candidate.Head.Ref = "other"
			case "sha":
				candidate.Head.SHA = "changed-sha"
			case "draft":
				draft = false
			case "unknown draft":
				candidate.Draft = nil
			}
			var posts atomic.Int32
			provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts.Add(1)
				}
				pulls := []pullPayload{candidate}
				if mismatch == "ambiguous" {
					pulls = append(pulls, candidate)
				}
				writeJSON(t, w, http.StatusOK, pulls)
			})
			defer closeServer()
			request := createRequest()
			request.HeadSHA = "reviewed-sha"
			want := "does not match the delivery candidate"
			if mismatch == "ambiguous" {
				want = "multiple pull requests match"
			}
			if got, err := provider.CreatePR(t.Context(), request); err == nil ||
				!strings.Contains(err.Error(), want) || posts.Load() != 0 {
				t.Fatalf("mismatched CreatePR = %#v, %v, posts=%d; want %q", got, err, posts.Load(), want)
			}
		})
	}

	t.Run("Should find the exact candidate beyond the first lookup page", func(t *testing.T) {
		t.Parallel()
		draft := true
		var calls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Query().Get("page") == "1" {
				pulls := make([]pullPayload, 100)
				for i := range pulls {
					pulls[i].Head.Ref = "other"
				}
				writeJSON(t, w, http.StatusOK, pulls)
				return
			}
			writeJSON(t, w, http.StatusOK, []pullPayload{
				{
					Number:  45,
					HTMLURL: "https://github.com/acme/repo/pull/45",
					State:   "open",
					Head:    pullRef{Ref: "feature/exit", SHA: "reviewed-sha"},
					Base:    pullRef{Ref: "main"},
					Draft:   &draft,
				},
			})
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{
				"https://github.com/acme/repo",
			},
			Branch:  "feature/exit",
			Base:    "main",
			HeadSHA: "reviewed-sha",
		})
		if err != nil || got.PRNumber == nil || *got.PRNumber != 45 || calls.Load() != 2 {
			t.Fatalf("paged exact status = %#v, %v, calls=%d", got, err, calls.Load())
		}
	})

	t.Run("Should report merged pull request status", func(t *testing.T) {
		t.Parallel()
		mergedAt := time.Date(2026, time.August, 12, 10, 0, 0, 0, time.UTC)
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/graphql" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.URL.Query().Get("state") == "open" {
				writeJSON(t, w, http.StatusOK, []pullPayload{})
				return
			}
			writeJSON(t, w, http.StatusOK, []pullPayload{{
				UpdatedAt: mergedAt, Head: pullRef{Ref: "feature/exit", Repo: &struct {
					FullName string `json:"full_name"`
				}{"acme/repo"}},
				Number: 43, HTMLURL: "https://github.com/acme/repo/pull/43", State: "closed", MergedAt: &mergedAt,
			}})
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
		})
		if err != nil || got.PRState == nil || *got.PRState != "merged" || got.Merged == nil || !*got.Merged {
			t.Fatalf("Status() = %#v, %v", got, err)
		}
	})
}

const emptyGraphChecks = `{"data":{"repository":{"object":{"oid":"head-a"}}}}`

const graphOpenPull = `{"data":{"repository":{"pullRequests":{"nodes":[{"number":2,"url":"https://github.com/acme/repo/pull/2","state":"OPEN","headRefOid":"head-a","headRepository":{"nameWithOwner":"acme/repo"}}],"pageInfo":{"hasNextPage":false}}}}}`

func graphTopicRequest() extensioncontract.ForgeStatusRequest {
	return extensioncontract.ForgeStatusRequest{RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "topic"}
}

// graphServer answers the open pull request selection itself and every other query through respond.
func graphServer(t *testing.T, respond func(string, map[string]any) string) (*Provider, func()) {
	t.Helper()
	return testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Query     string
			Variables map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if strings.Contains(input.Query, "$branch") {
			writeGraphBody(t, w, graphOpenPull)
			return
		}
		body := respond(input.Query, input.Variables)
		if status, ok := strings.CutPrefix(body, "status:"); ok {
			code, err := strconv.Atoi(status)
			if err != nil {
				t.Error(err)
			}
			w.WriteHeader(code)
			return
		}
		writeGraphBody(t, w, body)
	})
}

func writeGraphBody(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}

// Invariant: canonical selection uses origin identity and immutable-head aggregate facts.
// Owner: GitHub provider; canonical httptest provider suite.
func TestProviderGraphQL(t *testing.T) {
	t.Parallel()
	t.Run("Should skip forks and read complete checks for the selected immutable head", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var input struct {
				Query     string
				Variables map[string]any
			}
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			calls.Add(1)
			if strings.Contains(input.Query, "$oid") {
				if input.Variables["oid"] != "head-a" {
					t.Errorf("checks oid=%v", input.Variables["oid"])
				}
				_, _ = w.Write(
					[]byte(
						`{"data":{"repository":{"object":{"oid":"head-a","statusCheckRollup":{"contexts":{"totalCount":102,"checkRunCountsByState":[{"state":"SUCCESS","count":100},{"state":"FAILURE","count":1}],"statusContextCountsByState":[{"state":"PENDING","count":1}],"nodes":[{"name":"lint","conclusion":"FAILURE"}]}}}}}}`,
					),
				)
				return
			}
			_, _ = w.Write(
				[]byte(
					`{"data":{"repository":{"pullRequests":{"nodes":[{"number":1,"headRepository":{"nameWithOwner":"fork/repo"}},{"number":2,"url":"https://github.com/acme/repo/pull/2","state":"OPEN","headRefOid":"head-a","headRepository":{"nameWithOwner":"acme/repo"},"reviewDecision":"APPROVED","mergeable":"MERGEABLE"}],"pageInfo":{"hasNextPage":false}}}}}`,
				),
			)
		})
		defer closeServer()
		got, err := provider.Status(
			t.Context(),
			extensioncontract.ForgeStatusRequest{RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "topic"},
		)
		if err != nil || got.PRNumber == nil || *got.PRNumber != 2 || got.HeadSHA != "head-a" || got.Checks == nil ||
			got.Checks.State != "failing" ||
			got.Checks.Passed != 100 ||
			got.Checks.Failed != 1 ||
			got.Checks.Pending != 1 ||
			got.ReviewDecision != "approved" ||
			calls.Load() != 3 {
			t.Fatalf("observation=%#v %v calls=%d", got, err, calls.Load())
		}
	})
	t.Run("Should page contexts until every failure on a long PR is named", func(t *testing.T) {
		t.Parallel()
		var checkCalls atomic.Int32
		provider, closeServer := graphServer(t, func(query string, variables map[string]any) string {
			if !strings.Contains(query, "$oid") {
				return `{"data":{"repository":{"pullRequest":{}}}}`
			}
			checkCalls.Add(1)
			page, next := `[{"name":"early","conclusion":"FAILURE"}]`, `true,"endCursor":"more"`
			if variables["after"] != nil {
				page, next = `[{"name":"late","conclusion":"TIMED_OUT"},{"context":"status","state":"FAILURE"}]`, `false`
			}
			return `{"data":{"repository":{"object":{"oid":"head-a","statusCheckRollup":{"contexts":{"totalCount":249,` +
				`"checkRunCountsByState":[{"state":"SUCCESS","count":246},{"state":"FAILURE","count":2}],` +
				`"statusContextCountsByState":[{"state":"FAILURE","count":1}],"nodes":` + page +
				`,"pageInfo":{"hasNextPage":` + next + `}}}}}}}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Checks == nil ||
			!slices.Equal(got.Checks.FailedNames, []string{"early", "late", "status"}) ||
			checkCalls.Load() != 2 {
			t.Fatalf("checks = %+v err = %v calls = %d", got.Checks, err, checkCalls.Load())
		}
	})
	t.Run("Should stop paging once its failures are named", func(t *testing.T) {
		t.Parallel()
		var checkCalls atomic.Int32
		provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
			if !strings.Contains(query, "$oid") {
				return `{"data":{"repository":{"pullRequest":{}}}}`
			}
			checkCalls.Add(1)
			return `{"data":{"repository":{"object":{"oid":"head-a","statusCheckRollup":{"contexts":{"totalCount":201,` +
				`"checkRunCountsByState":[{"state":"SUCCESS","count":200},{"state":"FAILURE","count":1}],` +
				`"statusContextCountsByState":[],"nodes":[{"name":"lint","conclusion":"FAILURE"}],` +
				`"pageInfo":{"hasNextPage":true,"endCursor":"more"}}}}}}}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Checks == nil || checkCalls.Load() != 1 {
			t.Fatalf("checks = %+v err = %v calls = %d", got.Checks, err, checkCalls.Load())
		}
	})
	t.Run("Should map submitted reviews and unresolved threads", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := graphServer(t, func(query string, variables map[string]any) string {
			if strings.Contains(query, "$oid") {
				return emptyGraphChecks
			}
			if variables["number"] != float64(2) {
				t.Errorf("number = %v", variables["number"])
			}
			threads := `{"nodes":[{"id":"t2","isResolved":false}],"pageInfo":{"hasPreviousPage":true,"startCursor":"p2"}}`
			if variables["before"] != nil {
				threads = `{"nodes":[{"id":"t0","isResolved":true},{"id":"t1","isResolved":false,"comments":{"nodes":[{"originalCommit":{"oid":"head-old"}}]}}],"pageInfo":{"hasPreviousPage":false}}`
			}
			return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[` +
				`{"id":"r1","state":"CHANGES_REQUESTED","submittedAt":"2026-11-02T17:09:56Z","author":{"__typename":"Bot","login":"reviewer[bot]"},"commit":{"oid":"head-a"}},` +
				`{"id":"r2","state":"COMMENTED","submittedAt":"2026-11-02T17:10:00Z","author":{"__typename":"User","login":"someone"},"commit":{"oid":"head-old"}},` +
				`{"id":"r3","state":"APPROVED","submittedAt":"2026-11-02T17:11:00Z","author":null,"commit":null},` +
				`{"id":"r4","state":"PENDING","author":{"__typename":"User","login":"viewer"}}` +
				`],"pageInfo":{"hasPreviousPage":false}},"reviewThreads":` + threads + `}}}}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Reviews == nil || got.Reviews.UnresolvedThreads != 2 || got.Reviews.Truncated ||
			len(got.Reviews.Items) != 3 {
			t.Fatalf("reviews = %+v err = %v", got.Reviews, err)
		}
		first := got.Reviews.Items[0]
		if first.ID != "r1" || first.State != extensioncontract.ForgeReviewChangesRequested || !first.IsBot ||
			first.Author != "reviewer[bot]" || first.Commit != "head-a" || first.SubmittedAt == nil {
			t.Fatalf("first review = %+v", first)
		}
		if ghost := got.Reviews.Items[2]; ghost.Author != "" || ghost.IsBot || ghost.Commit != "" {
			t.Fatalf("authorless review = %+v", ghost)
		}
		wantThreads := []extensioncontract.ForgeReviewThread{{ID: "t1", Commit: "head-old"}, {ID: "t2"}}
		if !slices.Equal(got.Reviews.Threads, wantThreads) {
			t.Fatalf("threads = %+v", got.Reviews.Threads)
		}
		sdk := forgeStatusSDKResponse(got)
		if sdk.Reviews == nil || sdk.Reviews.UnresolvedThreads != 2 || len(sdk.Reviews.Items) != 3 ||
			sdk.Reviews.Items[0].ID != "r1" || sdk.Reviews.Threads[0].Commit != "head-old" {
			t.Fatalf("SDK reviews = %+v", sdk.Reviews)
		}
	})
	t.Run("Should flag reviews it could not read whole", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
			if strings.Contains(query, "$oid") {
				return emptyGraphChecks
			}
			return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[],"pageInfo":{"hasPreviousPage":false}},` +
				`"reviewThreads":{"nodes":[{"id":"t1","isResolved":false}],"pageInfo":{"hasPreviousPage":true,"startCursor":"more"}}}}}}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Reviews == nil || !got.Reviews.Truncated ||
			got.Reviews.UnresolvedThreads != maxReviewThreadPages {
			t.Fatalf("reviews = %+v err = %v", got.Reviews, err)
		}
	})
	t.Run("Should flag threads it cannot page past", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
			if strings.Contains(query, "$oid") {
				return emptyGraphChecks
			}
			return `{"data":{"repository":{"pullRequest":{"reviews":{"nodes":[],"pageInfo":{"hasPreviousPage":false}},` +
				`"reviewThreads":{"nodes":[{"id":"t1","isResolved":false}],"pageInfo":{"hasPreviousPage":true,"startCursor":""}}}}}}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Reviews == nil || !got.Reviews.Truncated || got.Reviews.UnresolvedThreads != 1 {
			t.Fatalf("reviews = %+v err = %v", got.Reviews, err)
		}
	})
	t.Run("Should keep serving checks when review activity cannot be read", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
			if strings.Contains(query, "$oid") {
				return emptyGraphChecks
			}
			return `{"errors":[{"type":"FORBIDDEN"}]}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Cause != "" || got.Checks == nil || got.Reviews != nil {
			t.Fatalf("status = %+v err = %v", got, err)
		}
		if forgeStatusSDKResponse(got).Reviews != nil || got.ReviewsUnavailable {
			t.Fatalf("a refused query read as observable or as a failure: %+v", got)
		}
	})
	for _, testCase := range []struct {
		name, response string
		unavailable    bool
	}{
		{"Should mark reviews unavailable on a server error", "status:503", true},
		{"Should mark reviews unavailable on a transient GraphQL error", `{"errors":[{"type":"SERVICE_UNAVAILABLE"}]}`, true},
		{"Should mark reviews unavailable on an undecodable response", `not json`, true},
		{"Should mark reviews unavailable on an untyped execution error", `{"errors":[{"message":"Something went wrong"}]}`, true},
		{"Should report no capability when the endpoint is missing", "status:404", false},
		{"Should report no capability when the schema lacks the field", `{"errors":[{"extensions":{"code":"undefinedField"},"message":"Field 'x' doesn't exist on type 'y'"}]}`, false},
		{"Should report no capability when the query is forbidden", `{"errors":[{"type":"FORBIDDEN"}]}`, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
				if strings.Contains(query, "$oid") {
					return emptyGraphChecks
				}
				return testCase.response
			})
			defer closeServer()
			got, err := provider.Status(t.Context(), graphTopicRequest())
			if err != nil || got.Cause != "" || got.Checks == nil || got.Reviews != nil ||
				got.ReviewsUnavailable != testCase.unavailable {
				t.Fatalf("status = %+v err = %v", got, err)
			}
			if forgeStatusSDKResponse(got).ReviewsUnavailable != testCase.unavailable {
				t.Fatal("the SDK response lost the failed read")
			}
		})
	}
	t.Run("Should mark reviews unavailable when the connection dies", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var input struct{ Query string }
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			switch {
			case strings.Contains(input.Query, "$branch"):
				writeGraphBody(t, w, graphOpenPull)
			case strings.Contains(input.Query, "$oid"):
				writeGraphBody(t, w, emptyGraphChecks)
			default:
				panic(http.ErrAbortHandler)
			}
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || !got.ReviewsUnavailable || got.Checks == nil {
			t.Fatalf("status = %+v err = %v", got, err)
		}
	})
	t.Run("Should keep the newest threads and the full count past the contract bound", func(t *testing.T) {
		t.Parallel()
		reviews := &extensioncontract.ForgeReviews{UnresolvedThreads: extensioncontract.MaxForgeThreads + 5}
		for i := range extensioncontract.MaxForgeThreads + 5 {
			reviews.Threads = append(reviews.Threads, extensioncontract.ForgeReviewThread{ID: strconv.Itoa(i)})
		}
		bounded := boundGraphThreads(reviews)
		if len(bounded.Threads) != extensioncontract.MaxForgeThreads || !bounded.Truncated ||
			bounded.UnresolvedThreads != extensioncontract.MaxForgeThreads+5 || bounded.Threads[0].ID != "5" {
			t.Fatalf("bounded = %d threads, truncated %v", len(bounded.Threads), bounded.Truncated)
		}
	})
	t.Run("Should let a rate limit on reviews apply to the whole read", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := graphServer(t, func(query string, _ map[string]any) string {
			if strings.Contains(query, "$oid") {
				return emptyGraphChecks
			}
			return `{"errors":[{"type":"RATE_LIMITED"}]}`
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.Cause != extensioncontract.ForgeCauseRateLimited {
			t.Fatalf("status = %+v err = %v", got, err)
		}
	})
	t.Run("Should not read reviews of a pull request that ended", func(t *testing.T) {
		t.Parallel()
		var reviewCalls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var input struct{ Query string }
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			body := `{"data":{"repository":{"pullRequests":{"nodes":[{"number":2,"url":"https://github.com/acme/repo/pull/2","state":"MERGED","merged":true,"headRefOid":"head-a","headRepository":{"nameWithOwner":"acme/repo"}}],"pageInfo":{"hasNextPage":false}}}}}`
			switch {
			case strings.Contains(input.Query, "$oid"):
				body = emptyGraphChecks
			case strings.Contains(input.Query, "$number"):
				reviewCalls.Add(1)
			}
			if _, err := w.Write([]byte(body)); err != nil {
				t.Error(err)
			}
		})
		defer closeServer()
		got, err := provider.Status(t.Context(), graphTopicRequest())
		if err != nil || got.PRState == nil || *got.PRState != "merged" || reviewCalls.Load() != 0 {
			t.Fatalf("status = %+v err = %v review calls = %d", got, err, reviewCalls.Load())
		}
	})
	t.Run("Should bound candidate pages without confirming an incomplete population", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			_, _ = w.Write(
				[]byte(
					`{"data":{"repository":{"pullRequests":{"nodes":[{"number":1,"headRepository":{"nameWithOwner":"fork/repo"}}],"pageInfo":{"hasNextPage":true,"endCursor":"next"}}}}}`,
				),
			)
		})
		defer closeServer()
		got, err := provider.Status(
			t.Context(),
			extensioncontract.ForgeStatusRequest{RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "topic"},
		)
		if err != nil || got.PRNumber != nil || got.Selection != "incomplete" || calls.Load() != 4 {
			t.Fatalf("bounded=%#v %v calls=%d", got, err, calls.Load())
		}
	})
	t.Run("Should reject checks returned for another head", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var input struct{ Query string }
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				return
			}
			if strings.Contains(input.Query, "$oid") {
				_, _ = w.Write([]byte(`{"data":{"repository":{"object":{"oid":"head-b"}}}}`))
				return
			}
			_, _ = w.Write(
				[]byte(
					`{"data":{"repository":{"pullRequests":{"nodes":[{"number":2,"state":"OPEN","headRefOid":"head-a","headRepository":{"nameWithOwner":"acme/repo"}}],"pageInfo":{"hasNextPage":false}}}}}`,
				),
			)
		})
		defer closeServer()
		_, err := provider.Status(
			t.Context(),
			extensioncontract.ForgeStatusRequest{RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "topic"},
		)
		if err == nil {
			t.Fatal("mismatched checks accepted")
		}
	})
	for _, testCase := range []struct {
		name   string
		status int
	}{
		{"Should retain the HTTP rate limit reset without retrying", http.StatusForbidden},
		{"Should retain the GraphQL rate limit reset without retrying", http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "1800000000")
				w.WriteHeader(testCase.status)
				if testCase.status == http.StatusOK {
					if _, err := w.Write([]byte(`{"errors":[{"type":"RATE_LIMITED"}]}`)); err != nil {
						t.Error(err)
					}
				}
			})
			defer closeServer()
			got, err := provider.Status(
				t.Context(),
				extensioncontract.ForgeStatusRequest{
					RemoteURLs: []string{"https://github.com/acme/repo"},
					Branch:     "topic",
				},
			)
			if err != nil || got.Cause != extensioncontract.ForgeCauseRateLimited || got.RetryAfter == nil ||
				got.RetryAfter.Unix() != 1800000000 ||
				calls.Load() != 1 {
				t.Fatalf("rate limit=%#v %v", got, err)
			}
		})
	}
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

// Invariants: issue delivery advances only through the consumed batch; GitHub I/O stays in the provider.
func TestProviderIssues(t *testing.T) {
	t.Parallel()
	t.Run("Should bound titles and count deterministic attribute overflow", func(t *testing.T) {
		t.Parallel()
		issue := graphIssue{Number: 1, Title: strings.Repeat("界", 256), Body: strings.Repeat("界", 450)}
		for i := range 15 {
			issue.ClosedByPullRequestsReferences.Nodes = append(
				issue.ClosedByPullRequestsReferences.Nodes,
				issuePull{
					Number: i + 1,
					Title:  strings.Repeat("界", 200),
					URL:    fmt.Sprintf("https://example.com/pr/%d", i),
					State:  "OPEN",
				},
			)
		}
		for range 100 {
			issue.Labels.Nodes = append(issue.Labels.Nodes, struct{ Name string }{Name: strings.Repeat("label", 50)})
		}
		var first []byte
		for range 2 {
			result, err := issueBatch(
				[]graphIssue{issue},
				issuesParams{},
				repository{owner: "a", name: "b"},
				issuesState{},
				compozysdk.HeartbeatSourceLimits{MaxItems: 20, MaxSummaryBytes: 8192},
			)
			if err != nil || len(result.Items) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			item := result.Items[0]
			if utf8.RuneCountInString(item.Title) != 200 || len(item.Attributes) > 2048 {
				t.Fatalf("item=%+v", item)
			}
			var attrs struct {
				Omitted int `json:"omitted"`
			}
			if err := json.Unmarshal(item.Attributes, &attrs); err != nil || attrs.Omitted == 0 {
				t.Fatalf("attrs=%s err=%v", item.Attributes, err)
			}
			var canonical map[string]any
			if err := json.Unmarshal(item.Attributes, &canonical); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(canonical)
			if err != nil {
				t.Fatal(err)
			}
			if first != nil && !slices.Equal(data, first) {
				t.Fatal("nondeterministic overflow")
			}
			first = data
		}
	})
	t.Run("Should baseline without delivering old issues UT050", func(t *testing.T) {
		t.Parallel()
		provider := issueTestProvider(t, []map[string]any{issueFixture(1201), issueFixture(1202)}, http.StatusOK)
		result, err := provider.pollIssues(t.Context(), issueRequest("", `{"repo":"compozy/compozy"}`))
		if err != nil || len(result.Items) != 0 || string(result.State) != `{"max_number":1202}` {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("Should deliver issue fields and bounded Unicode excerpt UT051", func(t *testing.T) {
		t.Parallel()
		issue := issueFixture(1201)
		issue["body"] = strings.Repeat("界", 450)
		provider := issueTestProvider(t, []map[string]any{issue}, http.StatusOK)
		result, err := provider.pollIssues(
			t.Context(),
			issueRequest(`{"max_number":1200}`, `{"repo":"compozy/compozy"}`),
		)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var validated source.Result
		if err := json.Unmarshal(data, &validated); err != nil {
			t.Fatal(err)
		}
		if err := source.ValidateResult(validated, source.Limits{MaxItems: 20, MaxStateBytes: 16384}); err != nil {
			t.Fatal(err)
		}
		item := result.Items[0]
		var attrs struct {
			Author      string
			Labels      []string
			BodyExcerpt string `json:"body_excerpt"`
		}
		if err := json.Unmarshal(item.Attributes, &attrs); err != nil {
			t.Fatal(err)
		}
		if item.Key != "compozy/compozy#1201" || item.Title != "Issue 1201" ||
			item.URL != "https://github.com/compozy/compozy/issues/1201" ||
			attrs.Author != "pedro" ||
			!slices.Equal(attrs.Labels, []string{"bug"}) ||
			len([]rune(attrs.BodyExcerpt)) != 400 ||
			len(item.Summary) > 1024 || !utf8.ValidString(item.Summary) ||
			string(result.State) != `{"max_number":1201}` {
			t.Fatalf("item=%+v attrs=%+v state=%s", item, attrs, result.State)
		}
	})
	t.Run("Should combine closing and cross references from open PRs UT052", func(t *testing.T) {
		t.Parallel()
		issue := issueFixture(1201)
		open := map[string]any{
			"__typename": "PullRequest",
			"number":     1203,
			"url":        "https://github.com/compozy/compozy/pull/1203",
			"state":      "OPEN",
		}
		closed := map[string]any{"number": 1204, "url": "closed", "state": "CLOSED"}
		cross := map[string]any{"__typename": "PullRequest", "number": 1205, "url": "cross", "state": "OPEN"}
		issue["closedByPullRequestsReferences"] = map[string]any{"nodes": []any{open, closed}}
		issue["timelineItems"] = map[string]any{
			"nodes": []any{
				map[string]any{"source": open},
				map[string]any{"source": cross},
				map[string]any{"source": closed},
			},
		}
		provider := issueTestProvider(t, []map[string]any{issue}, http.StatusOK)
		result, err := provider.pollIssues(
			t.Context(),
			issueRequest(`{"max_number":1200}`, `{"repo":"compozy/compozy"}`),
		)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		var attrs struct {
			PRs []linkedIssuePR `json:"linked_open_prs"`
		}
		if err := json.Unmarshal(result.Items[0].Attributes, &attrs); err != nil {
			t.Fatal(err)
		}
		if len(attrs.PRs) != 2 || attrs.PRs[0].Number != 1203 || !attrs.PRs[0].Closes || attrs.PRs[1].Number != 1205 ||
			attrs.PRs[1].Closes {
			t.Fatalf("PRs=%+v", attrs.PRs)
		}
	})
	t.Run("Should drain oldest first without losing remaining issues UT053", func(t *testing.T) {
		t.Parallel()
		issues := make([]map[string]any, 0, 30)
		for n := 1230; n > 1200; n-- {
			issues = append(issues, issueFixture(n))
		}
		provider := issueTestProvider(t, issues, http.StatusOK)
		req := issueRequest(`{"max_number":1200}`, `{"repo":"compozy/compozy"}`)
		first, err := provider.pollIssues(t.Context(), req)
		if err != nil || len(first.Items) != 20 || !first.More || first.Items[0].Key != "compozy/compozy#1201" ||
			string(first.State) != `{"max_number":1220}` {
			t.Fatalf("first=%+v err=%v", first, err)
		}
		req.State = first.State
		second, err := provider.pollIssues(t.Context(), req)
		if err != nil || len(second.Items) != 10 || second.More || second.Items[0].Key != "compozy/compozy#1221" ||
			string(second.State) != `{"max_number":1230}` {
			t.Fatalf("second=%+v err=%v", second, err)
		}
		req.State = second.State
		last, err := provider.pollIssues(t.Context(), req)
		if err != nil || len(last.Items) != 0 || string(last.State) != string(second.State) {
			t.Fatalf("last=%+v err=%v", last, err)
		}
	})
	t.Run("Should filter labels and exclude pull requests UT055", func(t *testing.T) {
		t.Parallel()
		excluded := issueFixture(1202)
		excluded["labels"] = map[string]any{"nodes": []any{map[string]string{"name": "wontfix"}}}
		pr := issueFixture(1203)
		pr["__typename"] = "PullRequest"
		provider := issueTestProvider(t, []map[string]any{pr, excluded, issueFixture(1201)}, http.StatusOK)
		result, err := provider.pollIssues(
			t.Context(),
			issueRequest(
				`{"max_number":1200}`,
				`{"repo":"compozy/compozy","labels":["BUG"],"exclude_labels":["wontfix"]}`,
			),
		)
		if err != nil || len(result.Items) != 1 || result.Items[0].Key != "compozy/compozy#1201" ||
			string(result.State) != `{"max_number":1202}` {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("Should backfill within the first window across batches UT057", func(t *testing.T) {
		t.Parallel()
		old := issueFixture(1200)
		old["createdAt"] = "2026-08-10T00:00:00Z"
		provider := issueTestProvider(t, []map[string]any{issueFixture(1202), issueFixture(1201), old}, http.StatusOK)
		req := issueRequest("", `{"repo":"compozy/compozy","backfill":"24h"}`)
		req.Limits.MaxItems = 1
		first, err := provider.pollIssues(t.Context(), req)
		if err != nil || len(first.Items) != 1 || !first.More || string(first.State) != `{"max_number":1201}` {
			t.Fatalf("first=%+v err=%v", first, err)
		}
		req.State = first.State
		second, err := provider.pollIssues(t.Context(), req)
		if err != nil || len(second.Items) != 1 || second.More || string(second.State) != `{"max_number":1202}` {
			t.Fatalf("second=%+v err=%v", second, err)
		}
	})
}

func TestProviderIssuesErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status int
		absent bool
		code   string
	}{
		{"Should report missing credentials UT054", 200, true, "source_credential_absent"},
		{"Should report expired credentials UT054", 401, false, "source_credential_expired"},
		{"Should report secondary rate limits UT054", 403, false, "source_rate_limited"},
		{"Should report exhausted rate limits UT054", 429, false, "source_rate_limited"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := issueTestProvider(t, nil, tc.status)
			if tc.absent {
				provider.credentials.getenv = func(string) string { return "" }
			}
			_, err := provider.pollIssues(t.Context(), issueRequest("", `{"repo":"compozy/compozy"}`))
			rpc, ok := errors.AsType[*compozysdk.RPCError](err)
			if !ok || !strings.Contains(string(rpc.Data), tc.code) ||
				(tc.absent && !strings.Contains(rpc.Message, "gh auth login")) {
				t.Fatalf("error=%+v", err)
			}
		})
	}
	t.Run("Should require repo for a non GitHub origin UT056", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for _, args := range [][]string{{"init", root}, {"-C", root, "remote", "add", "origin", "https://gitlab.com/acme/repo.git"}} {
			if out, err := exec.CommandContext(t.Context(), "git", args...).CombinedOutput(); err != nil {
				t.Fatalf("git: %s %v", out, err)
			}
		}
		req := issueRequest("", `{}`)
		req.Workspace.RootDir = root
		_, _, err := resolveIssuesParams(t.Context(), req)
		rpc, ok := errors.AsType[*compozysdk.RPCError](err)
		if !ok || !strings.Contains(string(rpc.Data), "source_params_invalid") ||
			!strings.Contains(rpc.Message, "set with.repo") {
			t.Fatalf("error=%+v", err)
		}
		if out, err := exec.CommandContext(t.Context(), "git", "-C", root, "remote", "set-url", "origin", "git@github.com:compozy/compozy.git").
			CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
		_, repo, err := resolveIssuesParams(t.Context(), req)
		if err != nil || repo.owner != "compozy" || repo.name != "compozy" {
			t.Fatalf("repo=%+v err=%v", repo, err)
		}
	})
}

func issueFixture(number int) map[string]any {
	return map[string]any{
		"__typename": "Issue",
		"number":     number,
		"title":      fmt.Sprintf("Issue %d", number),
		"body":       "Details",
		"url":        fmt.Sprintf("https://github.com/compozy/compozy/issues/%d", number),
		"createdAt":  "2026-08-12T10:00:00Z",
		"author":     map[string]string{"login": "pedro"},
		"labels":     map[string]any{"nodes": []any{map[string]string{"name": "bug"}}},
	}
}
func issueRequest(state, params string) compozysdk.HeartbeatSourceRequest {
	return compozysdk.HeartbeatSourceRequest{
		Version: 1,
		Kind:    "issues",
		Params:  []byte(params),
		State:   []byte(state),
		Limits:  compozysdk.HeartbeatSourceLimits{MaxItems: 20, MaxSummaryBytes: 8192},
	}
}
func issueTestProvider(t *testing.T, issues []map[string]any, status int) *Provider {
	t.Helper()
	provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/graphql" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		var query struct{ Query string }
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
			t.Error(err)
		}
		if !strings.Contains(query.Query, "issues(states:OPEN") ||
			!strings.Contains(query.Query, "CROSS_REFERENCED_EVENT") {
			t.Errorf("unexpected query: %s", query.Query)
		}
		if status == http.StatusForbidden {
			w.Header().Set("X-RateLimit-Remaining", "0")
		}
		writeJSON(
			t,
			w,
			status,
			map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"issues": map[string]any{"nodes": issues, "pageInfo": map[string]any{"hasNextPage": false}},
					},
				},
			},
		)
	})
	t.Cleanup(closeServer)
	return provider
}

func TestProviderIssuesPagination(t *testing.T) {
	t.Parallel()
	t.Run("Should bound each poll and advance only through delivered oldest pages", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, r *http.Request) {
			var request struct{ Variables struct{ After *string } }
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			calls.Add(1)
			nodes := []map[string]any{issueFixture(1201), issueFixture(1202)}
			more, cursor := true, "page2"
			if request.Variables.After != nil {
				if *request.Variables.After != "page2" {
					t.Errorf("cursor=%s", *request.Variables.After)
				}
				nodes = []map[string]any{issueFixture(1203), issueFixture(1204)}
				more, cursor = false, ""
			}
			writeJSON(
				t,
				w,
				http.StatusOK,
				map[string]any{
					"data": map[string]any{
						"repository": map[string]any{
							"issues": map[string]any{
								"nodes":    nodes,
								"pageInfo": map[string]any{"hasNextPage": more, "endCursor": cursor},
							},
						},
					},
				},
			)
		})
		t.Cleanup(closeServer)
		req := issueRequest(`{"max_number":1200}`, `{"repo":"compozy/compozy"}`)
		req.Limits.MaxItems = 2
		req.Limits.MaxSummaryBytes = 3
		result, err := provider.pollIssues(t.Context(), req)
		if err != nil || calls.Load() != 1 || len(result.Items) != 2 || !result.More ||
			result.Items[0].Key != "compozy/compozy#1201" ||
			string(result.State) != `{"max_number":1202,"cursor":"page2"}` ||
			result.Items[0].Summary != "Det" ||
			result.Items[1].Summary != "" {
			t.Fatalf("result=%+v calls=%d err=%v", result, calls.Load(), err)
		}
		req.State = result.State
		result, err = provider.pollIssues(t.Context(), req)
		if err != nil || calls.Load() != 2 || result.More || len(result.Items) != 2 ||
			result.Items[0].Key != "compozy/compozy#1203" ||
			string(result.State) != `{"max_number":1204}` {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
	t.Run("Should map GraphQL rate limits without accepting partial data", func(t *testing.T) {
		t.Parallel()
		provider, closeServer := testProvider(t, func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(t, w, http.StatusOK, map[string]any{"errors": []any{map[string]string{"type": "RATE_LIMITED"}}})
		})
		t.Cleanup(closeServer)
		_, err := provider.pollIssues(t.Context(), issueRequest(`{"max_number":1200}`, `{"repo":"compozy/compozy"}`))
		rpc, ok := errors.AsType[*compozysdk.RPCError](err)
		if !ok || !strings.Contains(string(rpc.Data), "source_rate_limited") {
			t.Fatalf("error=%v", err)
		}
	})
}
