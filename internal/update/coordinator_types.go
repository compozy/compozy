package update

import (
	"context"
	"time"
)

// CoordinatorManager supplies verified release and binary mutation operations.
type CoordinatorManager interface {
	ResolveReleaseByTag(context.Context, string) (*Release, error)
	ApplyReleaseObserved(context.Context, *Release, ApplyObserver) (AppliedBinary, error)
	RuntimeTargetPath() string
	Restore(AppliedBinary) error
	Finalize(AppliedBinary) error
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
	InstalledAppVersion(context.Context) (string, error)
}

// CoordinatorConfig wires the sole runtime-mutation executor.
type CoordinatorConfig struct {
	Store         *OperationStore
	Manager       CoordinatorManager
	Runtime       CoordinatorRuntime
	LeaseDuration time.Duration
	RenewInterval time.Duration
	Now           func() time.Time
}

// Coordinator executes and recovers one durable update operation.
type Coordinator struct {
	store         *OperationStore
	manager       CoordinatorManager
	runtime       CoordinatorRuntime
	leaseDuration time.Duration
	renewInterval time.Duration
	now           func() time.Time
}

// NewCoordinator constructs a detached-operation executor.
func NewCoordinator(cfg CoordinatorConfig) (*Coordinator, error) {
	if cfg.Store == nil || cfg.Manager == nil || cfg.Runtime == nil {
		return nil, errCoordinatorDependencies
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = 2 * time.Minute
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
		store: cfg.Store, manager: cfg.Manager, runtime: cfg.Runtime,
		leaseDuration: cfg.LeaseDuration, renewInterval: cfg.RenewInterval, now: cfg.Now,
	}, nil
}
