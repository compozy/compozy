package update

import (
	"context"
	"time"
)

// CoordinatorReleaseManager supplies verified release acquisition and application.
type CoordinatorReleaseManager interface {
	ResolveReleaseByTag(context.Context, string) (*Release, error)
	ApplyReleaseObserved(context.Context, *Release, ApplyObserver) (AppliedBinary, error)
}

// CoordinatorBinaryManager supplies installed-runtime mutation and recovery.
type CoordinatorBinaryManager interface {
	RuntimeTargetPath() string
	Restore(AppliedBinary) error
	Finalize(AppliedBinary) error
}

// CoordinatorManager is the complete manager capability implemented by Manager.
type CoordinatorManager interface {
	CoordinatorReleaseManager
	CoordinatorBinaryManager
}

// MutationLock is the short critical-section lock held only around the runtime swap.
type MutationLock interface {
	Release() error
}

// CoordinatorRuntime bridges process lifecycle operations owned outside internal/update.
type CoordinatorRuntime interface {
	AcquireMutationLock(context.Context) (MutationLock, error)
	RestartDaemon(context.Context) error
	HealthCheck(context.Context) error
	InstalledApp(context.Context) (InstalledApp, error)
}

// InstalledApp preserves the difference between no desktop app and an app whose version is unreadable.
type InstalledApp struct {
	Present bool
	Version string
}

// CoordinatorConfig wires the sole runtime-mutation executor.
type CoordinatorConfig struct {
	Store          *OperationStore
	ReleaseManager CoordinatorReleaseManager
	BinaryManager  CoordinatorBinaryManager
	Runtime        CoordinatorRuntime
	LeaseDuration  time.Duration
	RenewInterval  time.Duration
	Now            func() time.Time
}

// Coordinator executes and recovers one durable update operation.
type Coordinator struct {
	store          *OperationStore
	releaseManager CoordinatorReleaseManager
	binaryManager  CoordinatorBinaryManager
	runtime        CoordinatorRuntime
	leaseDuration  time.Duration
	renewInterval  time.Duration
	now            func() time.Time
}

// NewCoordinator constructs a detached-operation executor.
func NewCoordinator(cfg CoordinatorConfig) (*Coordinator, error) {
	if cfg.Store == nil || cfg.ReleaseManager == nil || cfg.BinaryManager == nil || cfg.Runtime == nil {
		return nil, errCoordinatorDependencies
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = DefaultLeaseDuration
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = cfg.LeaseDuration / 3
	}
	if cfg.RenewInterval <= 0 || cfg.RenewInterval >= cfg.LeaseDuration {
		return nil, errCoordinatorLeaseTiming
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Coordinator{
		store: cfg.Store, releaseManager: cfg.ReleaseManager, binaryManager: cfg.BinaryManager, runtime: cfg.Runtime,
		leaseDuration: cfg.LeaseDuration, renewInterval: cfg.RenewInterval, now: cfg.Now,
	}, nil
}
