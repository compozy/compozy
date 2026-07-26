package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

type providerHostSessionStub struct {
	mu sync.Mutex

	managed       []subprocess.InitializeBridgeManagedInstance
	syncFailures  int
	syncCalls     int
	getCalls      int
	reported      []bridgepkg.BridgesInstancesReportStateParams
	ingestResults []*bridgepkg.BridgesMessagesIngestResult
}

func (s *providerHostSessionStub) SyncInstances(context.Context) ([]subprocess.InitializeBridgeManagedInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncCalls++
	if s.syncCalls <= s.syncFailures {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeNotInitialized, "Not initialized", nil)
	}
	return append([]subprocess.InitializeBridgeManagedInstance(nil), s.managed...), nil
}

func (s *providerHostSessionStub) CachedInstances() []subprocess.InitializeBridgeManagedInstance {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]subprocess.InitializeBridgeManagedInstance(nil), s.managed...)
}

func (s *providerHostSessionStub) GetBridgeInstance(
	_ context.Context,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCalls++
	for _, managed := range s.managed {
		if managed.Instance.ID == bridgeInstanceID {
			instance := managed.Instance
			return &instance, nil
		}
	}
	return nil, errors.New("instance not found")
}

func (s *providerHostSessionStub) ReportBridgeInstanceState(
	_ context.Context,
	params bridgepkg.BridgesInstancesReportStateParams,
) (*bridgepkg.BridgeInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reported = append(s.reported, params)
	instance := testBridgeInstance(params.BridgeInstanceID)
	instance.Status = params.Status
	return &instance, nil
}

func (s *providerHostSessionStub) IngestBridgeMessage(
	_ context.Context,
	_ bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ingestResults) == 0 {
		return nil, errors.New("no ingest result")
	}
	result := s.ingestResults[0]
	s.ingestResults = s.ingestResults[1:]
	return result, nil
}

func TestProviderHostRetriesStartupAndOwnsStateTransitions(t *testing.T) {
	t.Parallel()

	t.Run("Should retry only not initialized and avoid duplicate ready reports", func(t *testing.T) {
		t.Parallel()

		waits := 0
		host := NewProviderHost(nil, NewAdapterMarkers("test", io.Discard), WithProviderHostRetryWait(
			func(context.Context, <-chan struct{}, time.Duration) error {
				waits++
				return nil
			},
		))
		stub := &providerHostSessionStub{
			managed:       []subprocess.InitializeBridgeManagedInstance{{Instance: testBridgeInstance("brg-1")}},
			syncFailures:  2,
			ingestResults: []*bridgepkg.BridgesMessagesIngestResult{{SessionID: "sess-1"}},
		}

		managed, err := host.SyncOwnedInstances(t.Context(), stub)
		if err != nil {
			t.Fatalf("SyncOwnedInstances() error = %v", err)
		}
		if len(managed) != 1 || stub.syncCalls != 3 || waits != 2 {
			t.Fatalf("sync result = %d, calls = %d, waits = %d", len(managed), stub.syncCalls, waits)
		}
		if err := host.ReportReadyIfNeeded(t.Context(), stub, "brg-1"); err != nil {
			t.Fatalf("ReportReadyIfNeeded(first) error = %v", err)
		}
		if err := host.ReportReadyIfNeeded(t.Context(), stub, "brg-1"); err != nil {
			t.Fatalf("ReportReadyIfNeeded(second) error = %v", err)
		}
		if got := len(stub.reported); got != 1 {
			t.Fatalf("state reports = %d, want 1", got)
		}
		if got := host.Statuses()["brg-1"]; got != bridgepkg.BridgeStatusReady {
			t.Fatalf("Statuses()[brg-1] = %q, want ready", got)
		}
		host.ForgetStatus("brg-1")
		if _, ok := host.Statuses()["brg-1"]; ok {
			t.Fatal("Statuses()[brg-1] retained forgotten status")
		}
		result, err := host.IngestBridgeMessage(
			t.Context(),
			stub,
			bridgepkg.InboundMessageEnvelope{BridgeInstanceID: "brg-1"},
		)
		if err != nil || result.SessionID != "sess-1" {
			t.Fatalf("IngestBridgeMessage() = (%#v, %v), want sess-1", result, err)
		}
	})
}

