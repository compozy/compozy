package bridgesdk

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
)

const (
	runtimeErrorKey       = "error"
	runtimeShutdownMethod = "shutdown"
)

// InitializeHandler runs after the provider runtime receives the negotiated
// initialize request and seeds its Host API client and managed-instance cache.
type InitializeHandler func(context.Context, *Session) error

// DeliveryHandler handles one daemon-originated `bridges/deliver` request.
type DeliveryHandler func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)

// ProgressHandler handles presentation-only tool progress and the empty final
// that closes a progress-only turn. When omitted, the runtime acknowledges
// both without invoking the provider's text handler.
type ProgressHandler func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error)

// TargetSnapshotHandler handles one daemon-originated bridge target discovery request.
type TargetSnapshotHandler func(
	context.Context,
	*Session,
	bridgepkg.BridgeTargetSnapshotRequest,
) ([]bridgepkg.BridgeTargetSnapshot, error)

// CheckHandler runs one read-only provider-owned bridge check operation.
type CheckHandler func(
	context.Context,
	*Session,
	bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error)

// WebhookRegistrationHandler registers the configured provider webhook.
type WebhookRegistrationHandler func(
	context.Context,
	*Session,
	bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error)

// HealthHandler handles one daemon health-check probe.
type HealthHandler func(context.Context, *Session) error

// ShutdownHandler handles one daemon-originated cooperative shutdown request.
type ShutdownHandler func(context.Context, *Session, subprocess.ShutdownRequest) error

// RuntimeConfig configures the shared provider runtime scaffold.
type RuntimeConfig struct {
	ExtensionInfo   subprocess.InitializeExtensionInfo
	Initialize      InitializeHandler
	Deliver         DeliveryHandler
	Progress        ProgressHandler
	TargetSnapshots TargetSnapshotHandler
	Check           CheckHandler
	RegisterWebhook WebhookRegistrationHandler
	HealthCheck     HealthHandler
	Shutdown        ShutdownHandler
	Now             func() time.Time
}

// Runtime is the shared provider runtime scaffold built on the bridge SDK.
type Runtime struct {
	config RuntimeConfig

	mu           sync.RWMutex
	peer         *Peer
	session      *Session
	initializing bool

	shutdownState runtimeShutdownState
}

type runtimeShutdownState uint8

const (
	runtimeShutdownIdle runtimeShutdownState = iota
	runtimeShutdownRunning
	runtimeShutdownSucceeded
)

// Session captures the negotiated provider runtime session state.
type Session struct {
	request  subprocess.InitializeRequest
	response subprocess.InitializeResponse
	host     *HostAPIClient
	cache    *InstanceCache
	now      func() time.Time
}

// NewRuntime constructs the shared provider runtime scaffold.
func NewRuntime(config RuntimeConfig) (*Runtime, error) {
	if strings.TrimSpace(config.ExtensionInfo.Name) == "" {
		return nil, errors.New("bridgesdk: runtime extension info name is required")
	}
	if strings.TrimSpace(config.ExtensionInfo.Version) == "" {
		return nil, errors.New("bridgesdk: runtime extension info version is required")
	}
	if config.Deliver == nil {
		return nil, errors.New("bridgesdk: runtime deliver handler is required")
	}
	if config.Check == nil {
		return nil, errors.New("bridgesdk: runtime check handler is required")
	}
	if config.TargetSnapshots == nil {
		config.TargetSnapshots = TargetSnapshotsFromManagedInstances
	}
	if config.Now == nil {
		config.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &Runtime{config: config}, nil
}

// Serve runs the provider runtime over the supplied stdio transport.
func (r *Runtime) Serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	if r == nil {
		return errors.New("bridgesdk: runtime is required")
	}
	if ctx == nil {
		return errors.New("bridgesdk: runtime context is required")
	}

	peer, err := NewPeer(stdin, stdout)
	if err != nil {
		return err
	}
	if err := peer.Handle("initialize", r.handleInitialize); err != nil {
		return err
	}
	if err := peer.Handle("bridges/deliver", r.handleDeliver); err != nil {
		return err
	}
	if err := peer.Handle("bridges/targets/snapshot", r.handleTargetSnapshots); err != nil {
		return err
	}
	if err := peer.Handle(string(bridgepkg.ControlMethodCheck), r.handleCheck); err != nil {
		return err
	}
	if err := peer.Handle(string(bridgepkg.ControlMethodWebhookRegister), r.handleRegisterWebhook); err != nil {
		return err
	}
	if err := peer.Handle("health_check", r.handleHealthCheck); err != nil {
		return err
	}
	if err := peer.Handle(runtimeShutdownMethod, r.handleShutdown); err != nil {
		return err
	}

	r.mu.Lock()
	r.peer = peer
	r.mu.Unlock()

	return peer.Serve(ctx)
}

