package bridgesdk

import (
	"context"
	"errors"
	"maps"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	providerHostRetryAttempts = 6
	providerHostRetryInitial  = 10 * time.Millisecond
	providerHostRetryMaximum  = 100 * time.Millisecond
)

// ProviderHostSession is the narrow Host API surface used by shared provider lifecycle code.
type ProviderHostSession interface {
	SyncInstances(context.Context) ([]subprocess.InitializeBridgeManagedInstance, error)
	CachedInstances() []subprocess.InitializeBridgeManagedInstance
	GetBridgeInstance(context.Context, string) (*bridgepkg.BridgeInstance, error)
	ReportBridgeInstanceState(
		context.Context,
		bridgepkg.BridgesInstancesReportStateParams,
	) (*bridgepkg.BridgeInstance, error)
	IngestBridgeMessage(
		context.Context,
		bridgepkg.InboundMessageEnvelope,
	) (*bridgepkg.BridgesMessagesIngestResult, error)
}

type providerHostRetryWait func(context.Context, <-chan struct{}, time.Duration) error

// ProviderHostOption customizes shared provider Host API behavior.
type ProviderHostOption func(*ProviderHost)

// WithProviderHostRetryWait replaces retry waiting for deterministic tests.
func WithProviderHostRetryWait(wait providerHostRetryWait) ProviderHostOption {
	return func(host *ProviderHost) {
		host.wait = wait
	}
}

// ProviderHost centralizes retried ownership, state-report, and ingest Host API calls.
type ProviderHost struct {
	stop    <-chan struct{}
	markers *AdapterMarkers
	wait    providerHostRetryWait

	mu       sync.RWMutex
	statuses map[string]bridgepkg.BridgeStatus
}

// NewProviderHost creates shared Host API operations for one provider lifecycle.
func NewProviderHost(
	stop <-chan struct{},
	markers *AdapterMarkers,
	opts ...ProviderHostOption,
) *ProviderHost {
	host := &ProviderHost{
		stop:     stop,
		markers:  markers,
		wait:     waitProviderHostRetry,
		statuses: make(map[string]bridgepkg.BridgeStatus),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(host)
		}
	}
	return host
}

// SyncOwnedInstances refreshes one provider's managed-instance cache with startup retry.
func (h *ProviderHost) SyncOwnedInstances(
	ctx context.Context,
	session ProviderHostSession,
) ([]subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil {
		return nil, errors.New("bridgesdk: provider host session is required")
	}
	var result []subprocess.InitializeBridgeManagedInstance
	err := h.Retry(ctx, func(callCtx context.Context) error {
		items, callErr := session.SyncInstances(callCtx)
		if callErr == nil {
			result = items
		}
		return callErr
	})
	return result, err
}

// GetOwnedInstance reads one provider-owned bridge instance with startup retry.
func (h *ProviderHost) GetOwnedInstance(
	ctx context.Context,
	session ProviderHostSession,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	if session == nil {
		return nil, errors.New("bridgesdk: provider host session is required")
	}
	var result *bridgepkg.BridgeInstance
	err := h.Retry(ctx, func(callCtx context.Context) error {
		instance, callErr := session.GetBridgeInstance(callCtx, bridgeInstanceID)
		if callErr == nil {
			result = instance
		}
		return callErr
	})
	return result, err
}

// ReportState writes one provider-owned bridge status and marker.
func (h *ProviderHost) ReportState(
	ctx context.Context,
	session ProviderHostSession,
	bridgeInstanceID string,
	status bridgepkg.BridgeStatus,
	degradation *bridgepkg.BridgeDegradation,
) error {
	if session == nil {
		return errors.New("bridgesdk: provider host session is required")
	}
	bridgeInstanceID = strings.TrimSpace(bridgeInstanceID)
	var result *bridgepkg.BridgeInstance
	err := h.Retry(ctx, func(callCtx context.Context) error {
		instance, callErr := session.ReportBridgeInstanceState(
			callCtx,
			bridgepkg.BridgesInstancesReportStateParams{
				BridgeInstanceID: bridgeInstanceID,
				Status:           status,
				Degradation:      cloneProviderDegradation(degradation),
			},
		)
		if callErr == nil {
			result = instance
		}
		return callErr
	})
	if err != nil {
		h.markers.RecordState(StateMarker{
			BridgeInstanceID: bridgeInstanceID,
			Status:           status,
			Error:            err.Error(),
		})
		return err
	}
	if result == nil {
		return errors.New("bridgesdk: provider state report returned no instance")
	}
	h.mu.Lock()
	h.statuses[bridgeInstanceID] = result.Status.Normalize()
	h.mu.Unlock()
	h.markers.RecordState(StateMarker{
		BridgeInstanceID: result.ID,
		Status:           result.Status,
		Instance:         *result,
	})
	return nil
}

