package skills

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const registryConcurrentReadWriteTimeout = 30 * time.Second

func TestRegistryConcurrentReadAPIsAndSetEnabledContract(t *testing.T) {
	t.Parallel()

	t.Run("Should avoid races between read API clones and SetEnabled writes", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		registry, workspace := newRegistryReadWriteRaceFixtureContract(t)
		if _, err := registry.ForWorkspace(ctx, workspace); err != nil {
			t.Fatalf("ForWorkspace() seed error = %v", err)
		}

		start := make(chan struct{})
		done := make(chan struct{})
		errCh := make(chan error, 1)
		reportErr := func(err error) {
			select {
			case errCh <- err:
				cancel()
			default:
			}
		}

		var wg sync.WaitGroup
		for worker := range 8 {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				<-start
				for iteration := range 1_000 {
					select {
					case <-ctx.Done():
						return
					default:
					}

					name := "global-skill"
					if worker%2 == 1 {
						name = "workspace-skill"
					}
					if name == "global-skill" {
						if _, ok := registry.Get(name); !ok {
							reportErr(fmt.Errorf("Get(%q) ok = false at iteration %d", name, iteration))
							return
						}
					}
					if len(registry.List()) == 0 {
						reportErr(fmt.Errorf("List() returned no skills at iteration %d", iteration))
						return
					}
					if _, err := registry.ForWorkspace(ctx, workspace); err != nil {
						reportErr(fmt.Errorf("ForWorkspace() iteration %d: %w", iteration, err))
						return
					}
				}
			}(worker)
		}

		wg.Go(func() {
			<-start
			for iteration := range 1_000 {
				select {
				case <-ctx.Done():
					return
				default:
				}

				enabled := iteration%2 == 0
				if err := registry.SetEnabled("global-skill", nil, enabled); err != nil {
					reportErr(fmt.Errorf("SetEnabled(global-skill) iteration %d: %w", iteration, err))
					return
				}
				if err := registry.SetEnabled("workspace-skill", workspace, enabled); err != nil {
					reportErr(fmt.Errorf("SetEnabled(workspace-skill) iteration %d: %w", iteration, err))
					return
				}
			}
		})

		go func() {
			wg.Wait()
			close(done)
		}()
		close(start)

		select {
		case err := <-errCh:
			cancel()
			<-done
			t.Fatal(err)
		case <-done:
		case <-time.After(registryConcurrentReadWriteTimeout):
			cancel()
			<-done
			t.Fatal("concurrent registry read/write operations timed out")
		}
	})
}

func newRegistryReadWriteRaceFixtureContract(t *testing.T) (*Registry, *workspacepkg.ResolvedWorkspace) {
	t.Helper()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	writeSkillFile(
		t,
		userDir,
		filepath.Join("global-skill", skillFileName),
		skillWithDescription("global-skill", "Global race skill"),
	)

	workspaceRoot := filepath.Join(root, "workspace")
	workspaceSkillsRoot := filepath.Join(workspaceRoot, ".agh", "skills")
	workspaceSkillDir := filepath.Join(workspaceSkillsRoot, "workspace-skill")
	writeSkillFile(
		t,
		workspaceSkillsRoot,
		filepath.Join("workspace-skill", skillFileName),
		skillWithDescription("workspace-skill", "Workspace race skill"),
	)

	registry := newTestRegistry(t, RegistryConfig{
		UserSkillsDir: userDir,
	})
	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	workspace := resolvedWorkspacePtr(
		"race-workspace",
		workspaceRoot,
		resolvedSkillPath(workspaceSkillDir, "workspace"),
	)
	return registry, workspace
}
