package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/subprocess"
)

const (
	defaultProviderInitializeTimeout = 15 * time.Second
	defaultProviderShutdownTimeout   = 15 * time.Second
)

// ProviderInitialState is one resolved instance state emitted after reconciliation.
type ProviderInitialState struct {
	BridgeInstanceID string
	Status           bridgepkg.BridgeStatus
	Degradation      *bridgepkg.BridgeDegradation
}

// ProviderReconcileFunc reconciles one complete managed-instance snapshot.
type ProviderReconcileFunc func(
	context.Context,
	[]subprocess.InitializeBridgeManagedInstance,
) ([]ProviderInitialState, error)

// ProviderLifecycleConfig configures shared provider lifecycle orchestration.
type ProviderLifecycleConfig struct {
	ProviderName       string
	Markers            *AdapterMarkers
	Host               *ProviderHost
	Reconcile          ProviderReconcileFunc
	FinalizeInitialize func(error)
	OnStop             func()
	ShutdownResources  func(context.Context) error
	HealthCheck        func() error
	InitializeTimeout  time.Duration
	ShutdownTimeout    time.Duration
}

// ProviderLifecycle owns provider session, initialization work, stop state, and health error.
type ProviderLifecycle struct {
	config ProviderLifecycleConfig
	ctx    context.Context
	cancel context.CancelFunc

	mu         sync.RWMutex
	session    *Session
	lastError  error
	initCancel context.CancelFunc

	stopCh      chan struct{}
	stopOnce    sync.Once
	initDone    chan struct{}
	initOnce    sync.Once
	routesReady chan struct{}
	routesOnce  sync.Once
	taskMu      sync.Mutex
	activeTask  int
	done        chan struct{}
	doneOnce    sync.Once
}

// NewProviderLifecycle creates one shared provider lifecycle.
func NewProviderLifecycle(config ProviderLifecycleConfig) (*ProviderLifecycle, error) {
	config.ProviderName = strings.TrimSpace(config.ProviderName)
	if config.ProviderName == "" {
		return nil, errors.New("bridgesdk: provider lifecycle name is required")
	}
	if config.Markers == nil {
		config.Markers = NewAdapterMarkers(config.ProviderName, io.Discard)
	}
	if config.InitializeTimeout <= 0 {
		config.InitializeTimeout = defaultProviderInitializeTimeout
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = defaultProviderShutdownTimeout
	}
	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	lifecycle := &ProviderLifecycle{
		config:      config,
		ctx:         lifecycleCtx,
		cancel:      lifecycleCancel,
		stopCh:      make(chan struct{}),
		initDone:    make(chan struct{}),
		routesReady: make(chan struct{}),
		done:        make(chan struct{}),
	}
	if lifecycle.config.Host == nil {
		lifecycle.config.Host = NewProviderHost(lifecycle.stopCh, lifecycle.config.Markers)
	}
	return lifecycle, nil
}

// Serve records process startup and runs the shared RPC runtime.
func (l *ProviderLifecycle) Serve(
	ctx context.Context,
	runtime *Runtime,
	stdin io.Reader,
	stdout io.Writer,
) error {
	if l == nil || runtime == nil {
		return errors.New("bridgesdk: provider lifecycle runtime is required")
	}
	l.config.Markers.RecordStart(os.Getpid())
	return runtime.Serve(ctx, stdin, stdout)
}

// Initialize stores the negotiated session and starts ownership reconciliation.
func (l *ProviderLifecycle) Initialize(ctx context.Context, session *Session) error {
	if l == nil || session == nil {
		return errors.New("bridgesdk: provider lifecycle session is required")
	}
	if ctx == nil {
		return errors.New("bridgesdk: provider lifecycle initialize context is required")
	}
	ctx, cancel := context.WithTimeout(ctx, l.config.InitializeTimeout)
	l.mu.Lock()
	l.session = session
	l.lastError = nil
	l.initCancel = cancel
	l.mu.Unlock()
	l.config.Markers.RecordInitialize(&InitializeMarker{
		Request:  session.InitializeRequest(),
		Response: session.InitializeResponse(),
	})
	if !l.Go(func() {
		defer cancel()
		l.runAfterInitialize(ctx, session)
	}) {
		cancel()
		l.initOnce.Do(func() { close(l.initDone) })
		l.MarkRoutesReady()
		return ErrProviderStopped
	}
	return nil
}

