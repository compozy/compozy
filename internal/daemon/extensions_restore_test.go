package daemon

import (
	"errors"
	"sync"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// Invariant: restore clears a built-in's tombstone and enrolls it at its bundled version under the package
// lock, refuses conflicts without touching the tombstone, and writes the tombstone back when enrollment fails.
// Owner: daemon extension suite (UT-002, UT-006, UT-007, UT-008, UT-017, IT-008, and the not-found case).

func restoreTestService(t *testing.T) (*daemonExtensionService, *extensionpkg.Registry) {
	t.Helper()
	deps, registry, _, _ := newNativeExtensionToolDeps(t)
	service := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry: registry, Profiles: deps.ProfileManager, HomePaths: deps.HomePaths,
		Logger: discardLogger(),
	}, withDaemonExtensionMarketplace(deps.ExtensionConfig, deps.ExtensionSources)).(*daemonExtensionService)
	return service, registry
}

func restoreTestSeedRow(
	t *testing.T,
	registry *extensionpkg.Registry,
	name string,
	source extensionpkg.ExtensionSource,
) {
	t.Helper()
	dir := writeNativeLocalExtensionFixture(t, name, "1.0.0")
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Install(manifest, dir, checksum, extensionpkg.WithInstallSource(source)); err != nil {
		t.Fatalf("fixture %s install error = %v", source, err)
	}
}

// restoreTestRemovePermanently seeds a bundled built-in and removes it permanently, leaving its tombstone.
func restoreTestRemovePermanently(
	t *testing.T,
	service *daemonExtensionService,
	registry *extensionpkg.Registry,
	name string,
) {
	t.Helper()
	restoreTestSeedRow(t, registry, name, extensionpkg.SourceBundled)
	if _, err := service.RemovePermanently(t.Context(), name, restoreTestOperator(t)); err != nil {
		t.Fatalf("RemovePermanently(%q) error = %v", name, err)
	}
}

func restoreTestOperator(t *testing.T) taskpkg.ActorContext {
	t.Helper()
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "extension restore")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	return actor
}

func restoreTestRequireTombstone(t *testing.T, registry *extensionpkg.Registry, name string, want bool) {
	t.Helper()
	got, err := registry.Tombstoned(t.Context(), name)
	if err != nil || got != want {
		t.Fatalf("Tombstoned(%q) = %v, %v; want %v, nil", name, got, err, want)
	}
}

func TestDaemonExtensionBuiltinRestore(t *testing.T) {
	t.Run(
		"Should clear the tombstone and restore the built-in as bundled and enabled, then refuse a second restore",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")

			item, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if err != nil {
				t.Fatalf("RestoreBuiltin(tombstoned) error = %v", err)
			}
			if item.Name != "spec-cycle" || item.Status != "restored" ||
				item.Source != extensionpkg.SourceBundled.String() ||
				!item.Enabled ||
				item.Path == "" ||
				item.Version == "" {
				t.Fatalf("RestoreBuiltin payload = %#v, want restored bundled enabled with version and path", item)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", false)
			installed, err := registry.Get("spec-cycle")
			if err != nil || installed.Source != extensionpkg.SourceBundled || !installed.Enabled {
				t.Fatalf("Get(restored) = %#v, %v; want an enabled bundled row", installed, err)
			}

			_, err = service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionBuiltinNotRemoved) {
				t.Fatalf("second RestoreBuiltin error = %v, want ErrExtensionBuiltinNotRemoved", err)
			}
		},
	)

	t.Run(
		"Should report an unknown name as not found when it is neither tombstoned nor a built-in",
		func(t *testing.T) {
			t.Parallel()
			service, _ := restoreTestService(t)

			_, err := service.RestoreBuiltin(t.Context(), "missing-ext", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				t.Fatalf("RestoreBuiltin(unknown) error = %v, want ErrExtensionNotFound", err)
			}
		},
	)

	t.Run("Should refuse restore over a non-bundled row and keep the tombstone unchanged", func(t *testing.T) {
		t.Parallel()
		service, registry := restoreTestService(t)
		restoreTestRemovePermanently(t, service, registry, "spec-cycle")
		restoreTestSeedRow(t, registry, "spec-cycle", extensionpkg.SourceMarketplace)

		_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
		if !errors.Is(err, extensionpkg.ErrExtensionExists) {
			t.Fatalf("RestoreBuiltin(operator row) error = %v, want ErrExtensionExists", err)
		}
		restoreTestRequireTombstone(t, registry, "spec-cycle", true)
		installed, err := registry.Get("spec-cycle")
		if err != nil || installed.Source != extensionpkg.SourceMarketplace {
			t.Fatalf("Get(operator row) = %#v, %v; want the marketplace row unchanged", installed, err)
		}
	})

	t.Run(
		"Should write the tombstone back and report restore failed when enrollment fails after the clear",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(compozyconfig.HomePaths, *extensionpkg.Registry) error {
					return errors.New("injected enrollment failure")
				},
			}}

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
				t.Fatalf("RestoreBuiltin(enrollment failure) error = %v, want ErrExtensionRestoreFailed", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)
			if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				t.Fatalf("Get(after failed restore) error = %v, want ErrExtensionNotFound", err)
			}
		},
	)

	t.Run("Should let exactly one of two concurrent restores succeed", func(t *testing.T) {
		t.Parallel()
		service, registry := restoreTestService(t)
		restoreTestRemovePermanently(t, service, registry, "spec-cycle")
		actor := restoreTestOperator(t)

		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Go(func() {
				_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", actor)
				errs <- err
			})
		}
		wg.Wait()
		close(errs)

		restored, notRemoved := 0, 0
		for err := range errs {
			switch {
			case err == nil:
				restored++
			case errors.Is(err, extensionpkg.ErrExtensionBuiltinNotRemoved):
				notRemoved++
			default:
				t.Errorf("concurrent RestoreBuiltin error = %v, want nil or ErrExtensionBuiltinNotRemoved", err)
			}
		}
		if restored != 1 || notRemoved != 1 {
			t.Fatalf("concurrent restores: restored=%d notRemoved=%d, want 1 and 1", restored, notRemoved)
		}
	})

	t.Run("Should deny restore to an actor without write authority and keep the tombstone", func(t *testing.T) {
		t.Parallel()
		service, registry := restoreTestService(t)
		restoreTestRemovePermanently(t, service, registry, "spec-cycle")
		reader := restoreTestOperator(t)
		reader.Authority = taskpkg.Authority{Read: true}

		_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", reader)
		if !errors.Is(err, taskpkg.ErrPermissionDenied) {
			t.Fatalf("RestoreBuiltin(read-only) error = %v, want ErrPermissionDenied", err)
		}
		restoreTestRequireTombstone(t, registry, "spec-cycle", true)
	})

	t.Run(
		"Should refuse restore while an operator install exists, then restore after that install is removed",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			restoreTestSeedRow(t, registry, "spec-cycle", extensionpkg.SourceMarketplace)

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionExists) {
				t.Fatalf("RestoreBuiltin(operator install) error = %v, want ErrExtensionExists", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)

			if _, err := service.Remove(t.Context(), "spec-cycle", restoreTestOperator(t)); err != nil {
				t.Fatalf("Remove(operator install) error = %v", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)

			item, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if err != nil {
				t.Fatalf("RestoreBuiltin(after operator install removed) error = %v", err)
			}
			if item.Source != extensionpkg.SourceBundled.String() || !item.Enabled {
				t.Fatalf("RestoreBuiltin payload = %#v, want a bundled enabled row", item)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", false)
		},
	)
}
