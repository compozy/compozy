package coderabbit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/provider"
)

func TestParseNitpickReviewItemsDeduplicatesHashesAndKeepsNewestReview(t *testing.T) {
	t.Parallel()

	sharedTitle := "Prefer reusing existing stop-reason helper to avoid duplicated normalization."
	sharedBody := "This block duplicates logic already present in the same package (`sessionMetaStopReason`). Reusing the helper keeps stop normalization behavior centralized."
	reviews := []pullRequestReview{
		{
			ID: 4089982130,
			Body: testNitpickReviewBody(
				testNitpickFileSection("internal/session/query.go", "213-216", sharedTitle, sharedBody),
			),
			SubmittedAt: "2026-04-10T13:33:25Z",
			User: struct {
				Login string `json:"login"`
			}{Login: defaultBotLogin},
		},
		{
			ID:          4090227334,
			Body:        "",
			SubmittedAt: "2026-04-10T14:10:44Z",
			User: struct {
				Login string `json:"login"`
			}{Login: defaultBotLogin},
		},
		{
			ID: 4090314487,
			Body: testNitpickReviewBody(
				testNitpickFileSection("internal/session/query.go", "213-216", sharedTitle, sharedBody),
				testNitpickFileSection(
					"internal/session/query_test.go",
					"358-371",
					"Split the legacy-path assertions into an explicit subtest.",
					"This introduces a second scenario in the same test body; isolating it in `t.Run(\"Should ...\")` keeps failures scoped and aligns with the test conventions.",
				),
			),
			SubmittedAt: "2026-04-10T14:24:56Z",
			User: struct {
				Login string `json:"login"`
			}{Login: defaultBotLogin},
		},
		{
			ID:          4090314499,
			Body:        testNitpickReviewBody("internal/session/query.go", "213-216", sharedTitle, sharedBody),
			SubmittedAt: "2026-04-10T14:25:00Z",
			User: struct {
				Login string `json:"login"`
			}{Login: "pedro"},
		},
	}

	items := parseNitpickReviewItems(reviews, defaultBotLogin)
	if len(items) != 2 {
		t.Fatalf("expected 2 deduped nitpicks, got %d (%#v)", len(items), items)
	}

	hash := buildNitpickHash("internal/session/query.go", "213-216", sharedTitle, sharedBody)
	itemByHash := make(map[string]provider.ReviewItem, len(items))
	for _, item := range items {
		itemByHash[item.ReviewHash] = item
	}

	queryItem, ok := itemByHash[hash]
	if !ok {
		t.Fatalf("expected query.go nitpick hash %q, got %#v", hash, itemByHash)
	}
	if queryItem.SourceReviewID != "4090314487" {
		t.Fatalf("expected newest review id to win, got %q", queryItem.SourceReviewID)
	}
	if queryItem.ProviderRef != "review:4090314487,nitpick_hash:"+hash {
		t.Fatalf("unexpected provider ref: %q", queryItem.ProviderRef)
	}
	if queryItem.Line != 213 || queryItem.Severity != nitpickSeverity {
		t.Fatalf("unexpected query nitpick metadata: %#v", queryItem)
	}
}

func TestParseNitpickReviewItemsKeepsLocationsDistinct(t *testing.T) {
	t.Parallel()

	t.Run("Should keep identical nitpicks at different locations as separate items", func(t *testing.T) {
		t.Parallel()

		sharedTitle := "Prefer reusing existing stop-reason helper to avoid duplicated normalization."
		sharedBody := "This block duplicates logic already present in the same package (`sessionMetaStopReason`). Reusing the helper keeps stop normalization behavior centralized."
		reviews := []pullRequestReview{
			{
				ID: 4090314487,
				Body: testNitpickReviewBody(
					testNitpickFileSection("internal/session/query.go", "213-216", sharedTitle, sharedBody),
					testNitpickFileSection("internal/session/query.go", "240-243", sharedTitle, sharedBody),
				),
				SubmittedAt: "2026-04-10T14:24:56Z",
				User: struct {
					Login string `json:"login"`
				}{Login: defaultBotLogin},
			},
		}

		items := parseNitpickReviewItems(reviews, defaultBotLogin)
		if len(items) != 2 {
			t.Fatalf("expected 2 nitpick items, got %d (%#v)", len(items), items)
		}

		firstHash := buildNitpickHash("internal/session/query.go", "213-216", sharedTitle, sharedBody)
		secondHash := buildNitpickHash("internal/session/query.go", "240-243", sharedTitle, sharedBody)
		if firstHash == secondHash {
			t.Fatalf("expected distinct hashes for distinct locations, got %q", firstHash)
		}

		itemByHash := make(map[string]provider.ReviewItem, len(items))
		for _, item := range items {
			itemByHash[item.ReviewHash] = item
		}
		if _, ok := itemByHash[firstHash]; !ok {
			t.Fatalf("expected first nitpick hash %q, got %#v", firstHash, itemByHash)
		}
		if _, ok := itemByHash[secondHash]; !ok {
			t.Fatalf("expected second nitpick hash %q, got %#v", secondHash, itemByHash)
		}
	})
}

