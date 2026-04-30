package coderabbit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/provider"
)

func TestFetchReviewsFiltersResolvedThreadsAndNonBotComments(t *testing.T) {
	t.Parallel()

	run := func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
			return []byte(`{"owner":{"login":"acme"},"name":"compozy"}`), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/comments"):
			return []byte(`[
				{"id":101,"node_id":"RC_101","body":"Please add a nil check","path":"internal/app/service.go","line":42,"user":{"login":"coderabbitai[bot]"}},
				{"id":102,"node_id":"RC_102","body":"Already resolved thread","path":"internal/app/service.go","line":51,"user":{"login":"coderabbitai[bot]"}},
				{"id":103,"node_id":"RC_103","body":"Human review comment","path":"internal/app/service.go","line":99,"user":{"login":"pedro"}}
			]`), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "graphql":
			return []byte(`{
				"data":{
					"repository":{
						"pullRequest":{
							"reviewThreads":{
								"pageInfo":{"hasNextPage":false,"endCursor":""},
								"nodes":[
									{"id":"PRT_1","isResolved":false,"comments":{"nodes":[{"id":"comment-1","databaseId":101}]}},
									{"id":"PRT_2","isResolved":true,"comments":{"nodes":[{"id":"comment-2","databaseId":102}]}}
								]
							}
						}
					}
				}
			}`), nil
		default:
			return nil, errors.New("unexpected gh invocation: " + strings.Join(args, " "))
		}
	}

	items, err := New(WithCommandRunner(run)).FetchReviews(context.Background(), provider.FetchRequest{PR: "259"})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 unresolved bot review item, got %d", len(items))
	}
	if items[0].File != "internal/app/service.go" || items[0].Line != 42 {
		t.Fatalf("unexpected item location: %#v", items[0])
	}
	if items[0].ProviderRef != "thread:PRT_1,comment:RC_101" {
		t.Fatalf("unexpected provider ref: %q", items[0].ProviderRef)
	}
}

func TestResolveIssuesDeduplicatesThreadsAndAggregatesErrors(t *testing.T) {
	t.Parallel()

	var resolvedThreads []string
	run := func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) < 2 || args[0] != "api" || args[1] != "graphql" {
			return nil, errors.New("unexpected gh invocation")
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "threadId=") {
				threadID := strings.TrimPrefix(arg, "threadId=")
				resolvedThreads = append(resolvedThreads, threadID)
				if threadID == "PRT_2" {
					return nil, errors.New("boom")
				}
			}
		}
		return []byte(`{"data":{"resolveReviewThread":{"thread":{"isResolved":true}}}}`), nil
	}

	err := New(WithCommandRunner(run)).ResolveIssues(context.Background(), "259", []provider.ResolvedIssue{
		{FilePath: "/tmp/issue_001.md", ProviderRef: "thread:PRT_1,comment:RC_1"},
		{FilePath: "/tmp/issue_002.md", ProviderRef: "thread:PRT_1,comment:RC_2"},
		{FilePath: "/tmp/issue_003.md", ProviderRef: "thread:PRT_2,comment:RC_3"},
	})
	if err == nil {
		t.Fatal("expected aggregated resolve error")
	}
	if len(resolvedThreads) != 2 {
		t.Fatalf("expected 2 unique thread resolutions, got %d (%v)", len(resolvedThreads), resolvedThreads)
	}
	if !strings.Contains(err.Error(), "PRT_2") {
		t.Fatalf("expected aggregated error to mention failing thread, got %v", err)
	}
}

