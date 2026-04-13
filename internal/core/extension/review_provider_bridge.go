package extensions

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/provider"
)

type fetchReviewsRequest struct {
	Provider string `json:"provider"`
	provider.FetchRequest
}

type resolveIssuesRequest struct {
	Provider string                   `json:"provider"`
	PR       string                   `json:"pr"`
	Issues   []provider.ResolvedIssue `json:"issues,omitempty"`
}

// ReviewProviderBridge lazily reuses an active extension session when one is
// already running for the workspace, or starts a standalone single-extension
// manager the first time a review provider is invoked.
type ReviewProviderBridge struct {
	prototype       *RuntimeExtension
	workspaceRoot   string
	invokingCommand string

	mu      sync.Mutex
	manager *Manager
}

var _ provider.ExtensionBridge = (*ReviewProviderBridge)(nil)

// NewReviewProviderBridge constructs a lazy bridge for one extension-backed
// review provider declaration.
func NewReviewProviderBridge(
	entry DeclaredProvider,
	workspaceRoot string,
	invokingCommand string,
) (*ReviewProviderBridge, error) {
	if entry.Manifest == nil || entry.Manifest.Subprocess == nil {
		return nil, fmt.Errorf(
			"build review provider bridge for %q: missing extension subprocess config",
			entry.Name,
		)
	}

	prototype, err := runtimeExtensionFromDeclaredProvider(entry)
	if err != nil {
		return nil, err
	}

	return &ReviewProviderBridge{
		prototype:       prototype,
		workspaceRoot:   strings.TrimSpace(workspaceRoot),
		invokingCommand: strings.TrimSpace(invokingCommand),
	}, nil
}

func (b *ReviewProviderBridge) FetchReviews(
	ctx context.Context,
	providerName string,
	req provider.FetchRequest,
) ([]provider.ReviewItem, error) {
	session, err := b.session(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch reviews with extension provider %q: %w", providerName, err)
	}

	var items []provider.ReviewItem
	if err := session.Call(ctx, "fetch_reviews", fetchReviewsRequest{
		Provider:     strings.TrimSpace(providerName),
		FetchRequest: req,
	}, &items); err != nil {
		return nil, fmt.Errorf("call fetch_reviews for provider %q: %w", providerName, err)
	}
	return items, nil
}

func (b *ReviewProviderBridge) ResolveIssues(
	ctx context.Context,
	providerName string,
	pr string,
	issues []provider.ResolvedIssue,
) error {
	session, err := b.session(ctx)
	if err != nil {
		return fmt.Errorf("resolve issues with extension provider %q: %w", providerName, err)
	}

	if err := session.Call(ctx, "resolve_issues", resolveIssuesRequest{
		Provider: strings.TrimSpace(providerName),
		PR:       strings.TrimSpace(pr),
		Issues:   issues,
	}, nil); err != nil {
		return fmt.Errorf("call resolve_issues for provider %q: %w", providerName, err)
	}
	return nil
}

func (b *ReviewProviderBridge) Close() error {
	if b == nil {
		return nil
	}

	b.mu.Lock()
	manager := b.manager
	b.manager = nil
	b.mu.Unlock()

	if manager == nil {
		return nil
	}
	return manager.Shutdown(context.Background())
}

func (b *ReviewProviderBridge) session(ctx context.Context) (*extensionSession, error) {
	if b == nil || b.prototype == nil {
		return nil, fmt.Errorf("missing review provider bridge")
	}

	name := b.prototype.normalizedName()
	if session := lookupActiveExtensionSession(b.workspaceRoot, name); session != nil {
		return session, nil
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if session := b.cachedSession(name); session != nil {
		return session, nil
	}

	manager, session, err := b.startStandaloneManager(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := b.replaceManager(name, manager); err != nil {
		return nil, err
	}
	b.manager = manager
	return session, nil
}

func (b *ReviewProviderBridge) cachedSession(name string) *extensionSession {
	if b == nil || b.manager == nil {
		return nil
	}
	session, ok := b.manager.sessionForExtension(name)
	if !ok || session == nil || session.closedErr() != nil {
		return nil
	}
	return session
}

func (b *ReviewProviderBridge) startStandaloneManager(
	ctx context.Context,
	name string,
) (*Manager, *extensionSession, error) {
	cloned, err := cloneRuntimeExtension(b.prototype)
	if err != nil {
		return nil, nil, err
	}
	manager, err := newManagerForExtensions(managerConfig{
		WorkspaceRoot:   b.workspaceRoot,
		InvokingCommand: b.invokingCommand,
	}, []*RuntimeExtension{cloned})
	if err != nil {
		return nil, nil, err
	}
	if err := manager.Start(ctx); err != nil {
		return nil, nil, err
	}

	session, ok := manager.sessionForExtension(name)
	if ok && session != nil {
		return manager, session, nil
	}

	shutdownErr := manager.Shutdown(context.Background())
	return nil, nil, missingRegisteredSessionError(name, shutdownErr)
}

func (b *ReviewProviderBridge) replaceManager(name string, next *Manager) error {
	if b == nil || b.manager == nil {
		return nil
	}
	if err := b.manager.Shutdown(context.Background()); err != nil {
		cleanupErr := next.Shutdown(context.Background())
		if cleanupErr != nil {
			return fmt.Errorf(
				"replace review provider extension %q manager: shutdown previous manager: %v; shutdown new manager: %w",
				name,
				err,
				cleanupErr,
			)
		}
		return fmt.Errorf(
			"replace review provider extension %q manager: shutdown previous manager: %w",
			name,
			err,
		)
	}
	return nil
}

func missingRegisteredSessionError(name string, shutdownErr error) error {
	err := fmt.Errorf("start review provider extension %q: session was not registered", name)
	if shutdownErr == nil {
		return err
	}
	return fmt.Errorf("%w; shutdown during cleanup: %v", err, shutdownErr)
}
