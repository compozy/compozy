package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

const providerInitializeTimeout = 15 * time.Second

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
}

// ProviderLifecycle owns provider session, initialization work, stop state, and health error.
type ProviderLifecycle struct {
	config ProviderLifecycleConfig

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
	wg          sync.WaitGroup
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
	lifecycle := &ProviderLifecycle{
		config:      config,
		stopCh:      make(chan struct{}),
		initDone:    make(chan struct{}),
		routesReady: make(chan struct{}),
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
func (l *ProviderLifecycle) Initialize(_ context.Context, session *Session) error {
	if l == nil || session == nil {
		return errors.New("bridgesdk: provider lifecycle session is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerInitializeTimeout)
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
	_ context.Context,
	_ *Session,
	request subprocess.ShutdownRequest,
) error {
	if l == nil {
		return nil
	}
	l.Stop()
	shutdownCtx := context.Background()
	if request.DeadlineMS > 0 {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(
			context.Background(),
			time.Duration(request.DeadlineMS)*time.Millisecond,
		)
		defer cancel()
	}
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

// Go starts one lifecycle-owned goroutine unless shutdown already began.
func (l *ProviderLifecycle) Go(run func()) bool {
	if l == nil || run == nil {
		return false
	}
	l.taskMu.Lock()
	defer l.taskMu.Unlock()
	if l.Stopped() {
		return false
	}
	l.wg.Go(run)
	return true
}

// Wait joins lifecycle-owned work until completion or context cancellation.
func (l *ProviderLifecycle) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("bridgesdk: provider lifecycle wait context is required")
	}
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