// Session returns the negotiated runtime session once initialize succeeds.
func (r *Runtime) Session() *Session {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.session
}

// BridgeRuntime returns the current managed-instance runtime snapshot.
func (s *Session) BridgeRuntime() *subprocess.InitializeBridgeRuntime {
	if s == nil || s.cache == nil {
		return nil
	}
	return s.cache.Snapshot()
}

// InitializeRequest returns a clone of the negotiated initialize request.
func (s *Session) InitializeRequest() subprocess.InitializeRequest {
	if s == nil {
		return subprocess.InitializeRequest{}
	}

	return subprocess.CloneInitializeRequest(s.request)
}

// InitializeResponse returns a copy of the initialize response sent by the runtime.
func (s *Session) InitializeResponse() subprocess.InitializeResponse {
	if s == nil {
		return subprocess.InitializeResponse{}
	}

	response := s.response
	response.AcceptedCapabilities.Provides = append([]string(nil), response.AcceptedCapabilities.Provides...)
	response.AcceptedCapabilities.Permissions = append(
		[]extensionprotocol.HostAPIMethod(nil),
		response.AcceptedCapabilities.Permissions...,
	)
	response.ImplementedMethods = append([]string(nil), response.ImplementedMethods...)
	response.SupportedHookEvents = append([]string(nil), response.SupportedHookEvents...)
	return response
}

// HostAPI returns the typed bridge Host API client.
func (s *Session) HostAPI() *HostAPIClient {
	if s == nil {
		return nil
	}
	return s.host
}

// Cache returns the provider-owned managed-instance cache.
func (s *Session) Cache() *InstanceCache {
	if s == nil {
		return nil
	}
	return s.cache
}

// SyncInstances refreshes the managed-instance cache from the Host API.
func (s *Session) SyncInstances(ctx context.Context) ([]subprocess.InitializeBridgeManagedInstance, error) {
	if s == nil || s.cache == nil {
		return nil, errors.New("bridgesdk: runtime session cache is required")
	}
	return s.cache.Sync(ctx, s.host)
}

// AckDelivery builds and validates one delivery acknowledgement for the request.
func (s *Session) AckDelivery(
	req bridgepkg.DeliveryRequest,
	remoteMessageID string,
	replaceRemoteMessageID string,
) (bridgepkg.DeliveryAck, error) {
	ack := bridgepkg.DeliveryAck{
		DeliveryID:             req.Event.DeliveryID,
		Seq:                    req.Event.Seq,
		Outcome:                bridgepkg.DeliveryAckOutcomeSuccess,
		RemoteMessageID:        strings.TrimSpace(remoteMessageID),
		ReplaceRemoteMessageID: strings.TrimSpace(replaceRemoteMessageID),
	}
	if err := ack.ValidateFor(req.Event); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}
	return ack, nil
}

// ReportClassifiedError applies the recovery mapping for one provider failure
// and reports the resulting instance status transition through the Host API.
func (s *Session) ReportClassifiedError(
	ctx context.Context,
	bridgeInstanceID string,
	classified ClassifiedError,
) (*bridgepkg.BridgeInstance, RecoveryDecision, error) {
	if s == nil || s.host == nil {
		return nil, RecoveryDecision{}, errors.New("bridgesdk: runtime session host api is required")
	}

	recovery := classified.Recovery()
	if recovery.Status == "" {
		return nil, recovery, nil
	}

	updated, err := s.host.ReportBridgeInstanceState(ctx, bridgepkg.BridgesInstancesReportStateParams{
		BridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
		Status:           recovery.Status,
		Degradation:      recovery.Degradation,
	})
	if err != nil {
		return nil, recovery, err
	}
	return updated, recovery, nil
}