func TestFetchReviewsSkipsPullRequestReviewsWhenNitpicksDisabled(t *testing.T) {
	t.Parallel()

	reviewsEndpointCalled := false
	run := func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
			return []byte(`{"owner":{"login":"acme"},"name":"compozy"}`), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/comments"):
			return []byte(`[]`), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "graphql":
			return []byte(
				`{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}}`,
			), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/reviews"):
			reviewsEndpointCalled = true
			return []byte(`[]`), nil
		default:
			return nil, errors.New("unexpected gh invocation: " + strings.Join(args, " "))
		}
	}

	items, err := New(WithCommandRunner(run)).FetchReviews(context.Background(), provider.FetchRequest{PR: "259"})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items, got %#v", items)
	}
	if reviewsEndpointCalled {
		t.Fatal("expected pull request reviews endpoint to be skipped without --nitpicks")
	}
}

func TestFetchReviewsIncludesNitpicksWhenRequested(t *testing.T) {
	t.Parallel()

	reviewsEndpointCalled := false
	run := func(_ context.Context, args ...string) ([]byte, error) {
		switch {
		case len(args) >= 4 && args[0] == "repo" && args[1] == "view":
			return []byte(`{"owner":{"login":"acme"},"name":"compozy"}`), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/comments"):
			return []byte(`[]`), nil
		case len(args) >= 2 && args[0] == "api" && args[1] == "graphql":
			return []byte(
				`{"data":{"repository":{"pullRequest":{"reviewThreads":{"pageInfo":{"hasNextPage":false,"endCursor":""},"nodes":[]}}}}}`,
			), nil
		case len(args) >= 2 && args[0] == "api" && strings.HasPrefix(args[1], "repos/acme/compozy/pulls/259/reviews"):
			reviewsEndpointCalled = true
			return []byte(fmt.Sprintf(`[
				{"id":4089982130,"submitted_at":"2026-04-10T13:33:25Z","body":%q,"user":{"login":"%s"}}
			]`, testNitpickReviewBody(
				testNitpickFileSection(
					"internal/session/query.go",
					"213-216",
					"Prefer reusing existing stop-reason helper to avoid duplicated normalization.",
					"This block duplicates logic already present in the same package (`sessionMetaStopReason`). Reusing the helper keeps stop normalization behavior centralized.",
				),
			), defaultBotLogin)), nil
		default:
			return nil, errors.New("unexpected gh invocation: " + strings.Join(args, " "))
		}
	}

	items, err := New(WithCommandRunner(run)).FetchReviews(context.Background(), provider.FetchRequest{
		PR:              "259",
		IncludeNitpicks: true,
	})
	if err != nil {
		t.Fatalf("fetch reviews: %v", err)
	}
	if !reviewsEndpointCalled {
		t.Fatal("expected pull request reviews endpoint to be used when nitpicks are enabled")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 nitpick item, got %#v", items)
	}
	if items[0].Severity != nitpickSeverity || items[0].ReviewHash == "" {
		t.Fatalf("unexpected nitpick item metadata: %#v", items[0])
	}
}

func testNitpickReviewBody(fileSections ...string) string {
	joinedSections := strings.Join(fileSections, "\n\n")
	return strings.Join([]string{
		"<details>",
		fmt.Sprintf("<summary>🧹 Nitpick comments (%d)</summary><blockquote>", len(fileSections)),
		"",
		joinedSections,
		"",
		"</blockquote></details>",
		"",
	}, "\n")
}

func testNitpickFileSection(filePath string, lineRange string, title string, body string) string {
	return strings.Join([]string{
		"<details>",
		fmt.Sprintf("<summary>%s (1)</summary><blockquote>", filePath),
		"",
		fmt.Sprintf("`%s`: **%s**", lineRange, title),
		"",
		body,
		"",
		"<details>",
		"<summary>♻️ Proposed refactor</summary>",
		"",
		"```diff",
		"+ ignored nested details",
		"```",
		"</details>",
		"",
		"<details>",
		"<summary>🤖 Prompt for AI Agents</summary>",
		"",
		"```",
		"ignored prompt details",
		"```",
		"</details>",
		"",
		"</blockquote></details>",
	}, "\n")
}