func TestProviderLifecycleReconcilesOwnershipAndStopsCleanly(t *testing.T) {
	t.Parallel()

	t.Run("Should reconcile fetched ownership report initial state and reject work after stop", func(t *testing.T) {
		t.Parallel()

		stub := &providerHostSessionStub{
			managed: []subprocess.InitializeBridgeManagedInstance{{Instance: testBridgeInstance("brg-1")}},
		}
		reconciled := make(chan []subprocess.InitializeBridgeManagedInstance, 1)
		finalized := make(chan error, 1)
		lifecycle, err := NewProviderLifecycle(ProviderLifecycleConfig{
			ProviderName: "test",
			Markers:      NewAdapterMarkers("test", io.Discard),
			Reconcile: func(
				_ context.Context,
				managed []subprocess.InitializeBridgeManagedInstance,
			) ([]ProviderInitialState, error) {
				reconciled <- managed
				return []ProviderInitialState{{BridgeInstanceID: "brg-1", Status: bridgepkg.BridgeStatusReady}}, nil
			},
			FinalizeInitialize: func(err error) { finalized <- err },
		})
		if err != nil {
			t.Fatalf("NewProviderLifecycle() error = %v", err)
		}

		lifecycle.runAfterInitialize(t.Context(), stub)
		if err := <-finalized; err != nil {
			t.Fatalf("FinalizeInitialize() error = %v", err)
		}
		managed := <-reconciled
		if len(managed) != 1 || stub.getCalls != 1 || len(stub.reported) != 1 {
			t.Fatalf("managed = %d, get calls = %d, reports = %d", len(managed), stub.getCalls, len(stub.reported))
		}

		lifecycle.Stop()
		if lifecycle.Go(func() { t.Error("work started after stop") }) {
			t.Fatal("Go() = true after Stop()")
		}
		if err := lifecycle.Wait(t.Context()); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	})

	t.Run("Should expose lifecycle health signals and drain owned work", func(t *testing.T) {
		t.Parallel()

		stopped := make(chan struct{})
		shutdownResources := false
		lifecycle, err := NewProviderLifecycle(ProviderLifecycleConfig{
			ProviderName: "test",
			OnStop:       func() { close(stopped) },
			ShutdownResources: func(context.Context) error {
				shutdownResources = true
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewProviderLifecycle() error = %v", err)
		}
		if lifecycle.Host() == nil || lifecycle.Session() != nil {
			t.Fatalf("initial lifecycle host/session = (%#v, %#v)", lifecycle.Host(), lifecycle.Session())
		}
		lifecycle.SetError(errors.New("boom"))
		if err := lifecycle.Health(); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("Health() error = %v, want boom", err)
		}
		lifecycle.ClearError()
		if err := lifecycle.Health(); err != nil {
			t.Fatalf("Health(clear) error = %v", err)
		}
		workStarted := make(chan struct{})
		workRelease := make(chan struct{})
		if !lifecycle.Go(func() {
			close(workStarted)
			<-workRelease
		}) {
			t.Fatal("Go() = false before shutdown")
		}
		<-workStarted
		close(workRelease)
		if err := lifecycle.Shutdown(
			t.Context(),
			nil,
			subprocess.ShutdownRequest{DeadlineMS: 1000},
		); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		<-stopped
		if !shutdownResources || !lifecycle.Stopped() {
			t.Fatalf("shutdown resources/stopped = (%t, %t)", shutdownResources, lifecycle.Stopped())
		}
		select {
		case <-lifecycle.StopChannel():
		default:
			t.Fatal("StopChannel() remained open after shutdown")
		}
	})

	t.Run("Should initialize a negotiated session and publish readiness signals", func(t *testing.T) {
		t.Parallel()

		request := testInitializeRequest()
		host := NewHostAPIClientFromCall(func(_ context.Context, method string, params any, result any) error {
			switch method {
			case "bridges/instances/list":
				instances := result.(*[]bridgepkg.BridgeInstance)
				*instances = []bridgepkg.BridgeInstance{request.Runtime.Bridge.ManagedInstances[0].Instance}
				return nil
			case "bridges/instances/get":
				_ = params
				instance := result.(*bridgepkg.BridgeInstance)
				*instance = request.Runtime.Bridge.ManagedInstances[0].Instance
				return nil
			default:
				return fmt.Errorf("unexpected Host API method %q", method)
			}
		})
		session := &Session{
			request:  request,
			response: subprocess.InitializeResponse{ProtocolVersion: "1"},
			host:     host,
			cache:    NewInstanceCache(request.Runtime.Bridge),
		}
		lifecycle, err := NewProviderLifecycle(ProviderLifecycleConfig{
			ProviderName: "test",
			Reconcile: func(
				context.Context,
				[]subprocess.InitializeBridgeManagedInstance,
			) ([]ProviderInitialState, error) {
				return nil, nil
			},
		})
		if err != nil {
			t.Fatalf("NewProviderLifecycle() error = %v", err)
		}
		if err := lifecycle.Initialize(t.Context(), session); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		select {
		case <-lifecycle.Initialized():
		case <-t.Context().Done():
			t.Fatal("Initialized() did not close")
		}
		select {
		case <-lifecycle.RoutesReady():
		case <-t.Context().Done():
			t.Fatal("RoutesReady() did not close")
		}
		if lifecycle.Session() != session || len(session.CachedInstances()) != 1 {
			t.Fatalf("Session()/CachedInstances() = (%p, %d)", lifecycle.Session(), len(session.CachedInstances()))
		}
		lifecycle.Stop()
	})

	t.Run("Should cancel and join initialization before shutdown returns", func(t *testing.T) {
		t.Parallel()

		request := testInitializeRequest()
		host := NewHostAPIClientFromCall(func(_ context.Context, method string, _ any, result any) error {
			switch method {
			case "bridges/instances/list":
				instances := result.(*[]bridgepkg.BridgeInstance)
				*instances = []bridgepkg.BridgeInstance{request.Runtime.Bridge.ManagedInstances[0].Instance}
				return nil
			case "bridges/instances/get":
				instance := result.(*bridgepkg.BridgeInstance)
				*instance = request.Runtime.Bridge.ManagedInstances[0].Instance
				return nil
			default:
				return fmt.Errorf("unexpected Host API method %q", method)
			}
		})
		session := &Session{
			request:  request,
			response: subprocess.InitializeResponse{ProtocolVersion: "1"},
			host:     host,
			cache:    NewInstanceCache(request.Runtime.Bridge),
		}
		reconcileStarted := make(chan struct{})
		reconcileExited := make(chan struct{})
		lifecycle, err := NewProviderLifecycle(ProviderLifecycleConfig{
			ProviderName: "test",
			Reconcile: func(ctx context.Context, _ []subprocess.InitializeBridgeManagedInstance) ([]ProviderInitialState, error) {
				close(reconcileStarted)
				<-ctx.Done()
				close(reconcileExited)
				return nil, ctx.Err()
			},
		})
		if err != nil {
			t.Fatalf("NewProviderLifecycle() error = %v", err)
		}
		if err := lifecycle.Initialize(t.Context(), session); err != nil {
			t.Fatalf("Initialize() error = %v", err)
		}
		<-reconcileStarted
		if err := lifecycle.Shutdown(
			t.Context(),
			nil,
			subprocess.ShutdownRequest{DeadlineMS: 1000},
		); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		select {
		case <-reconcileExited:
		default:
			t.Fatal("Shutdown() returned before initialization exited")
		}
	})
}
