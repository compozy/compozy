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
	})
	if err != nil {
		return nil, mapForgeRuntimeError(err)
	}
	return &worktree.ForgeStatus{
		WorktreeID: request.WorktreeID, Provider: response.Provider, PRNumber: response.PRNumber,
		PRState: response.PRState, PRURL: response.PRURL, Merged: response.Merged,
		FetchedAt: &response.FetchedAt,
	}, nil
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
		RemoteURLs: append([]string(nil), request.RemoteURLs...), Head: request.Head, Base: request.Base,
		Title: request.Title, Body: request.Body, Draft: request.Draft,
	})
	if err != nil {
		return nil, mapForgeRuntimeError(err)
	}
	return &worktree.ForgePRResult{Status: response.Status, Number: response.Number, URL: response.URL}, nil
}

func mapForgeRuntimeError(err error) error {
	cause := extensionpkg.ForgeProviderErrorCause(err)
	if errors.Is(err, toolspkg.ErrToolUnavailable) {
		return worktree.NewForgeFailure(worktree.ErrForgeUnavailable, cause, err)
	}
	return worktree.NewForgeFailure(worktree.ErrForge, cause, err)
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
		return nil, worktree.ErrForgeUnavailable
	}
	runtime, ok := p.runtime().(extensionpkg.ForgeRuntime)
	if !ok || runtime == nil {
		return nil, worktree.ErrForgeUnavailable
	}
	return runtime, nil
}
