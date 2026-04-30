package provider

import (
	"context"
	"errors"
	"testing"
)

type unsupportedWatchProvider struct{}

func (unsupportedWatchProvider) Name() string {
	return "plain"
}

func (unsupportedWatchProvider) FetchReviews(context.Context, FetchRequest) ([]ReviewItem, error) {
	return nil, nil
}

func (unsupportedWatchProvider) ResolveIssues(context.Context, string, []ResolvedIssue) error {
	return nil
}

func TestFetchWatchStatusReturnsUnsupportedError(t *testing.T) {
	t.Parallel()

	status, err := FetchWatchStatus(context.Background(), unsupportedWatchProvider{}, WatchStatusRequest{PR: "259"})
	if err == nil {
		t.Fatal("expected unsupported watch-status error")
	}
	if !errors.Is(err, ErrWatchStatusUnsupported) {
		t.Fatalf("expected ErrWatchStatusUnsupported, got %v", err)
	}
	if status.State != WatchStatusUnsupported {
		t.Fatalf("status state = %q, want %q", status.State, WatchStatusUnsupported)
	}
}