func (l *ProviderLifecycle) runAfterInitialize(ctx context.Context, session ProviderHostSession) {
	defer l.initOnce.Do(func() { close(l.initDone) })
	defer l.MarkRoutesReady()
	listed, ownershipErr := l.config.Host.SyncOwnedInstances(ctx, session)
	fetched := make([]bridgepkg.BridgeInstance, 0, len(listed))
	if ownershipErr == nil {
		for _, managed := range listed {
			instance, err := l.config.Host.GetOwnedInstance(ctx, session, managed.Instance.ID)
			if err != nil {
				ownershipErr = err
				break
			}
			if instance != nil {
				fetched = append(fetched, *instance)
			}
		}
	}
	if len(listed) == 0 {
		listed = session.CachedInstances()
	}
	ownership := OwnershipMarker{
		Listed:  managedProviderInstancesToInstances(listed),
		Fetched: fetched,
	}
	if ownershipErr != nil {
		ownership.Error = ownershipErr.Error()
	}
	l.config.Markers.RecordOwnership(ownership)

	if l.Stopped() {
		return
	}
	var reconcileErr error
	var states []ProviderInitialState
	if l.config.Reconcile != nil {
		states, reconcileErr = l.config.Reconcile(ctx, listed)
	}
	stateErr := l.reportInitialStates(ctx, session, states)
	resultErr := errors.Join(ownershipErr, reconcileErr, stateErr)
	if l.config.FinalizeInitialize != nil {
		l.config.FinalizeInitialize(resultErr)
		return
	}
	if resultErr != nil {
		l.SetError(resultErr)
	} else {
		l.ClearError()
	}
}

// MarkRoutesReady admits delivery after the provider atomically publishes its first route snapshot.
func (l *ProviderLifecycle) MarkRoutesReady() {
	if l == nil {
		return
	}
	l.routesOnce.Do(func() { close(l.routesReady) })
}

// RoutesReady closes when the provider publishes or definitively rejects its first route snapshot.
func (l *ProviderLifecycle) RoutesReady() <-chan struct{} {
	if l == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return l.routesReady
}

// Initialized closes after ownership, reconciliation, and initial state reporting finish.
func (l *ProviderLifecycle) Initialized() <-chan struct{} {
	if l == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return l.initDone
}

func (l *ProviderLifecycle) reportInitialStates(
	ctx context.Context,
	session ProviderHostSession,
	states []ProviderInitialState,
) error {
	var reportErr error
	for _, state := range states {
		if l.Stopped() {
			return errors.Join(reportErr, ErrProviderStopped)
		}
		status := state.Status.Normalize()
		if status == "" {
			status = bridgepkg.BridgeStatusReady
		}
		if err := l.config.Host.ReportState(
			ctx,
			session,
			state.BridgeInstanceID,
			status,
			state.Degradation,
		); err != nil {
			reportErr = errors.Join(reportErr, err)
		}
	}
	return reportErr
}

// Shutdown stops provider resources, joins lifecycle work, and records shutdown.
func (l *ProviderLifecycle) Shutdown(
	ctx context.Context,
	_ *Session,
	request subprocess.ShutdownRequest,
) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("bridgesdk: provider lifecycle shutdown context is required")
	}
	l.Stop()
	shutdownTimeout := minimumProviderShutdownTimeout(l.config.ShutdownTimeout, request.DeadlineMS)
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	var shutdownErr error
	if l.config.ShutdownResources != nil {
		shutdownErr = l.config.ShutdownResources(shutdownCtx)
	}
	shutdownErr = errors.Join(shutdownErr, l.Wait(shutdownCtx))
	l.config.Markers.RecordShutdown(os.Getpid())
	return shutdownErr
}

