package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestClientReviewRequestsEncodeDaemonPathsAndBodies(t *testing.T) {
	t.Run("fetch review trims input and returns created state", func(t *testing.T) {
		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Timeout: time.Second,
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost {
						t.Fatalf("method = %s, want POST", req.Method)
					}
					if req.URL.Path != "/api/reviews/demo/fetch" {
						t.Fatalf("path = %s, want /api/reviews/demo/fetch", req.URL.Path)
					}
					var payload map[string]any
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode fetch body: %v", err)
					}
					if payload["workspace"] != "/tmp/workspace" ||
						payload["provider"] != "sdk-review" ||
						payload["pr_ref"] != "123" ||
						payload["round"] != float64(7) {
						t.Fatalf("unexpected fetch payload: %#v", payload)
					}
					return jsonResponse(http.StatusCreated, `{"review":{"workflow_slug":"demo","round_number":7}}`), nil
				}),
			},
		}

		round := 7
		result, err := client.FetchReview(
			context.Background(),
			" /tmp/workspace ",
			" demo ",
			apicore.ReviewFetchRequest{
				Provider: " sdk-review ",
				PRRef:    " 123 ",
				Round:    &round,
			},
		)
		if err != nil {
			t.Fatalf("FetchReview() error = %v", err)
		}
		if !result.Created || result.Summary.RoundNumber != 7 {
			t.Fatalf("unexpected fetch result: %#v", result)
		}
	})

	t.Run("review lookups use expected GET paths", func(t *testing.T) {
		requests := make([]string, 0, 2)
		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Timeout: time.Second,
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests = append(requests, req.Method+" "+req.URL.RequestURI())
					switch req.URL.Path {
					case "/api/reviews/demo":
						return jsonResponse(http.StatusOK, `{"review":{"workflow_slug":"demo","round_number":8}}`), nil
					case "/api/reviews/demo/rounds/8":
						return jsonResponse(
							http.StatusOK,
							`{"round":{"id":"round-8","workflow_slug":"demo","round_number":8}}`,
						), nil
					case "/api/reviews/demo/rounds/8/issues":
						return jsonResponse(
							http.StatusOK,
							`{"issues":[{"id":"issue-1","issue_number":1,"status":"pending"}]}`,
						), nil
					default:
						t.Fatalf("unexpected request URI: %s", req.URL.RequestURI())
						return nil, nil
					}
				}),
			},
		}

		latest, err := client.GetLatestReview(context.Background(), "/tmp/workspace", "demo")
		if err != nil {
			t.Fatalf("GetLatestReview() error = %v", err)
		}
		if latest.RoundNumber != 8 {
			t.Fatalf("unexpected latest review: %#v", latest)
		}

		round, err := client.GetReviewRound(context.Background(), "/tmp/workspace", "demo", 8)
		if err != nil {
			t.Fatalf("GetReviewRound() error = %v", err)
		}
		if round.ID != "round-8" || round.RoundNumber != 8 {
			t.Fatalf("unexpected review round: %#v", round)
		}

		issues, err := client.ListReviewIssues(context.Background(), "/tmp/workspace", "demo", 8)
		if err != nil {
			t.Fatalf("ListReviewIssues() error = %v", err)
		}
		if len(issues) != 1 || issues[0].IssueNumber != 1 {
			t.Fatalf("unexpected review issues: %#v", issues)
		}

		for _, want := range []string{
			"GET /api/reviews/demo?workspace=%2Ftmp%2Fworkspace",
			"GET /api/reviews/demo/rounds/8?workspace=%2Ftmp%2Fworkspace",
			"GET /api/reviews/demo/rounds/8/issues?workspace=%2Ftmp%2Fworkspace",
		} {
			if !containsRequest(requests, want) {
				t.Fatalf("expected requests to contain %q, got %#v", want, requests)
			}
		}
	})

	t.Run("start review run preserves runtime override payloads", func(t *testing.T) {
		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Timeout: time.Second,
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost {
						t.Fatalf("method = %s, want POST", req.Method)
					}
					if req.URL.Path != "/api/reviews/demo/rounds/9/runs" {
						t.Fatalf("path = %s, want /api/reviews/demo/rounds/9/runs", req.URL.Path)
					}
					var payload map[string]any
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode review run body: %v", err)
					}
					runtimeOverrides, ok := payload["runtime_overrides"].(map[string]any)
					if !ok || runtimeOverrides["dry_run"] != true {
						t.Fatalf("unexpected runtime overrides payload: %#v", payload)
					}
					batching, ok := payload["batching"].(map[string]any)
					if !ok || batching["batch_size"] != float64(2) {
						t.Fatalf("unexpected batching payload: %#v", payload)
					}
					return jsonResponse(http.StatusCreated, `{"run":{"run_id":"review-run-9","mode":"review"}}`), nil
				}),
			},
		}

		run, err := client.StartReviewRun(context.Background(), "/tmp/workspace", "demo", 9, apicore.ReviewRunRequest{
			PresentationMode: "detach",
			RuntimeOverrides: json.RawMessage(`{"dry_run":true}`),
			Batching:         json.RawMessage(`{"batch_size":2}`),
		})
		if err != nil {
			t.Fatalf("StartReviewRun() error = %v", err)
		}
		if run.RunID != "review-run-9" || run.Mode != "review" {
			t.Fatalf("unexpected review run: %#v", run)
		}
	})

	t.Run("start task run escapes workflow slug in request path", func(t *testing.T) {
		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Timeout: time.Second,
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.Method != http.MethodPost {
						t.Fatalf("method = %s, want POST", req.Method)
					}
					if req.URL.EscapedPath() != "/api/tasks/demo%20alpha%2Fbeta/runs" {
						t.Fatalf(
							"escaped path = %s, want /api/tasks/demo%%20alpha%%2Fbeta/runs",
							req.URL.EscapedPath(),
						)
					}
					return jsonResponse(http.StatusCreated, `{"run":{"run_id":"task-run-1","mode":"task"}}`), nil
				}),
			},
		}

		run, err := client.StartTaskRun(context.Background(), " demo alpha/beta ", apicore.TaskRunRequest{
			Workspace: "/tmp/workspace",
		})
		if err != nil {
			t.Fatalf("StartTaskRun() error = %v", err)
		}
		if run.RunID != "task-run-1" || run.Mode != "task" {
			t.Fatalf("unexpected task run: %#v", run)
		}
	})
}

