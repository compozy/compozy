package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/fileutil"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	mainTelegramReferenceValue = "telegram-reference"
)

const (
	adapterHandshakeEnv = "AGH_BRIDGE_ADAPTER_HANDSHAKE_PATH"
	adapterOwnershipEnv = "AGH_BRIDGE_ADAPTER_OWNERSHIP_PATH"
	adapterStateEnv     = "AGH_BRIDGE_ADAPTER_STATE_PATH"
	adapterDeliveryEnv  = "AGH_BRIDGE_ADAPTER_DELIVERY_PATH"
	adapterIngestEnv    = "AGH_BRIDGE_ADAPTER_INGEST_PATH"
	adapterUpdatesEnv   = "AGH_BRIDGE_ADAPTER_UPDATES_PATH"
	adapterStartsEnv    = "AGH_BRIDGE_ADAPTER_STARTS_PATH"
	adapterShutdownEnv  = "AGH_BRIDGE_ADAPTER_SHUTDOWN_PATH"
	adapterCrashOnceEnv = "AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH"

	rpcCodeNotInitialized = -32003
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
	reportSideEffectError(
		stderr,
		"write start marker",
		appendMarkerLine(os.Getenv(adapterStartsEnv), fmt.Sprintf("pid=%d", os.Getpid())),
	)

	runtime, err := newTelegramReferenceRuntime(stderr)
	if err != nil {
		return err
	}
	return runtime.serve(context.Background(), stdin, stdout)
}

type adapterEnv struct {
	handshakePath string
	ownershipPath string
	statePath     string
	deliveryPath  string
	ingestPath    string
	updatesPath   string
	startsPath    string
	shutdownPath  string
	crashOncePath string
}

type sideEffectPath struct {
	root string
	name string
}

func (p sideEffectPath) isZero() bool {
	return p.root == "" && p.name == ""
}

func (p sideEffectPath) fullPath() string {
	if p.isZero() {
		return ""
	}
	return filepath.Join(p.root, p.name)
}

var sideEffectSnapshots = struct {
	mu      sync.Mutex
	payload map[sideEffectPath][]byte
}{
	payload: make(map[sideEffectPath][]byte),
}

func adapterEnvFromProcess() adapterEnv {
	return adapterEnv{
		handshakePath: strings.TrimSpace(os.Getenv(adapterHandshakeEnv)),
		ownershipPath: strings.TrimSpace(os.Getenv(adapterOwnershipEnv)),
		statePath:     strings.TrimSpace(os.Getenv(adapterStateEnv)),
		deliveryPath:  strings.TrimSpace(os.Getenv(adapterDeliveryEnv)),
		ingestPath:    strings.TrimSpace(os.Getenv(adapterIngestEnv)),
		updatesPath:   strings.TrimSpace(os.Getenv(adapterUpdatesEnv)),
		startsPath:    strings.TrimSpace(os.Getenv(adapterStartsEnv)),
		shutdownPath:  strings.TrimSpace(os.Getenv(adapterShutdownEnv)),
		crashOncePath: strings.TrimSpace(os.Getenv(adapterCrashOnceEnv)),
	}
}

type telegramReferenceRuntime struct {
	sdk    *bridgesdk.Runtime
	stderr io.Writer
	now    func() time.Time
	env    adapterEnv

	mu         sync.RWMutex
	lastError  string
	deliveries map[string]deliveryState

	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	stopCh          chan struct{}
	stopOnce        sync.Once
	// wg joins background workers during test/process cleanup; shutdown ACK only signals stop.
	wg sync.WaitGroup
}

type deliveryState struct {
	LastSeq         int64
	RemoteMessageID string
}

type initializeMarker struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
}

type ownershipMarker struct {
	Listed  []bridgepkg.BridgeInstance `json:"listed,omitempty"`
	Fetched []bridgepkg.BridgeInstance `json:"fetched,omitempty"`
	Error   string                     `json:"error,omitempty"`
}

