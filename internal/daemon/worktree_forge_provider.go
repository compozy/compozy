package daemon

import (
	"context"
	"errors"
	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/worktree"
)

type daemonExtensionForgeProvider struct {
	runtime func() extensionRuntime
}

var _ worktree.ForgeProvider = daemonExtensionForgeProvider{}

func (p daemonExtensionForgeProvider) Capabilities(
	ctx context.Context,
	remoteURLs []string,
) (*worktree.ForgeCapabilities, error) {
	runtime, err := p.resolve()
	if err != nil {
		return nil, err
	}
	response, err := runtime.ForgeCapabilities(ctx, remoteURLs)
	if err != nil {
		return nil, mapForgeRuntimeError(err)
	}
	if !response.Served || !response.Available {
		cause := strings.TrimSpace(response.Cause)
		if cause == "" {
			cause = extensioncontract.ForgeCauseUnsupportedRemote
		}
		return nil, worktree.NewForgeFailure(forgeFailureKind(cause), cause, nil)
	}
	return &worktree.ForgeCapabilities{
		Provider: response.Provider, Winner: response.Winner,
		RequestNoun: response.RequestNoun, OpenActionLabel: response.OpenActionLabel,
		ViewActionLabel: response.ViewActionLabel, SupportsDraft: response.SupportsDraft,
		CompareURLTemplate: response.CompareURLTemplate, CredentialSource: response.CredentialSource,
		DefaultBranch: response.DefaultBranch, TemplatePaths: append([]string(nil), response.TemplatePaths...),
	}, nil
}

func (p daemonExtensionForgeProvider) Status(
	ctx context.Context,
	request worktree.ForgeStatusRequest,
) (*worktree.ForgeStatus, error) {
	runtime, err := p.resolve()
	if err != nil {
		return nil, err
	}
	response, err := runtime.ForgeStatus(ctx, extensioncontract.ForgeStatusRequest{
		RemoteURLs: append([]string(nil), request.RemoteURLs...), Branch: request.Branch,
		Base: request.Base, HeadSHA: request.HeadSHA,
	})
	if err != nil {
		return nil, mapForgeRuntimeError(err)
	}
	var checks *worktree.ForgeChecks
	if response.Checks != nil {
		checks = &worktree.ForgeChecks{
			State:       response.Checks.State,
			Passed:      response.Checks.Passed,
			Failed:      response.Checks.Failed,
			Pending:     response.Checks.Pending,
			FailedNames: response.Checks.FailedNames,
		}
	}
	return &worktree.ForgeStatus{
		Checks:             checks,
		Reviews:            forgeReviews(response.Reviews),
		ReviewsUnavailable: response.ReviewsUnavailable,
		ReviewDecision:     response.ReviewDecision,
		Mergeable:          response.Mergeable,
		UpdatedAt:          response.UpdatedAt,
		Selection:          response.Selection,
		WorktreeID:         request.WorktreeID,
		Provider:           response.Provider,
		PRNumber:           response.PRNumber,
		PRState:            response.PRState,
		PRURL:              response.PRURL,
		Title:              response.Title,
		Merged:             response.Merged,
		Head:               response.Head,
		Base:               response.Base,
		HeadSHA:            response.HeadSHA,
		Draft:              response.Draft,
		FetchedAt:          &response.FetchedAt,
	}, nil
}

func forgeReviews(reviews *extensioncontract.ForgeReviews) *worktree.ForgeReviews {
	if reviews == nil {
		return nil
	}
	items := make([]worktree.ForgeReview, 0, len(reviews.Items))
	for i := range reviews.Items {
		item := &reviews.Items[i]
		items = append(items, worktree.ForgeReview{
			ID: item.ID, Author: item.Author, IsBot: item.IsBot, State: item.State,
			SubmittedAt: item.SubmittedAt, Commit: item.Commit,
		})
	}
	threads := make([]worktree.ForgeReviewThread, 0, len(reviews.Threads))
	for _, thread := range reviews.Threads {
		threads = append(threads, worktree.ForgeReviewThread{ID: thread.ID, Commit: thread.Commit})
	}
	return &worktree.ForgeReviews{
		Items: items, Threads: threads, UnresolvedThreads: reviews.UnresolvedThreads, Truncated: reviews.Truncated,
	}
}

func (p daemonExtensionForgeProvider) CreatePR(
	ctx context.Context,
	request worktree.ForgePRRequest,
) (*worktree.ForgePRResult, error) {
	runtime, err := p.resolve()
	if err != nil {
		return nil, err
	}
	response, err := runtime.ForgeCreatePR(ctx, extensioncontract.ForgePRCreateRequest{
		RemoteURLs: append(
			[]string(nil),
			request.RemoteURLs...),
		Head:    request.Head,
		Base:    request.Base,
		HeadSHA: request.HeadSHA,
		Title:   request.Title,
		Body:    request.Body,
		Draft:   request.Draft,
	})
	if err != nil {
		return nil, mapForgeRuntimeError(err)
	}
	return &worktree.ForgePRResult{Status: response.Status, Number: response.Number, URL: response.URL}, nil
}

func mapForgeRuntimeError(err error) error {
	cause := extensionpkg.ForgeProviderErrorCause(err)
	kind := worktree.ErrForge
	if errors.Is(err, toolspkg.ErrToolUnavailable) {
		kind = worktree.ErrForgeUnavailable
	}
	failure := worktree.NewForgeFailure(kind, cause, err)
	if source, ok := errors.AsType[*extensionpkg.ForgeProviderError](err); ok {
		if target, ok := errors.AsType[*worktree.ForgeFailure](failure); ok {
			target.RetryAfter = source.RetryAfter
		}
	}
	return failure
}

func forgeFailureKind(cause string) error {
	if cause == extensioncontract.ForgeCauseRateLimited ||
		cause == extensioncontract.ForgeCauseCredentialExpired {
		return worktree.ErrForge
	}
	return worktree.ErrForgeUnavailable
}

func (p daemonExtensionForgeProvider) resolve() (extensionpkg.ForgeRuntime, error) {
	if p.runtime == nil {
		return nil, worktree.NewForgeFailure(worktree.ErrForgeUnavailable, "provider_disabled", nil)
	}
	runtime, ok := p.runtime().(extensionpkg.ForgeRuntime)
	if !ok || runtime == nil {
		return nil, worktree.NewForgeFailure(worktree.ErrForgeUnavailable, "provider_disabled", nil)
	}
	return runtime, nil
}
