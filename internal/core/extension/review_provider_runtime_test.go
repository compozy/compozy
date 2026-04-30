package extensions

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestManagerResolveReviewProviderBridgeCachesNormalizedNames(t *testing.T) {
	binary := buildMockExtensionBinary(t)
	entry := DeclaredProvider{
		Extension:    Ref{Name: "mock-ext", Source: SourceWorkspace},
		ManifestPath: filepath.Join(filepath.Dir(binary), "extension.toml"),
		ExtensionDir: filepath.Dir(binary),
		Manifest: &Manifest{
			Extension: ExtensionInfo{
				Name:              "mock-ext",
				Version:           "1.0.0",
				MinCompozyVersion: "0.0.0",
			},
			Subprocess: &SubprocessConfig{Command: binary},
			Security:   SecurityConfig{Capabilities: []Capability{CapabilityProvidersRegister}},
		},
		ProviderEntry: ProviderEntry{Name: "Demo", Kind: ProviderKindExtension},
	}

	manager := &Manager{
		workspaceRoot:   t.TempDir(),
		invokingCommand: invokingCommandFixReviews,
		reviewProviders: map[string]DeclaredProvider{reviewProviderKey("Demo"): entry},
		reviewBridges:   make(map[string]*ReviewProviderBridge),
	}

	first, ok := manager.ResolveReviewProviderBridge(" DEMO ")
	if !ok || first == nil {
		t.Fatalf("ResolveReviewProviderBridge(first) = (%v, %v), want non-nil true", first, ok)
	}
	second, ok := manager.ResolveReviewProviderBridge("demo")
	if !ok || second == nil {
		t.Fatalf("ResolveReviewProviderBridge(second) = (%v, %v), want non-nil true", second, ok)
	}
	if first != second {
		t.Fatal("ResolveReviewProviderBridge() did not return cached bridge for normalized provider name")
	}
	if _, ok := manager.reviewBridges["demo"]; !ok {
		t.Fatal(`manager.reviewBridges["demo"] missing after bridge resolution`)
	}
	if got := reviewProviderKey(" Demo "); got != "demo" {
		t.Fatalf("reviewProviderKey() = %q, want %q", got, "demo")
	}
}

func TestManagerCloseReviewProviderBridgesShutsDownCachedManagers(t *testing.T) {
	closed := false
	bridgeManager := &Manager{
		shutdownHook: func(context.Context) error {
			closed = true
			return nil
		},
	}

	manager := &Manager{
		reviewBridges: map[string]*ReviewProviderBridge{
			"demo": {manager: bridgeManager},
		},
	}

	if err := manager.closeReviewProviderBridges(); err != nil {
		t.Fatalf("closeReviewProviderBridges() error = %v", err)
	}
	if !closed {
		t.Fatal("expected cached review provider manager to shut down")
	}
	if len(manager.reviewBridges) != 0 {
		t.Fatalf("reviewBridges length = %d, want 0", len(manager.reviewBridges))
	}
}

func TestManagerCloseReviewProviderBridgesAggregatesShutdownErrors(t *testing.T) {
	expectedErr := errors.New("cleanup failed")
	manager := &Manager{
		reviewBridges: map[string]*ReviewProviderBridge{
			"demo": {
				manager: &Manager{
					shutdownHook: func(context.Context) error {
						return expectedErr
					},
				},
			},
		},
	}

	err := manager.closeReviewProviderBridges()
	if !errors.Is(err, expectedErr) {
		t.Fatalf("closeReviewProviderBridges() error = %v, want %v", err, expectedErr)
	}
}