type deliveryMarker struct {
	PID     int                       `json:"pid"`
	Request bridgepkg.DeliveryRequest `json:"request"`
	Ack     *bridgepkg.DeliveryAck    `json:"ack,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

type stateMarker struct {
	BridgeInstanceID string                   `json:"bridge_instance_id,omitempty"`
	Status           bridgepkg.BridgeStatus   `json:"status"`
	Instance         bridgepkg.BridgeInstance `json:"instance"`
	Error            string                   `json:"error,omitempty"`
}

type ingestMarker struct {
	Envelope bridgepkg.InboundMessageEnvelope      `json:"envelope"`
	Result   bridgepkg.BridgesMessagesIngestResult `json:"result"`
	Error    string                                `json:"error,omitempty"`
}

func newTelegramReferenceRuntime(stderr io.Writer) (*telegramReferenceRuntime, error) {
	if stderr == nil {
		stderr = io.Discard
	}

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	runtime := &telegramReferenceRuntime{
		stderr:          stderr,
		now:             func() time.Time { return time.Now().UTC() },
		env:             adapterEnvFromProcess(),
		deliveries:      make(map[string]deliveryState),
		lifecycleCtx:    lifecycleCtx,
		lifecycleCancel: lifecycleCancel,
		stopCh:          make(chan struct{}),
	}

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
			return runtime.healthCheck()
		},
		Shutdown: runtime.handleShutdown,
		Now:      runtime.now,
	})
	if err != nil {
		return nil, err
	}

	runtime.sdk = sdkRuntime
	return runtime, nil
}

func (r *telegramReferenceRuntime) serve(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	return r.sdk.Serve(ctx, stdin, stdout)
}

func (r *telegramReferenceRuntime) handleInitialize(_ context.Context, session *bridgesdk.Session) error {
	marker := initializeMarker{
		Request:  session.InitializeRequest(),
		Response: session.InitializeResponse(),
	}
	r.reportSideEffectError("write initialize marker", writeJSONFile(r.env.handshakePath, marker))
	r.clearLastError()

	r.wg.Add(2)
	go func() {
		defer r.wg.Done()
		r.afterInitialize(r.lifecycleCtx, session)
	}()
	go func() {
		defer r.wg.Done()
		r.pollInboundUpdates(r.lifecycleCtx, session)
	}()

	return nil
}

func (r *telegramReferenceRuntime) afterInitialize(
	lifecycleCtx context.Context,
	session *bridgesdk.Session,
) {
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(lifecycleCtx, 5*time.Second)
	defer cancel()

	listed, err := r.syncOwnedInstances(ctx, session)
	fetched := make([]bridgepkg.BridgeInstance, 0)
	ownershipErr := err
	if ownershipErr == nil {
		for _, managed := range listed {
			instance, getErr := r.getOwnedInstance(ctx, session, managed.Instance.ID)
			if getErr != nil {
				ownershipErr = getErr
				break
			}
			fetched = append(fetched, *instance)
		}
	}
	if len(listed) == 0 {
		listed = session.Cache().List()
	}
	ownership := ownershipMarker{
		Listed:  managedInstancesToInstances(listed),
		Fetched: fetched,
	}
	if ownershipErr != nil {
		ownership.Error = ownershipErr.Error()
		r.setLastError(ownershipErr)
	}
	r.reportSideEffectError("write ownership marker", writeJSONFile(r.env.ownershipPath, ownership))

	var lastErr error
	for _, managed := range session.Cache().List() {
		status := bridgeStatusForManaged(session, managed.Instance.ID)
		if _, reportErr := r.reportState(ctx, session, managed.Instance.ID, status); reportErr != nil {
			lastErr = reportErr
		}
	}

	if lastErr != nil {
		r.setLastError(lastErr)
		return
	}
	if ownershipErr == nil {
		r.clearLastError()
	}
}

func (r *telegramReferenceRuntime) handleShutdown(
	_ context.Context,
	_ *bridgesdk.Session,
	_ subprocess.ShutdownRequest,
) error {
	r.stop()
	r.reportSideEffectError(
		"write shutdown marker",
		appendMarkerLine(r.env.shutdownPath, fmt.Sprintf("pid=%d", os.Getpid())),
	)
	return nil
}

func (r *telegramReferenceRuntime) stop() {
	r.stopOnce.Do(func() {
		if r.lifecycleCancel != nil {
			r.lifecycleCancel()
		}
		close(r.stopCh)
	})
}

func (r *telegramReferenceRuntime) pollInboundUpdates(
	lifecycleCtx context.Context,
	session *bridgesdk.Session,
) {
	if lifecycleCtx == nil {
		lifecycleCtx = context.Background()
	}
	updatesPath := strings.TrimSpace(r.env.updatesPath)
	if updatesPath == "" {
		return
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	processed := 0
	for {
		select {
		case <-lifecycleCtx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			payload, err := os.ReadFile(updatesPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				r.setLastError(err)
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
					r.setLastError(parseErr)
					r.reportSideEffectError(
						"write failed ingest marker",
						appendJSONLine(r.env.ingestPath, ingestMarker{Error: parseErr.Error()}),
					)
					processed++
					continue
				}
				if err := r.ingestTelegramUpdate(lifecycleCtx, session, update); err != nil {
					r.setLastError(err)
				} else {
					r.clearLastError()
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
		ctx = context.Background()
	}
	managed, err := resolveManagedInstance(session, update.BridgeInstanceID)
	if err != nil {
		r.reportSideEffectError("write failed ingest marker", appendJSONLine(r.env.ingestPath, ingestMarker{
			Envelope: bridgepkg.InboundMessageEnvelope{BridgeInstanceID: strings.TrimSpace(update.BridgeInstanceID)},
			Error:    err.Error(),
		}))
		return err
	}

	envelope, err := mapTelegramUpdate(update, *managed, r.now)
	if err != nil {
		r.reportSideEffectError("write failed ingest marker", appendJSONLine(r.env.ingestPath, ingestMarker{
			Envelope: bridgepkg.InboundMessageEnvelope{BridgeInstanceID: managed.Instance.ID},
			Error:    err.Error(),
		}))
		return err
	}

	result, err := r.ingestBridgeMessage(ctx, session, envelope)
	if err != nil {
		r.reportSideEffectError("write failed ingest marker", appendJSONLine(r.env.ingestPath, ingestMarker{
			Envelope: envelope,
			Error:    err.Error(),
		}))
		return err
	}

	r.reportSideEffectError("write ingest marker", appendJSONLine(r.env.ingestPath, ingestMarker{
		Envelope: envelope,
		Result:   *result,
	}))
	return nil
}

func (r *telegramReferenceRuntime) syncOwnedInstances(
	ctx context.Context,
	session *bridgesdk.Session,
) ([]subprocess.InitializeBridgeManagedInstance, error) {
	var result []subprocess.InitializeBridgeManagedInstance
	err := r.retryHostCall(ctx, func(callCtx context.Context) error {
		items, callErr := session.SyncInstances(callCtx)
		if callErr == nil {
			result = items
		}
		return callErr
	})
	return result, err
}

func (r *telegramReferenceRuntime) getOwnedInstance(
	ctx context.Context,
	session *bridgesdk.Session,
	bridgeInstanceID string,
) (*bridgepkg.BridgeInstance, error) {
	var result *bridgepkg.BridgeInstance
	err := r.retryHostCall(ctx, func(callCtx context.Context) error {
		instance, callErr := session.HostAPI().GetBridgeInstance(callCtx, bridgeInstanceID)
		if callErr == nil {
			result = instance
		}
		return callErr
	})
	return result, err
}

func (r *telegramReferenceRuntime) reportState(
	ctx context.Context,
	session *bridgesdk.Session,
	bridgeInstanceID string,
	status bridgepkg.BridgeStatus,
) (*bridgepkg.BridgeInstance, error) {
	var result *bridgepkg.BridgeInstance
	err := r.retryHostCall(ctx, func(callCtx context.Context) error {
		instance, callErr := session.HostAPI().
			ReportBridgeInstanceState(callCtx, bridgepkg.BridgesInstancesReportStateParams{
				BridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
				Status:           status,
			})
		if callErr == nil {
			result = instance
		}
		return callErr
	})
	if err != nil {
		r.reportSideEffectError("write failed state marker", appendJSONLine(r.env.statePath, stateMarker{
			BridgeInstanceID: strings.TrimSpace(bridgeInstanceID),
			Status:           status,
			Error:            err.Error(),
		}))
		return nil, err
	}

	r.reportSideEffectError("write state marker", appendJSONLine(r.env.statePath, stateMarker{
		BridgeInstanceID: result.ID,
		Status:           result.Status,
		Instance:         *result,
	}))
	return result, nil
}

func (r *telegramReferenceRuntime) ingestBridgeMessage(
	ctx context.Context,
	session *bridgesdk.Session,
	envelope bridgepkg.InboundMessageEnvelope,
) (*bridgepkg.BridgesMessagesIngestResult, error) {
	var result *bridgepkg.BridgesMessagesIngestResult
	err := r.retryHostCall(ctx, func(callCtx context.Context) error {
		ingestResult, callErr := session.HostAPI().IngestBridgeMessage(callCtx, envelope)
		if callErr == nil {
			result = ingestResult
		}
		return callErr
	})
	return result, err
}

func (r *telegramReferenceRuntime) retryHostCall(ctx context.Context, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	delay := 10 * time.Millisecond
	var lastErr error
	for range 6 {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		if !isNotInitializedRPCError(err) {
			return err
		}

		lastErr = err
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-r.stopCh:
			if !timer.Stop() {
				<-timer.C
			}
			return err
		case <-timer.C:
		}

		if delay < 100*time.Millisecond {
			delay *= 2
			if delay > 100*time.Millisecond {
				delay = 100 * time.Millisecond
			}
		}
	}

	if lastErr != nil {
		return lastErr
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

func (r *telegramReferenceRuntime) setLastError(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastError = err.Error()
}

func (r *telegramReferenceRuntime) clearLastError() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastError = ""
}

func (r *telegramReferenceRuntime) reportSideEffectError(action string, err error) {
	reportSideEffectError(r.stderr, action, err)
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

func bridgeStatusForManaged(session *bridgesdk.Session, bridgeInstanceID string) bridgepkg.BridgeStatus {
	if session == nil || session.Cache() == nil {
		return bridgepkg.BridgeStatusError
	}
	value, ok := session.Cache().BoundSecretValue(bridgeInstanceID, "bot_token")
	if !ok || strings.TrimSpace(value) == "" {
		return bridgepkg.BridgeStatusAuthRequired
	}
	return bridgepkg.BridgeStatusReady
}

func managedInstancesToInstances(items []subprocess.InitializeBridgeManagedInstance) []bridgepkg.BridgeInstance {
	instances := make([]bridgepkg.BridgeInstance, 0, len(items))
	for _, item := range items {
		instances = append(instances, item.Instance)
	}
	return instances
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

func isNotInitializedRPCError(err error) bool {
	if err == nil {
		return false
	}
	var rpcErr *subprocess.RPCError
	if !errors.As(err, &rpcErr) {
		return false
	}
	return rpcErr.Code == rpcCodeNotInitialized ||
		strings.EqualFold(strings.TrimSpace(rpcErr.Message), "Not initialized")
}

func shouldCrashOnce(path string) bool {
	target, err := safeSideEffectPath(path)
	if err != nil || target.isZero() {
		return false
	}
	_, err = os.Stat(target.fullPath())
	return errors.Is(err, os.ErrNotExist)
}

func appendMarkerLine(path string, line string) error {
	return appendSideEffectPayload(path, []byte(strings.TrimSpace(line)+"\n"))
}

func appendJSONLine(path string, value any) error {
	var payload bytes.Buffer
	encoder := json.NewEncoder(&payload)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return err
	}

	return appendSideEffectPayload(path, payload.Bytes())
}

func appendSideEffectPayload(path string, payload []byte) error {
	target, err := safeSideEffectPath(path)
	if err != nil {
		return err
	}
	if target.isZero() {
		return nil
	}

	return writeSideEffectSnapshot(target, func(current []byte) []byte {
		return append(current, payload...)
	})
}

func writeJSONFile(path string, value any) error {
	target, err := safeSideEffectPath(path)
	if err != nil {
		return err
	}
	if target.isZero() {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeSideEffectSnapshot(target, func([]byte) []byte {
		return append([]byte(nil), payload...)
	})
}

func writeSideEffectFile(path sideEffectPath, payload []byte) error {
	if err := ensureSideEffectDir(path); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path.fullPath(), payload, 0o600)
}

func writeSideEffectSnapshot(path sideEffectPath, update func([]byte) []byte) error {
	sideEffectSnapshots.mu.Lock()
	defer sideEffectSnapshots.mu.Unlock()

	current, ok := sideEffectSnapshots.payload[path]
	if !ok {
		diskPayload, readErr := readSideEffectFile(path)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		current = diskPayload
	}
	current = append([]byte(nil), current...)
	next := update(current)
	if err := writeSideEffectFile(path, next); err != nil {
		return err
	}
	sideEffectSnapshots.payload[path] = append([]byte(nil), next...)
	return nil
}

func readSideEffectFile(path sideEffectPath) (payload []byte, err error) {
	root, err := os.OpenRoot(path.root)
	if err != nil {
		return nil, fmt.Errorf("open side-effect root %q: %w", path.root, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close side-effect root %q: %w", path.root, closeErr))
		}
	}()

	file, err := root.Open(path.name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close side-effect file %q: %w", path.fullPath(), closeErr))
		}
	}()
	payload, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read side-effect file %q: %w", path.fullPath(), err)
	}
	return payload, nil
}

func ensureSideEffectDir(path sideEffectPath) (err error) {
	dir := filepath.Dir(path.name)
	if dir == "." {
		return nil
	}

	root, err := os.OpenRoot(path.root)
	if err != nil {
		return fmt.Errorf("open side-effect root %q: %w", path.root, err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close side-effect root %q: %w", path.root, closeErr))
		}
	}()

	if err := root.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create side-effect directory for %q: %w", path.fullPath(), err)
	}
	return nil
}

func safeSideEffectPath(path string) (sideEffectPath, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return sideEffectPath{}, nil
	}

	cleaned := filepath.Clean(target)
	if !filepath.IsAbs(cleaned) {
		return sideEffectPath{}, fmt.Errorf("side-effect path %q must be absolute", target)
	}
	tempRoot := filepath.Clean(os.TempDir())
	relativeToTemp, err := filepath.Rel(tempRoot, cleaned)
	if err != nil {
		return sideEffectPath{}, fmt.Errorf("resolve side-effect path %q: %w", target, err)
	}
	if relativeToTemp == "." ||
		relativeToTemp == ".." ||
		filepath.IsAbs(relativeToTemp) ||
		strings.HasPrefix(relativeToTemp, ".."+string(os.PathSeparator)) {
		return sideEffectPath{}, fmt.Errorf("side-effect path %q must stay under %q", target, tempRoot)
	}
	return sideEffectPath{root: tempRoot, name: relativeToTemp}, nil
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

func reportSideEffectError(writer io.Writer, action string, err error) {
	if err == nil || writer == nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "telegram-reference: %s: %v\n", strings.TrimSpace(action), err)
}
