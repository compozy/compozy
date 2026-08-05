package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/bridgesdk"
	"github.com/compozy/compozy/internal/subprocess"
)

const (
	mainTelegramReferenceValue = "telegram-reference"
)

const adapterUpdatesEnv = "COMPOZY_BRIDGE_ADAPTER_UPDATES_PATH"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(1)
		}
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "serve" {
		return runServe(stdin, stdout, stderr)
	}
	return fmt.Errorf("telegram-reference: unsupported command %q", strings.TrimSpace(args[0]))
}

func runServe(stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	runtime, err := newTelegramReferenceRuntime(stderr)
	if err != nil {
		return err
	}
	return runtime.serve(stdin, stdout)
}

type adapterEnv struct {
	updatesPath string
}

func adapterEnvFromProcess() adapterEnv {
	return adapterEnv{
		updatesPath: strings.TrimSpace(os.Getenv(adapterUpdatesEnv)),
	}
}

type telegramReferenceRuntime struct {
	sdk       *bridgesdk.Runtime
	lifecycle *bridgesdk.ProviderLifecycle
	markers   *bridgesdk.AdapterMarkers
	now       func() time.Time
	env       adapterEnv

	mu         sync.Mutex
	deliveries map[string]deliveryState
}

type deliveryState struct {
	LastSeq         int64
	RemoteMessageID string
}

func newTelegramReferenceRuntime(stderr io.Writer) (*telegramReferenceRuntime, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	runtime := &telegramReferenceRuntime{
		markers:    bridgesdk.NewAdapterMarkers(mainTelegramReferenceValue, stderr),
		now:        func() time.Time { return time.Now().UTC() },
		env:        adapterEnvFromProcess(),
		deliveries: make(map[string]deliveryState),
	}
	lifecycle, err := bridgesdk.NewProviderLifecycle(bridgesdk.ProviderLifecycleConfig{
		ProviderName: mainTelegramReferenceValue,
		Markers:      runtime.markers,
		Reconcile:    runtime.reconcileManagedInstances,
	})
	if err != nil {
		return nil, err
	}
	runtime.lifecycle = lifecycle

	sdkRuntime, err := bridgesdk.NewRuntime(bridgesdk.RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    mainTelegramReferenceValue,
			Version: "0.1.0",
			SDKName: "bridgesdk",
		},
		Initialize: runtime.handleInitialize,
		Deliver:    runtime.handleBridgesDeliver,
		Progress:   runtime.handleBridgesProgress,
		Check:      runtime.handleBridgeCheck,
		HealthCheck: func(context.Context, *bridgesdk.Session) error {
			return runtime.lifecycle.Health()
		},
		Shutdown: runtime.lifecycle.Shutdown,
		Now:      runtime.now,
	})
	if err != nil {
		return nil, err
	}

	runtime.sdk = sdkRuntime
	return runtime, nil
}

func (r *telegramReferenceRuntime) serve(stdin io.Reader, stdout io.Writer) error {
	return r.lifecycle.Serve(context.Background(), r.sdk, stdin, stdout)
}

func (r *telegramReferenceRuntime) handleInitialize(
	ctx context.Context,
	session *bridgesdk.Session,
) error {
	if err := r.lifecycle.Initialize(ctx, session); err != nil {
		return err
	}
	if !r.lifecycle.Go(func() { r.pollInboundUpdates(session) }) {
		return bridgesdk.ErrProviderStopped
	}
	return nil
}

func (r *telegramReferenceRuntime) reconcileManagedInstances(
	_ context.Context,
	managed []subprocess.InitializeBridgeManagedInstance,
) ([]bridgesdk.ProviderInitialState, error) {
	states := make([]bridgesdk.ProviderInitialState, 0, len(managed))
	for _, item := range managed {
		states = append(states, bridgesdk.ProviderInitialState{
			BridgeInstanceID: item.Instance.ID,
			Status:           bridgeStatusForManaged(item),
		})
	}
	return states, nil
}