func TestWatchStatusClassifiesCodeRabbitReviewState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		headSHA    string
		reviews    []pullRequestReview
		wantState  provider.WatchStatusState
		wantCommit string
	}{
		{
			name:    "pending when latest provider review has not been submitted",
			headSHA: "head-new",
			reviews: []pullRequestReview{
				testPullRequestReview(4100, "old-head", "COMMENTED", "2026-04-10T13:33:25Z", defaultBotLogin, ""),
				testPullRequestReview(4101, "old-head", "PENDING", "", defaultBotLogin, ""),
			},
			wantState:  provider.WatchStatusPending,
			wantCommit: "old-head",
		},
		{
			name:    "stale when latest submitted provider review is for an older head",
			headSHA: "head-new",
			reviews: []pullRequestReview{
				testPullRequestReview(4101, "old-head", "COMMENTED", "2026-04-10T13:33:25Z", defaultBotLogin, ""),
			},
			wantState:  provider.WatchStatusStale,
			wantCommit: "old-head",
		},
		{
			name:    "current reviewed when latest submitted provider review matches head",
			headSHA: "head-current",
			reviews: []pullRequestReview{
				testPullRequestReview(4101, "old-head", "COMMENTED", "2026-04-10T13:33:25Z", defaultBotLogin, ""),
				testPullRequestReview(4102, "head-current", "COMMENTED", "2026-04-10T14:33:25Z", defaultBotLogin, ""),
			},
			wantState:  provider.WatchStatusCurrentReviewed,
			wantCommit: "head-current",
		},
		{
			name:    "pending when no provider review exists",
			headSHA: "head-without-review",
			reviews: []pullRequestReview{
				testPullRequestReview(4103, "head-without-review", "COMMENTED", "2026-04-10T14:33:25Z", "pedro", ""),
			},
			wantState: provider.WatchStatusPending,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := New(WithCommandRunner(testWatchStatusRunner(t, tt.headSHA, tt.reviews))).
				WatchStatus(context.Background(), provider.WatchStatusRequest{PR: "259"})
			if err != nil {
				t.Fatalf("watch status: %v", err)
			}
			if status.PRHeadSHA != tt.headSHA {
				t.Fatalf("PRHeadSHA = %q, want %q", status.PRHeadSHA, tt.headSHA)
			}
			if status.State != tt.wantState {
				t.Fatalf("State = %q, want %q", status.State, tt.wantState)
			}
			if status.ReviewCommitSHA != tt.wantCommit {
				t.Fatalf("ReviewCommitSHA = %q, want %q", status.ReviewCommitSHA, tt.wantCommit)
			}
		})
	}
}

func TestWatchStatusRejectsMalformedPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should reject incomplete pull request metadata", func(t *testing.T) {
		t.Parallel()

		run := func(_ context.Context, args ...string) ([]byte, error) {
			switch {
			case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
				return []byte(`{"owner":{"login":"acme"},"name":"compozy"}`), nil
			case len(args) >= 2 && args[0] == "api" && args[1] == "repos/acme/compozy/pulls/259":
				return []byte(`{"head":{}}`), nil
			default:
				return nil, errors.New("unexpected gh invocation: " + strings.Join(args, " "))
			}
		}

		_, err := New(WithCommandRunner(run)).WatchStatus(context.Background(), provider.WatchStatusRequest{PR: "259"})
		if err == nil {
			t.Fatal("expected malformed metadata error")
		}
		if !strings.Contains(err.Error(), "pull request metadata response is incomplete") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Should reject malformed submitted_at on submitted provider review", func(t *testing.T) {
		t.Parallel()

		reviews := []pullRequestReview{
			testPullRequestReview(4104, "head-current", "COMMENTED", "not-a-time", defaultBotLogin, ""),
		}
		_, err := New(WithCommandRunner(testWatchStatusRunner(t, "head-current", reviews))).
			WatchStatus(context.Background(), provider.WatchStatusRequest{PR: "259"})
		if err == nil {
			t.Fatal("expected malformed submitted_at error")
		}
		if !strings.Contains(err.Error(), "decode pull request review 4104 submitted_at") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestWatchStatusAndFetchReviewsUseRepresentativeGHPayloads(t *testing.T) {
	t.Parallel()

	reviewBody := testReviewBodyCommentBlock(
		"🧹 Nitpick comments",
		testReviewBodyCommentFileSection(
			"internal/session/query.go",
			"213-216",
			"Prefer reusing existing stop-reason helper.",
			"This block duplicates logic already present in the same package.",
		),
	)
	reviews := []pullRequestReview{
		testPullRequestReview(4105, "head-current", "COMMENTED", "2026-04-10T14:33:25Z", defaultBotLogin, reviewBody),
	}
	comments := []pullRequestComment{testPullRequestComment(
		101,
		"RC_101",
		"Please add a nil check",
		"internal/app/service.go",
		42,
		defaultBotLogin,
	)}
	run := testFullReviewRunner(t, "head-current", reviews, comments)
	providerUnderTest := New(WithCommandRunner(run))

	status, err := providerUnderTest.WatchStatus(context.Background(), provider.WatchStatusRequest{PR: "259"})
	if err != nil {
		t.Fatalf("watch status: %v", err)
	}
	if status.State != provider.WatchStatusCurrentReviewed {
		t.Fatalf("watch status = %q, want %q", status.State, provider.WatchStatusCurrentReviewed)
	}

	items, err := providerUnderTest.FetchReviews(context.Background(), provider.FetchRequest{
		PR:              "259",
		IncludeNitpicks: true,
	})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected inline and review-body items, got %d (%#v)", len(items), items)
	}
	if !hasReviewItem(items, "internal/app/service.go", "thread:PRT_1,comment:RC_101") {
		t.Fatalf("missing normalized inline item: %#v", items)
	}
	if !hasReviewBodyItem(items, "internal/session/query.go", reviewBodyCommentSeverityNitpick) {
		t.Fatalf("missing normalized review-body item: %#v", items)
	}
}