// Stop closes the provider stop signal and provider-specific resources once.
func (l *ProviderLifecycle) Stop() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() {
		l.taskMu.Lock()
		close(l.stopCh)
		l.cancel()
		if l.activeTask == 0 {
			l.doneOnce.Do(func() { close(l.done) })
		}
		l.taskMu.Unlock()
		l.mu.RLock()
		cancelInitialize := l.initCancel
		l.mu.RUnlock()
		if cancelInitialize != nil {
			cancelInitialize()
		}
		if l.config.OnStop != nil {
			l.config.OnStop()
		}
	})
}

// Context returns a lifecycle-owned context canceled when provider shutdown begins.
func (l *ProviderLifecycle) Context() context.Context {
	if l == nil || l.ctx == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return l.ctx
}

// Go starts one lifecycle-owned goroutine unless shutdown already began.
// Panics in provider callbacks are recovered and exposed through Health.
func (l *ProviderLifecycle) Go(run func()) bool {
	if l == nil || run == nil {
		return false
	}
	l.taskMu.Lock()
	defer l.taskMu.Unlock()
	if l.Stopped() {
		return false
	}
	l.activeTask++
	go l.runTask(run)
	return true
}

func (l *ProviderLifecycle) runTask(run func()) {
	defer func() {
		if recovered := recover(); recovered != nil {
			l.SetError(fmt.Errorf("bridgesdk: provider lifecycle task panicked: %v", recovered))
		}
		l.taskMu.Lock()
		l.activeTask--
		if l.activeTask == 0 && l.Stopped() {
			l.doneOnce.Do(func() { close(l.done) })
		}
		l.taskMu.Unlock()
	}()
	run()
}

// Wait joins lifecycle-owned work after Stop closes task admission.
func (l *ProviderLifecycle) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("bridgesdk: provider lifecycle wait context is required")
	}
	select {
	case <-l.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func minimumProviderShutdownTimeout(configured time.Duration, deadlineMS int64) time.Duration {
	if deadlineMS <= 0 || deadlineMS > math.MaxInt64/int64(time.Millisecond) {
		return configured
	}
	return min(configured, time.Duration(deadlineMS)*time.Millisecond)
}

// Session returns the negotiated provider session.
func (l *ProviderLifecycle) Session() *Session {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.session
}

// StopChannel returns the provider lifecycle stop signal.
func (l *ProviderLifecycle) StopChannel() <-chan struct{} {
	if l == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return l.stopCh
}

// Stopped reports whether provider shutdown began.
func (l *ProviderLifecycle) Stopped() bool {
	if l == nil {
		return true
	}
	select {
	case <-l.stopCh:
		return true
	default:
		return false
	}
}

// SetError records one provider health error.
func (l *ProviderLifecycle) SetError(err error) {
	if l == nil || err == nil {
		return
	}
	l.mu.Lock()
	l.lastError = err
	l.mu.Unlock()
}

// ClearError clears the provider health error.
func (l *ProviderLifecycle) ClearError() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.lastError = nil
	l.mu.Unlock()
}

// Health returns provider-specific health or the shared lifecycle error.
func (l *ProviderLifecycle) Health() error {
	if l == nil {
		return errors.New("bridgesdk: provider lifecycle is required")
	}
	if l.config.HealthCheck != nil {
		return l.config.HealthCheck()
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.lastError == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", l.config.ProviderName, l.lastError)
}

// Host returns the shared provider Host API operations.
func (l *ProviderLifecycle) Host() *ProviderHost {
	if l == nil {
		return nil
	}
	return l.config.Host
}
