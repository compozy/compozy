package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	extension "github.com/compozy/compozy/sdk/extension"
)

type record struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type recorder struct {
	path string
	mu   sync.Mutex
}

func main() {
	rec := &recorder{path: strings.TrimSpace(os.Getenv("COMPOZY_SDK_RECORD_PATH"))}
	mode := strings.TrimSpace(os.Getenv("COMPOZY_SDK_REVIEW_MODE"))
	providerName := strings.TrimSpace(os.Getenv("COMPOZY_SDK_REVIEW_PROVIDER"))
	if providerName == "" {
		providerName = "sdk-review"
	}

	ext := extension.New("sdk-review-ext", "1.0.0")
	if mode != "missing_capability" {
		ext.WithCapabilities(extension.CapabilityProvidersRegister)
	}

	if mode != "missing_registration" {
		ext.RegisterReviewProvider(providerName, extension.ReviewProvider{
			FetchReviewsFunc: func(
				_ context.Context,
				ctx extension.ReviewProviderContext,
				req extension.FetchRequest,
			) ([]extension.ReviewItem, error) {
				rec.write("fetch_reviews", map[string]any{
					"provider":         ctx.Provider,
					"pr":               req.PR,
					"include_nitpicks": req.IncludeNitpicks,
				})
				return []extension.ReviewItem{{
					Title:       "Fetched review",
					Body:        "from go sdk review provider",
					ProviderRef: "thread-go-1",
				}}, nil
			},
			ResolveIssuesFunc: func(
				_ context.Context,
				ctx extension.ReviewProviderContext,
				req extension.ResolveIssuesRequest,
			) error {
				rec.write("resolve_issues", map[string]any{
					"provider": ctx.Provider,
					"pr":       req.PR,
					"issues":   req.Issues,
				})
				return nil
			},
		})
	}

	if err := ext.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (r *recorder) write(kind string, payload map[string]any) {
	if r == nil || strings.TrimSpace(r.path) == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	raw, err := json.Marshal(record{Type: strings.TrimSpace(kind), Payload: payload})
	if err != nil {
		r.logf("marshal record: %v", err)
		return
	}

	file, err := os.OpenFile(r.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		r.logf("open record file %q: %v", r.path, err)
		return
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			r.logf("close record file %q: %v", r.path, closeErr)
		}
	}()

	if _, err := file.Write(append(raw, '\n')); err != nil {
		r.logf("write record file %q: %v", r.path, err)
	}
}

func (r *recorder) logf(format string, args ...any) {
	if r == nil {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "sdk_review_extension recorder: "+format+"\n", args...)
}
