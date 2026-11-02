package github

import (
	"context"
	"errors"
	"fmt"
	"io"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	compozysdk "github.com/compozy/compozy/sdk/go"
)

// RunProvider serves the bundled provider over the public extension SDK protocol.
func RunProvider(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if ctx == nil {
		return errors.New("github forge: context is required")
	}
	if stdin == nil || stdout == nil || stderr == nil {
		return errors.New("github forge: stdio is required")
	}
	provider := newProvider()
	extension := compozysdk.NewExtension(
		providerDefinition(),
		compozysdk.WithStdio(stdin, stdout),
		compozysdk.WithStderr(stderr),
	)
	if err := compozysdk.ForgeProvider(extension, forgeProviderHandlers(provider)); err != nil {
		return fmt.Errorf("github forge: register provider: %w", err)
	}
	if err := extension.Run(ctx); err != nil {
		return fmt.Errorf("github forge: run extension protocol: %w", err)
	}
	return nil
}

func forgeProviderHandlers(provider *Provider) compozysdk.ForgeProviderHandlers {
	return compozysdk.ForgeProviderHandlers{
		SearchPRs: func(ctx context.Context, _ compozysdk.ExtensionContext, req compozysdk.ForgePRSearchRequest) (compozysdk.ForgePRSearchResponse, error) {
			response, err := provider.SearchPRs(ctx, extensioncontract.ForgePRSearchRequest(req))
			result := compozysdk.ForgePRSearchResponse{HasMore: response.HasMore, Items: make([]compozysdk.ForgePRCandidate, 0, len(response.Items))}
			for _, item := range response.Items {
				result.Items = append(result.Items, compozysdk.ForgePRCandidate(item))
			}
			return result, err
		},
		ReadPR: func(ctx context.Context, _ compozysdk.ExtensionContext, req compozysdk.ForgePRReadRequest) (compozysdk.ForgePRCandidate, error) {
			response, err := provider.ReadPR(ctx, extensioncontract.ForgePRReadRequest(req))
			return compozysdk.ForgePRCandidate(response), err
		},
		Capabilities: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgeCapabilitiesRequest,
		) (compozysdk.ForgeCapabilitiesResponse, error) {
			response, err := provider.Capabilities(ctx, extensioncontract.ForgeCapabilitiesRequest(request))
			return compozysdk.ForgeCapabilitiesResponse(response), err
		},
		Status: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgeStatusRequest,
		) (compozysdk.ForgeStatusResponse, error) {
			response, err := provider.Status(ctx, extensioncontract.ForgeStatusRequest(request))
			return forgeStatusSDKResponse(response), err
		},
		CreatePR: func(
			ctx context.Context,
			_ compozysdk.ExtensionContext,
			request compozysdk.ForgePRCreateRequest,
		) (compozysdk.ForgePRCreateResponse, error) {
			response, err := provider.CreatePR(ctx, extensioncontract.ForgePRCreateRequest(request))
			return compozysdk.ForgePRCreateResponse(response), err
		},
	}
}

func providerDefinition() compozysdk.ExtensionDefinition {
	return compozysdk.ExtensionDefinition{
		Name:        Name,
		Version:     "0.1.0",
		Description: "First-party GitHub pull request provider for worktree exit actions.",
		RequiresEnv: []string{"GITHUB_TOKEN"},
		Capabilities: compozysdk.CapabilitiesConfig{
			Provides: []string{compozysdk.CapabilityProvideForgeProvider},
		},
		Subprocess: compozysdk.DescribeSubprocess{
			Command: "{{compozy_executable}}",
			Args:    []string{"__internal", "extension-provider", Name},
		},
	}
}

func forgeStatusSDKResponse(response extensioncontract.ForgeStatusResponse) compozysdk.ForgeStatusResponse {
	result := compozysdk.ForgeStatusResponse{
		Title: response.Title, Provider: response.Provider, PRNumber: response.PRNumber, PRState: response.PRState,
		PRURL: response.PRURL, Head: response.Head, Base: response.Base, HeadSHA: response.HeadSHA,
		Draft: response.Draft, Merged: response.Merged, FetchedAt: response.FetchedAt, Cause: response.Cause,
		ReviewDecision: response.ReviewDecision, Mergeable: response.Mergeable,
		UpdatedAt: response.UpdatedAt, Selection: response.Selection, RetryAfter: response.RetryAfter,
		ReviewsUnavailable: response.ReviewsUnavailable,
	}
	if response.Checks != nil {
		result.Checks = (*compozysdk.ForgeChecks)(response.Checks)
	}
	if response.Reviews != nil {
		items := make([]compozysdk.ForgeReview, 0, len(response.Reviews.Items))
		for i := range response.Reviews.Items {
			items = append(items, compozysdk.ForgeReview(response.Reviews.Items[i]))
		}
		threads := make([]compozysdk.ForgeReviewThread, 0, len(response.Reviews.Threads))
		for i := range response.Reviews.Threads {
			threads = append(threads, compozysdk.ForgeReviewThread(response.Reviews.Threads[i]))
		}
		result.Reviews = &compozysdk.ForgeReviews{
			Items: items, Threads: threads, UnresolvedThreads: response.Reviews.UnresolvedThreads,
			Truncated: response.Reviews.Truncated,
		}
	}
	return result
}
