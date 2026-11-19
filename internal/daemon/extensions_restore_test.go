package daemon

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// Invariant: restore clears a built-in's tombstone and enrolls it at its bundled version under the package
// lock, enables it for the default profile, refuses conflicts without touching the tombstone, and writes the
// tombstone back when enrollment, enablement, or the rollback's reload fails. The tombstone is written back only
// once a read-back confirms no row remains, even when the partial row's removal reports a failure.
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

// restoreTestRuntime is the extension runtime restore needs once it enables a row: enabling inspects the package
// resources, and the reload is routed through reload so a test can control how long it takes.
type restoreTestRuntime struct {
	registry *extensionpkg.Registry
	reload   func(context.Context) error
}

func (r restoreTestRuntime) Start(context.Context) error { return nil }

func (r restoreTestRuntime) Stop(context.Context) error { return nil }

func (r restoreTestRuntime) Reload(ctx context.Context) error {
	if r.reload != nil {
		return r.reload(ctx)
	}
	return nil
}

func (r restoreTestRuntime) Get(string) (*extensionpkg.Extension, error) {
	return nil, extensionpkg.ErrExtensionNotFound
}

func (r restoreTestRuntime) InspectPackageResources(_ context.Context, name string) (*extensionpkg.Extension, error) {
	info, err := r.registry.Get(name)
	if err != nil {
		return nil, err
	}
	manifest, err := extensionpkg.LoadInstalledManifest(info)
	if err != nil {
		return nil, err
	}
	return &extensionpkg.Extension{Info: *info, Manifest: manifest}, nil
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

	t.Run("Should write the tombstone back when the caller cancels during enrollment", func(t *testing.T) {
		t.Parallel()
		service, registry := restoreTestService(t)
		restoreTestRemovePermanently(t, service, registry, "spec-cycle")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		service.builtinEnrollments = []bundledEnrollment{{
			Name: "spec-cycle",
			Enroll: func(compozyconfig.HomePaths, *extensionpkg.Registry) error {
				// The caller goes away after the tombstone clear; the rollback must still record the tombstone.
				cancel()
				return errors.New("injected enrollment failure")
			},
		}}

		_, err := service.RestoreBuiltin(ctx, "spec-cycle", restoreTestOperator(t))
		if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
			t.Fatalf("RestoreBuiltin(canceled enrollment) error = %v, want ErrExtensionRestoreFailed", err)
		}
		restoreTestRequireTombstone(t, registry, "spec-cycle", true)
		if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("Get(after canceled restore) error = %v, want ErrExtensionNotFound", err)
		}
	})

	t.Run(
		"Should write the tombstone back when the partial row enrolled before the failure is removed",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					return errors.New("injected enrollment failure")
				},
			}}

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
				t.Fatalf("RestoreBuiltin(partial row) error = %v, want ErrExtensionRestoreFailed", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)
			if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				t.Fatalf("Get(after rollback) error = %v, want ErrExtensionNotFound", err)
			}
		},
	)

	t.Run(
		"Should keep the partial row and leave the tombstone clear when that row cannot be removed",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					// A plain file where the partial row's install directory belongs makes its removal fail.
					installed, err := reg.Get("spec-cycle")
					if err != nil {
						t.Fatalf("Get(partial row) error = %v", err)
					}
					dir := extensionpkg.PackageRootFromManifest(installed.ManifestPath)
					if err := os.RemoveAll(dir); err != nil {
						t.Fatalf("os.RemoveAll(%q) error = %v", dir, err)
					}
					if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
						t.Fatalf("os.WriteFile(%q) error = %v", dir, err)
					}
					return errors.New("injected enrollment failure")
				},
			}}

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
				t.Fatalf("RestoreBuiltin(unremovable partial row) error = %v, want ErrExtensionRestoreFailed", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", false)
			installed, err := registry.Get("spec-cycle")
			if err != nil || installed.Source != extensionpkg.SourceBundled {
				t.Fatalf(
					"Get(partial row) = %#v, %v; want the bundled row kept, never a row with a tombstone",
					installed,
					err,
				)
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

	t.Run(
		"Should enable a restored built-in that enrolled with a disabled default-profile exception",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.runtime = restoreTestRuntime{registry: registry}
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					// A dev-linked name keeps its disabled exception when its row is deleted, so enrollment can find one.
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					return reg.Disable("spec-cycle")
				},
			}}

			item, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if err != nil {
				t.Fatalf("RestoreBuiltin(disabled enrollment) error = %v", err)
			}
			if !item.Enabled {
				t.Fatalf("RestoreBuiltin payload = %#v, want enabled", item)
			}
			enabled, err := registry.IsEnabledForProfile("spec-cycle", store.DefaultProfileID)
			if err != nil || !enabled {
				t.Fatalf("IsEnabledForProfile(restored, default) = %v, %v; want enabled, nil", enabled, err)
			}
			installed, err := registry.Get("spec-cycle")
			if err != nil || !installed.Enabled {
				t.Fatalf("Get(restored) = %#v, %v; want an enabled row", installed, err)
			}
		},
	)

	t.Run("Should write the tombstone back when a reload spends the rollback budget", func(t *testing.T) {
		t.Parallel()
		service, registry := restoreTestService(t)
		restoreTestRemovePermanently(t, service, registry, "spec-cycle")
		service.restoreRollbackTimeout = 500 * time.Millisecond
		// The reload holds until the rollback budget expires, so only a tombstone write with its own budget survives.
		service.runtime = restoreTestRuntime{registry: registry, reload: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}
		service.builtinEnrollments = []bundledEnrollment{{
			Name: "spec-cycle",
			Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
				restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
				return errors.New("injected enrollment failure")
			},
		}}

		_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
		if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
			t.Fatalf("RestoreBuiltin(spent rollback budget) error = %v, want ErrExtensionRestoreFailed", err)
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("RestoreBuiltin(spent rollback budget) error = %v, want the reload's deadline in the chain", err)
		}
		restoreTestRequireTombstone(t, registry, "spec-cycle", true)
		if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("Get(after rollback) error = %v, want ErrExtensionNotFound", err)
		}
	})

	t.Run(
		"Should write the tombstone back when the partial row removal deletes the row and then fails to compensate",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					return errors.New("injected enrollment failure")
				},
			}}
			// The removal deletes the partial row and then reports a failed compensation, so its error is not proof
			// that the row remains. Only the read-back shows that no row is left.
			service.partialRowRemoval = func(_ context.Context, name string) error {
				if err := registry.Uninstall(name); err != nil {
					t.Fatalf("Uninstall(%q) error = %v", name, err)
				}
				return extensionpkg.ErrExtensionRemovalCompensationFailed
			}

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
				t.Fatalf("RestoreBuiltin(removal after delete) error = %v, want ErrExtensionRestoreFailed", err)
			}
			if !errors.Is(err, extensionpkg.ErrExtensionRemovalCompensationFailed) {
				t.Fatalf("RestoreBuiltin(removal after delete) error = %v, want the removal failure in the chain", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)
			if _, err := registry.Get("spec-cycle"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
				t.Fatalf("Get(after rollback) error = %v, want ErrExtensionNotFound", err)
			}
		},
	)

	t.Run(
		"Should keep the tombstone clear when the partial row cannot be read back after its removal",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					return errors.New("injected enrollment failure")
				},
			}}
			// The removal reports success and the row stays, while the read-back fails with a reason other than
			// not-found. Absence is unconfirmed, so the rollback must not write the tombstone beside the row.
			readErr := errors.New("injected read-back failure")
			service.partialRowRemoval = func(context.Context, string) error { return nil }
			service.partialRowLookup = func(context.Context, string) error { return readErr }

			_, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t))
			if !errors.Is(err, extensionpkg.ErrExtensionRestoreFailed) {
				t.Fatalf("RestoreBuiltin(unreadable partial row) error = %v, want ErrExtensionRestoreFailed", err)
			}
			if !errors.Is(err, readErr) {
				t.Fatalf(
					"RestoreBuiltin(unreadable partial row) error = %v, want the read-back failure in the chain",
					err,
				)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", false)
			installed, err := registry.Get("spec-cycle")
			if err != nil || installed.Source != extensionpkg.SourceBundled {
				t.Fatalf(
					"Get(partial row) = %#v, %v; want the bundled row kept beside a clear tombstone",
					installed,
					err,
				)
			}
		},
	)

	t.Run(
		"Should read the partial row back under the rollback budget",
		func(t *testing.T) {
			t.Parallel()
			service, registry := restoreTestService(t)
			restoreTestRemovePermanently(t, service, registry, "spec-cycle")
			service.builtinEnrollments = []bundledEnrollment{{
				Name: "spec-cycle",
				Enroll: func(_ compozyconfig.HomePaths, reg *extensionpkg.Registry) error {
					restoreTestSeedRow(t, reg, "spec-cycle", extensionpkg.SourceBundled)
					return errors.New("injected enrollment failure")
				},
			}}
			service.partialRowRemoval = func(context.Context, string) error { return nil }
			service.partialRowLookup = func(ctx context.Context, _ string) error {
				if _, ok := ctx.Deadline(); !ok {
					return errors.New("read-back ran without the rollback deadline")
				}
				return extensionpkg.ErrExtensionNotFound
			}

			if _, err := service.RestoreBuiltin(t.Context(), "spec-cycle", restoreTestOperator(t)); !errors.Is(
				err,
				extensionpkg.ErrExtensionRestoreFailed,
			) {
				t.Fatalf("RestoreBuiltin(rollback budget) error = %v, want ErrExtensionRestoreFailed", err)
			}
			restoreTestRequireTombstone(t, registry, "spec-cycle", true)
		},
	)
}