func TestClientExecRequestAndGuardErrors(t *testing.T) {
	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Timeout: time.Second,
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPost {
					t.Fatalf("method = %s, want POST", req.Method)
				}
				if req.URL.Path != "/api/exec" {
					t.Fatalf("path = %s, want /api/exec", req.URL.Path)
				}
				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("read exec request body: %v", err)
				}
				if !strings.Contains(string(body), `"workspace_path":"/tmp/workspace"`) ||
					!strings.Contains(string(body), `"presentation_mode":"stream"`) {
					t.Fatalf("unexpected exec request body: %s", body)
				}
				return jsonResponse(http.StatusCreated, `{"run":{"run_id":"exec-run-1","mode":"exec"}}`), nil
			}),
		},
	}

	run, err := client.StartExecRun(context.Background(), apicore.ExecRequest{
		WorkspacePath:    "/tmp/workspace",
		Prompt:           "Summarize",
		PresentationMode: "stream",
		RuntimeOverrides: json.RawMessage(`{"persist":true}`),
	})
	if err != nil {
		t.Fatalf("StartExecRun() error = %v", err)
	}
	if run.RunID != "exec-run-1" || run.Mode != "exec" {
		t.Fatalf("unexpected exec run: %#v", run)
	}

	var nilClient *Client
	if _, err := nilClient.StartExecRun(context.Background(), apicore.ExecRequest{}); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil StartExecRun() error = %v", err)
	}
	if _, err := nilClient.FetchReview(context.Background(), "", "", apicore.ReviewFetchRequest{}); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil FetchReview() error = %v", err)
	}
	if _, err := client.GetLatestReview(context.Background(), "/tmp/workspace", " "); !errors.Is(
		err,
		ErrWorkflowSlugRequired,
	) {
		t.Fatalf("blank GetLatestReview() error = %v", err)
	}
	if _, err := client.GetReviewRound(context.Background(), "/tmp/workspace", " ", 1); !errors.Is(
		err,
		ErrWorkflowSlugRequired,
	) {
		t.Fatalf("blank GetReviewRound() error = %v", err)
	}
	if _, err := client.ListReviewIssues(context.Background(), "/tmp/workspace", " ", 1); !errors.Is(
		err,
		ErrWorkflowSlugRequired,
	) {
		t.Fatalf("blank ListReviewIssues() error = %v", err)
	}
	if _, err := client.StartReviewRun(
		context.Background(),
		"/tmp/workspace",
		" ",
		1,
		apicore.ReviewRunRequest{},
	); !errors.Is(err, ErrWorkflowSlugRequired) {
		t.Fatalf("blank StartReviewRun() error = %v", err)
	}
}

func TestClientStartTaskRunRejectsNilContext(t *testing.T) {
	t.Parallel()

	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Timeout: time.Second,
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Fatal("unexpected transport call for nil context")
				return nil, nil
			}),
		},
	}

	var nilCtx context.Context
	_, err := client.StartTaskRun(nilCtx, "demo", apicore.TaskRunRequest{Workspace: "/tmp/workspace"})
	if !errors.Is(err, ErrDaemonContextRequired) {
		t.Fatalf("StartTaskRun(nil) error = %v, want %v", err, ErrDaemonContextRequired)
	}
}

func TestClientRunStreamSendItemBackpressure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &clientRunStream{
		items: make(chan RunStreamItem, 1),
		ctx:   ctx,
	}
	stream.items <- RunStreamItem{
		Heartbeat: &RunStreamHeartbeat{Timestamp: time.Now().UTC()},
	}

	resultCh := make(chan error, 1)
	go func() {
		resultCh <- stream.sendItem(RunStreamItem{
			Overflow: &RunStreamOverflow{Reason: "slow consumer"},
		})
	}()

	select {
	case err := <-resultCh:
		t.Fatalf("sendItem() returned before buffer space was available: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	<-stream.items

	select {
	case err := <-resultCh:
		if err != nil {
			t.Fatalf("sendItem() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("sendItem() did not resume after buffer space became available")
	}
}

func TestClientRunStreamSendItemReturnsContextErrorOnCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	stream := &clientRunStream{
		items: make(chan RunStreamItem, 1),
		ctx:   ctx,
	}
	stream.items <- RunStreamItem{
		Heartbeat: &RunStreamHeartbeat{Timestamp: time.Now().UTC()},
	}
	cancel()

	err := stream.sendItem(RunStreamItem{
		Overflow: &RunStreamOverflow{Reason: "slow consumer"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("sendItem() error = %v, want context.Canceled", err)
	}
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func containsRequest(requests []string, want string) bool {
	for _, request := range requests {
		if request == want {
			return true
		}
	}
	return false
}
