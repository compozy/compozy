package coderabbit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/looper/internal/looper/provider"
)

func TestFetchReviewsFiltersResolvedThreadsAndNonBotComments(t *testing.T) {
	t.Parallel()

	run := func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
			return []byte(`{"owner":{"login":"acme"},"name":"looper"}`), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/looper/pulls/259/comments"):
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

	items, err := New(WithCommandRunner(run)).FetchReviews(context.Background(), "259")
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
