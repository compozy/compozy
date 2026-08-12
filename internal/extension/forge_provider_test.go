package extensionpkg

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Canonical suite: forge.provider selection, dispatch, and response validation.
func TestManagerForgeProvider(t *testing.T) {
	t.Parallel()

	t.Run("Should select the first enabled provider that serves the remote", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		calls := make([]string, 0, 8)
		record := func(value string) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, value)
		}

		unsupported := newFakeProcess(5101)
		unsupported.callFn = func(_ context.Context, method string, params, result any) error {
			record("unsupported:" + method)
			if method != string(extensionprotocol.ExtensionServiceMethodForgeCapabilities) {
				return errors.New("unexpected method on unsupported provider")
			}
			request := params.(extensioncontract.ForgeCapabilitiesRequest)
			if !slices.Equal(request.RemoteURLs, []string{"https://github.com/acme/repo.git"}) {
				return errors.New("unexpected normalized remotes")
			}
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = extensioncontract.ForgeCapabilitiesResponse{
				Cause: extensioncontract.ForgeCauseUnsupportedRemote,
			}
			return nil
		}
		github := newFakeProcess(5102)
		github.callFn = func(_ context.Context, method string, params, result any) error {
			record("github:" + method)
			switch method {
			case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
			case string(extensionprotocol.ExtensionServiceMethodForgeStatus):
				request := params.(extensioncontract.ForgeStatusRequest)
				if request.Branch != "feature/exit" {
					return errors.New("unexpected status branch")
				}
				state, merged, number := "open", false, 7
				*result.(*extensioncontract.ForgeStatusResponse) = extensioncontract.ForgeStatusResponse{
					PRNumber: &number, PRState: &state, PRURL: "https://github.com/acme/repo/pull/7", Merged: &merged,
				}
			case string(extensionprotocol.ExtensionServiceMethodForgePRCreate):
				request := params.(extensioncontract.ForgePRCreateRequest)
				if request.Head != "feature/exit" || request.Base != "main" || request.Title != "Exit worktree" {
					return errors.New("unexpected create request")
				}
				*result.(*extensioncontract.ForgePRCreateResponse) = extensioncontract.ForgePRCreateResponse{
					Status: "created", Number: 8, URL: "https://github.com/acme/repo/pull/8",
				}
			default:
				return errors.New("unexpected forge method")
			}
			return nil
		}

		manager := NewManager(nil)
		installForgeTestProvider(manager, "first-enabled", unsupported, time.Unix(1, 0))
		installForgeTestProvider(manager, "github-provider", github, time.Unix(2, 0))
		remotes := []string{" https://github.com/acme/repo.git ", "https://github.com/acme/repo.git"}

		capabilities, err := manager.ForgeCapabilities(context.Background(), remotes)
		if err != nil || capabilities.Winner != "github-provider" || !capabilities.Available {
			t.Fatalf("ForgeCapabilities() = %#v, %v", capabilities, err)
		}
		status, err := manager.ForgeStatus(context.Background(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: remotes, Branch: "feature/exit",
		})
		if err != nil || status.Provider != "github-provider" || status.PRNumber == nil || *status.PRNumber != 7 {
			t.Fatalf("ForgeStatus() = %#v, %v", status, err)
		}
		created, err := manager.ForgeCreatePR(context.Background(), extensioncontract.ForgePRCreateRequest{
			RemoteURLs: remotes, Head: "feature/exit", Base: "main", Title: "Exit worktree",
		})
		if err != nil || created.Status != "created" || created.Number != 8 {
			t.Fatalf("ForgeCreatePR() = %#v, %v", created, err)
		}
		mu.Lock()
		defer mu.Unlock()
		want := []string{
			"unsupported:forge/capabilities", "github:forge/capabilities",
			"unsupported:forge/capabilities", "github:forge/capabilities", "github:forge/status",
			"unsupported:forge/capabilities", "github:forge/capabilities", "github:forge/pr_create",
		}
		if !slices.Equal(calls, want) {
			t.Fatalf("forge calls = %#v, want %#v", calls, want)
		}
	})

	t.Run("Should reject an incomplete served capability response", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5103)
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = extensioncontract.ForgeCapabilitiesResponse{
				Served: true, Available: true, Provider: "broken", CredentialSource: "binding",
			}
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "broken", process, time.Unix(1, 0))
		_, err := manager.ForgeCapabilities(context.Background(), []string{"https://github.com/acme/repo"})
		if err == nil {
			t.Fatal("ForgeCapabilities() error = nil, want incomplete response failure")
		}
	})

	t.Run("Should reject an incomplete successful pull request", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5106)
		process.callFn = func(_ context.Context, method string, _ any, result any) error {
			switch method {
			case string(extensionprotocol.ExtensionServiceMethodForgeCapabilities):
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = availableForgeCapabilities()
			case string(extensionprotocol.ExtensionServiceMethodForgePRCreate):
				*result.(*extensioncontract.ForgePRCreateResponse) = extensioncontract.ForgePRCreateResponse{
					Status: "created",
				}
			}
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "incomplete-pr", process, time.Unix(1, 0))
		_, err := manager.ForgeCreatePR(context.Background(), extensioncontract.ForgePRCreateRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Head: "feature", Base: "main", Title: "Feature",
		})
		if err == nil {
			t.Fatal("ForgeCreatePR(incomplete) error = nil")
		}
	})

	t.Run("Should map an unavailable winner to the stable tool error", func(t *testing.T) {
		t.Parallel()
		process := newFakeProcess(5104)
		process.callFn = func(_ context.Context, _ string, _ any, result any) error {
			response := availableForgeCapabilities()
			response.Available = false
			response.CredentialSource = ""
			response.Cause = extensioncontract.ForgeCauseCredentialAbsent
			*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
			return nil
		}
		manager := NewManager(nil)
		installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
		_, err := manager.ForgeStatus(context.Background(), extensioncontract.ForgeStatusRequest{
			RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
		})
		if !errors.Is(err, toolspkg.ErrToolUnavailable) {
			t.Fatalf("ForgeStatus() error = %v, want ErrToolUnavailable", err)
		}
	})

	for _, cause := range []string{
		extensioncontract.ForgeCauseRateLimited,
		extensioncontract.ForgeCauseCredentialExpired,
	} {
		cause := cause
		t.Run("Should preserve transient cause "+cause+" as a forge failure", func(t *testing.T) {
			t.Parallel()
			process := newFakeProcess(5105)
			process.callFn = func(_ context.Context, _ string, _ any, result any) error {
				response := availableForgeCapabilities()
				response.Available = false
				response.CredentialSource = ""
				response.Cause = cause
				*result.(*extensioncontract.ForgeCapabilitiesResponse) = response
				return nil
			}
			manager := NewManager(nil)
			installForgeTestProvider(manager, "github-provider", process, time.Unix(1, 0))
			_, err := manager.ForgeStatus(context.Background(), extensioncontract.ForgeStatusRequest{
				RemoteURLs: []string{"https://github.com/acme/repo"}, Branch: "feature/exit",
			})
			if err == nil || errors.Is(err, toolspkg.ErrToolUnavailable) || ForgeProviderErrorCause(err) != cause {
				t.Fatalf("ForgeStatus() error = %v, want transient cause %q", err, cause)
			}
		})
	}
}

func installForgeTestProvider(manager *Manager, name string, process *fakeProcess, startedAt time.Time) {
	provides := []string{extensionprotocol.CapabilityProvideForgeProvider}
	manager.extensions[name] = &managedExtension{
		key: GlobalInstanceKey(name), info: ExtensionInfo{Name: name, Capabilities: CapabilitiesConfig{Provides: provides}},
		manifest: &Manifest{Name: name, Capabilities: CapabilitiesConfig{Provides: provides}},
		process:  process, active: true, lastStartedAt: startedAt,
		initialize: &subprocess.InitializeResponse{
			ImplementedMethods: extensionprotocol.CapabilityServiceMethods(provides),
		},
	}
}

func availableForgeCapabilities() extensioncontract.ForgeCapabilitiesResponse {
	return extensioncontract.ForgeCapabilitiesResponse{
		Served: true, Available: true, Provider: "github", ServedRemote: "https://github.com/acme/repo",
		RequestNoun: "PR", OpenActionLabel: "Open PR", ViewActionLabel: "View PR",
		CredentialSource: "binding", DefaultBranch: "main",
	}
}