func (r *telegramReferenceRuntime) pollInboundUpdates(
	session *bridgesdk.Session,
) {
	updatesPath := strings.TrimSpace(r.env.updatesPath)
	if updatesPath == "" {
		return
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	processed := 0
	for {
		select {
		case <-r.lifecycle.StopChannel():
			return
		case <-ticker.C:
			payload, err := os.ReadFile(updatesPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				r.lifecycle.SetError(err)
				continue
			}

			lines := nonEmptyLines(string(payload))
			for processed < len(lines) {
				var update telegramUpdate
				if err := json.Unmarshal([]byte(lines[processed]), &update); err != nil {
					parseErr := fmt.Errorf(
						"telegram-reference: decode update line %d: %w",
						processed+1,
						err,
					)
					r.lifecycle.SetError(parseErr)
					r.markers.RecordIngest(bridgesdk.IngestMarker{Error: parseErr.Error()})
					processed++
					continue
				}
				if err := r.ingestTelegramUpdate(r.lifecycle.Context(), session, update); err != nil {
					r.lifecycle.SetError(err)
				} else {
					r.lifecycle.ClearError()
				}
				processed++
			}
		}
	}
}

func (r *telegramReferenceRuntime) ingestTelegramUpdate(
	ctx context.Context,
	session *bridgesdk.Session,
	update telegramUpdate,
) error {
	if ctx == nil {
		return errors.New("telegram-reference: ingest context is required")
	}
	managed, err := resolveManagedInstance(session, update.BridgeInstanceID)
	if err != nil {
		r.markers.RecordIngest(bridgesdk.IngestMarker{
			Envelope: bridgepkg.InboundMessageEnvelope{BridgeInstanceID: strings.TrimSpace(update.BridgeInstanceID)},
			Error:    err.Error(),
		})
		return err
	}

	envelope, err := mapTelegramUpdate(update, *managed, r.now)
	if err != nil {
		r.markers.RecordIngest(bridgesdk.IngestMarker{
			Envelope: bridgepkg.InboundMessageEnvelope{BridgeInstanceID: managed.Instance.ID},
			Error:    err.Error(),
		})
		return err
	}

	_, err = r.lifecycle.Host().IngestBridgeMessage(ctx, session, envelope)
	if err != nil {
		return err
	}
	return nil
}

func (r *telegramReferenceRuntime) ackDelivery(request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
	if err := request.Validate(); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}

	event := request.Event
	instanceID := strings.TrimSpace(event.BridgeInstanceID)
	deliveryID := strings.TrimSpace(event.DeliveryID)
	key := deliveryStateKey(instanceID, deliveryID)

	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.deliveries[key]
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume && request.Snapshot != nil {
		state.LastSeq = request.Snapshot.LastAckedSeq
		state.RemoteMessageID = strings.TrimSpace(request.Snapshot.RemoteMessageID)
	}

	if normalizeDeliveryEventType(event.EventType) != bridgepkg.DeliveryEventTypeResume && event.Seq <= state.LastSeq {
		return bridgepkg.DeliveryAck{}, fmt.Errorf(
			"telegram-reference: out-of-order delivery seq %d after %d",
			event.Seq,
			state.LastSeq,
		)
	}

	remoteID := state.RemoteMessageID
	if normalizeDeliveryEventType(event.EventType) != bridgepkg.DeliveryEventTypeResume || remoteID == "" {
		remoteID = remoteMessageID(instanceID, deliveryID, event.Seq)
	}

	ack := bridgepkg.DeliveryAck{
		DeliveryID:      deliveryID,
		Seq:             event.Seq,
		RemoteMessageID: remoteID,
	}
	if state.RemoteMessageID != "" && state.RemoteMessageID != remoteID {
		ack.ReplaceRemoteMessageID = state.RemoteMessageID
	}
	if normalizeDeliveryEventType(event.EventType) == bridgepkg.DeliveryEventTypeResume && request.Snapshot != nil {
		if ack.RemoteMessageID == "" {
			ack.RemoteMessageID = strings.TrimSpace(request.Snapshot.RemoteMessageID)
		}
		if ack.ReplaceRemoteMessageID == "" {
			ack.ReplaceRemoteMessageID = strings.TrimSpace(request.Snapshot.ReplaceRemoteMessageID)
		}
	}

	state.LastSeq = event.Seq
	if ack.RemoteMessageID != "" {
		state.RemoteMessageID = ack.RemoteMessageID
	}
	r.deliveries[key] = state

	return ack, nil
}

func resolveManagedInstance(
	session *bridgesdk.Session,
	bridgeInstanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("telegram-reference: managed bridge cache is required")
	}

	trimmedID := strings.TrimSpace(bridgeInstanceID)
	if trimmedID != "" {
		managed, ok := session.Cache().Get(trimmedID)
		if !ok || managed == nil {
			return nil, fmt.Errorf(
				"telegram-reference: managed bridge instance %q is not owned by this runtime",
				trimmedID,
			)
		}
		return managed, nil
	}

	managed := session.Cache().List()
	switch len(managed) {
	case 0:
		return nil, errors.New("telegram-reference: provider runtime does not own any bridge instances")
	case 1:
		only := managed[0]
		return &only, nil
	default:
		return nil, errors.New(
			"telegram-reference: bridge_instance_id is required when provider owns multiple bridge instances",
		)
	}
}

func bridgeStatusForManaged(managed subprocess.InitializeBridgeManagedInstance) bridgepkg.BridgeStatus {
	value, ok := boundSecretValue(managed, "bot_token")
	if !ok || strings.TrimSpace(value) == "" {
		return bridgepkg.BridgeStatusAuthRequired
	}
	return bridgepkg.BridgeStatusReady
}

func boundSecretValue(bridgeRuntime subprocess.InitializeBridgeManagedInstance, bindingName string) (string, bool) {
	trimmed := strings.TrimSpace(bindingName)
	if trimmed == "" {
		return "", false
	}
	for _, secret := range bridgeRuntime.BoundSecrets {
		if strings.TrimSpace(secret.BindingName) == trimmed {
			return strings.TrimSpace(secret.Value), strings.TrimSpace(secret.Value) != ""
		}
	}
	return "", false
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func remoteMessageID(instanceID string, deliveryID string, seq int64) string {
	return fmt.Sprintf("telegram:%s:%s:%d", strings.TrimSpace(instanceID), strings.TrimSpace(deliveryID), seq)
}

func normalizeDeliveryEventType(eventType bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(eventType))))
}

func nonEmptyLines(input string) []string {
	lines := strings.Split(input, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}