func testWatchStatusRunner(t *testing.T, headSHA string, reviews []pullRequestReview) CommandRunner {
	t.Helper()
	return testFullReviewRunner(t, headSHA, reviews, nil)
}

func testFullReviewRunner(
	t *testing.T,
	headSHA string,
	reviews []pullRequestReview,
	comments []pullRequestComment,
) CommandRunner {
	t.Helper()
	return func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
			return []byte(`{"owner":{"login":"acme"},"name":"compozy"}`), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "repos/acme/compozy/pulls/259":
			pr := pullRequest{}
			pr.Head.SHA = headSHA
			return mustMarshalJSON(t, pr), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/reviews"):
			return mustMarshalJSON(t, reviews), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/comments"):
			return mustMarshalJSON(t, comments), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "graphql":
			return []byte(`{
				"data":{
					"repository":{
						"pullRequest":{
							"reviewThreads":{
								"pageInfo":{"hasNextPage":false,"endCursor":""},
								"nodes":[
									{"id":"PRT_1","isResolved":false,"comments":{"nodes":[{"id":"comment-1","databaseId":101}]}}
								]
							}
						}
					}
				}
			}`), nil
		default:
			return nil, errors.New("unexpected gh invocation: " + strings.Join(args, " "))
		}
	}
}

func testPullRequestReview(
	id int,
	commitID string,
	state string,
	submittedAt string,
	login string,
	body string,
) pullRequestReview {
	review := pullRequestReview{
		ID:          id,
		Body:        body,
		CommitID:    commitID,
		State:       state,
		SubmittedAt: submittedAt,
	}
	review.User.Login = login
	return review
}

func testPullRequestComment(
	id int,
	nodeID string,
	body string,
	path string,
	line int,
	login string,
) pullRequestComment {
	comment := pullRequestComment{
		ID:     id,
		NodeID: nodeID,
		Body:   body,
		Path:   path,
		Line:   line,
	}
	comment.User.Login = login
	return comment
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal test JSON: %v", err)
	}
	return payload
}

func hasReviewItem(items []provider.ReviewItem, file string, providerRef string) bool {
	for idx := range items {
		item := &items[idx]
		if item.File == file && item.ProviderRef == providerRef {
			return true
		}
	}
	return false
}

func hasReviewBodyItem(items []provider.ReviewItem, file string, severity string) bool {
	for idx := range items {
		item := &items[idx]
		if item.File == file && item.Severity == severity && item.ReviewHash != "" {
			return true
		}
	}
	return false
}