// ReportReadyIfNeeded restores ready state only when the provider previously reported another state.
func (h *ProviderHost) ReportReadyIfNeeded(
	ctx context.Context,
	session ProviderHostSession,
	bridgeInstanceID string,
) error {
	bridgeInstanceID = strings.TrimSpace(bridgeInstanceID)
	h.mu.RLock()
	status := h.statuses[bridgeInstanceID]
	h.mu.RUnlock()
	if status == bridgepkg.BridgeStatusReady {
		return nil
	}
	return h.ReportState(ctx, session, bridgeInstanceID, bridgepkg.BridgeStatusReady, nil)
}

// Statuses returns the last provider-reported status for every managed instance.
func (h *ProviderHost) Statuses() map[string]bridgepkg.BridgeStatus {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	statuses := make(map[string]bridgepkg.BridgeStatus, len(h.statuses))
	maps.Copy(statuses, h.statuses)
	return statuses
}

// ForgetStatus removes retained state for an instance no longer owned by the provider.
func (h *ProviderHost) ForgetStatus(bridgeInstanceID string) {
	if h == nil {
		return
	}
	h.mu.Lock()
	delete(h.statuses, strings.TrimSpace(bridgeInstanceID))
	h.mu.Unlock()
}

// IngestBridgeMessage submits one normalized inbound envelope and records its harness marker.
func (h *ProviderHost) IngestBridgeMessage(
	ctx context.Context,
	session ProviderHostSession,
	envelope bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	if session == nil {
		return nil, errors.New("bridgesdk: provider host session is required")
	}
	var result *bridgepkg.BridgesMessagesIngestResult
	err := h.Retry(ctx, func(callCtx context.Context) error {
		ingestResult, callErr := session.IngestBridgeMessage(callCtx, envelope)
		if callErr == nil {
			result = ingestResult
		}
		return callErr
	})
	marker := IngestMarker{Envelope: envelope}
	if err != nil {
		marker.Error = err.Error()
	} else if result != nil {
		marker.Result = *result
	}
	h.markers.RecordIngest(marker)
	return result, err
}

// Retry retries only the startup NotInitialized RPC error with bounded backoff.
func (h *ProviderHost) Retry(ctx context.Context, call func(context.Context) error) error {
	if ctx == nil {
		return errors.New("bridgesdk: provider host call context is required")
	}
	if call == nil {
		return errors.New("bridgesdk: provider host call is required")
	}
	delay := providerHostRetryInitial
	var lastErr error
	for range providerHostRetryAttempts {
		err := call(ctx)
		if err == nil {
			return nil
		}
		if !isProviderNotInitializedError(err) {
			return err
		}
		lastErr = err
		if err := h.wait(ctx, h.stop, delay); err != nil {
			return err
		}
		delay = min(delay*2, providerHostRetryMaximum)
	}
	return lastErr
}

func waitProviderHostRetry(ctx context.Context, stop <-chan struct{}, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-stop:
		return ErrProviderStopped
	case <-timer.C:
		return nil
	}
}

func isProviderNotInitializedError(err error) bool {
	var rpcErr *subprocess.RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == bridgeSDKRPCCodeNotInitialized
}

func cloneProviderDegradation(value *bridgepkg.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func managedProviderInstancesToInstances(
	managed []subprocess.InitializeBridgeManagedInstance,
) []bridgepkg.BridgeInstance {
	instances := make([]bridgepkg.BridgeInstance, 0, len(managed))
	for _, item := range managed {
		instances = append(instances, item.Instance)
	}
	return instances
}
