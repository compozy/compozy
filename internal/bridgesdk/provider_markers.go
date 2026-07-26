package bridgesdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	AdapterHandshakePathEnv = "AGH_BRIDGE_ADAPTER_HANDSHAKE_PATH"
	AdapterOwnershipPathEnv = "AGH_BRIDGE_ADAPTER_OWNERSHIP_PATH"
	AdapterStatePathEnv     = "AGH_BRIDGE_ADAPTER_STATE_PATH"
	AdapterDeliveryPathEnv  = "AGH_BRIDGE_ADAPTER_DELIVERY_PATH"
	AdapterIngestPathEnv    = "AGH_BRIDGE_ADAPTER_INGEST_PATH"
	AdapterStartsPathEnv    = "AGH_BRIDGE_ADAPTER_STARTS_PATH"
	AdapterShutdownPathEnv  = "AGH_BRIDGE_ADAPTER_SHUTDOWN_PATH"
	AdapterCrashOncePathEnv = "AGH_BRIDGE_ADAPTER_CRASH_ONCE_PATH"
)

// InitializeMarker records the negotiated provider handshake for integration tests.
type InitializeMarker struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
}

// OwnershipMarker records provider-owned instance list/get observations.
type OwnershipMarker struct {
	Listed  []bridgepkg.BridgeInstance `json:"listed,omitempty"`
	Fetched []bridgepkg.BridgeInstance `json:"fetched,omitempty"`
	Error   string                     `json:"error,omitempty"`
}

// DeliveryMarker records one provider delivery request and acknowledgement.
type DeliveryMarker struct {
	PID     int                       `json:"pid"`
	Request bridgepkg.DeliveryRequest `json:"request"`
	Ack     *bridgepkg.DeliveryAck    `json:"ack,omitempty"`
	Error   string                    `json:"error,omitempty"`
}

// StateMarker records one provider-owned bridge state report.
type StateMarker struct {
	BridgeInstanceID string                   `json:"bridge_instance_id,omitempty"`
	Status           bridgepkg.BridgeStatus   `json:"status"`
	Instance         bridgepkg.BridgeInstance `json:"instance"`
	Error            string                   `json:"error,omitempty"`
}

// IngestMarker records one provider inbound envelope and daemon result.
type IngestMarker struct {
	Envelope bridgepkg.InboundMessageEnvelope      `json:"envelope"`
	Result   bridgepkg.BridgesMessagesIngestResult `json:"result"`
	Error    string                                `json:"error,omitempty"`
}

// AdapterMarkerPaths is the shared provider-harness marker destination set.
type AdapterMarkerPaths struct {
	HandshakePath string
	OwnershipPath string
	StatePath     string
	DeliveryPath  string
	IngestPath    string
	StartsPath    string
	ShutdownPath  string
	CrashOncePath string
}

// AdapterMarkerPathsFromProcess reads the provider-harness marker destinations.
func AdapterMarkerPathsFromProcess() AdapterMarkerPaths {
	return AdapterMarkerPaths{
		HandshakePath: strings.TrimSpace(os.Getenv(AdapterHandshakePathEnv)),
		OwnershipPath: strings.TrimSpace(os.Getenv(AdapterOwnershipPathEnv)),
		StatePath:     strings.TrimSpace(os.Getenv(AdapterStatePathEnv)),
		DeliveryPath:  strings.TrimSpace(os.Getenv(AdapterDeliveryPathEnv)),
		IngestPath:    strings.TrimSpace(os.Getenv(AdapterIngestPathEnv)),
		StartsPath:    strings.TrimSpace(os.Getenv(AdapterStartsPathEnv)),
		ShutdownPath:  strings.TrimSpace(os.Getenv(AdapterShutdownPathEnv)),
		CrashOncePath: strings.TrimSpace(os.Getenv(AdapterCrashOncePathEnv)),
	}
}

type adapterMarkerPaths struct {
	handshake string
	ownership string
	state     string
	delivery  string
	ingest    string
	starts    string
	shutdown  string
	crashOnce string
}

// AdapterMarkers centralizes the optional side-effect files consumed by integration harnesses.
type AdapterMarkers struct {
	provider string
	stderr   io.Writer
	paths    adapterMarkerPaths

	mu        sync.Mutex
	reportErr error
}

// NewAdapterMarkers resolves marker destinations from the current process environment.
func NewAdapterMarkers(provider string, stderr io.Writer) *AdapterMarkers {
	if stderr == nil {
		stderr = io.Discard
	}
	paths := AdapterMarkerPathsFromProcess()
	return &AdapterMarkers{
		provider: strings.TrimSpace(provider),
		stderr:   stderr,
		paths: adapterMarkerPaths{
			handshake: paths.HandshakePath,
			ownership: paths.OwnershipPath,
			state:     paths.StatePath,
			delivery:  paths.DeliveryPath,
			ingest:    paths.IngestPath,
			starts:    paths.StartsPath,
			shutdown:  paths.ShutdownPath,
			crashOnce: paths.CrashOncePath,
		},
	}
}

// RecordStart records one provider process start.
func (m *AdapterMarkers) RecordStart(pid int) {
	m.report(
		"write start marker",
		appendMarkerLine(
			m.path(func(p adapterMarkerPaths) string { return p.starts }),
			fmt.Sprintf("pid=%d", pid),
		),
	)
}

// RecordListen records the bound webhook address used by integration harnesses.
func (m *AdapterMarkers) RecordListen(address string) {
	m.report(
		"write start marker",
		appendMarkerLine(
			m.path(func(p adapterMarkerPaths) string { return p.starts }),
			"listen="+strings.TrimSpace(address),
		),
	)
}

// RecordInitialize records one provider initialize handshake.
func (m *AdapterMarkers) RecordInitialize(marker *InitializeMarker) {
	m.report(
		"write initialize marker",
		writeMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.handshake }), marker),
	)
}

// RecordOwnership records provider-owned instance list/get observations.
func (m *AdapterMarkers) RecordOwnership(marker OwnershipMarker) {
	m.report(
		"write ownership marker",
		writeMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.ownership }), marker),
	)
}

// RecordDelivery appends one provider delivery observation.
func (m *AdapterMarkers) RecordDelivery(marker DeliveryMarker) {
	m.report(
		"write delivery marker",
		appendMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.delivery }), marker),
	)
}

// RecordState appends one provider-owned state observation.
func (m *AdapterMarkers) RecordState(marker StateMarker) {
	m.report(
		"write state marker",
		appendMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.state }), marker),
	)
}

// RecordIngest appends one provider inbound observation.
func (m *AdapterMarkers) RecordIngest(marker IngestMarker) {
	m.report(
		"write ingest marker",
		appendMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.ingest }), marker),
	)
}

// RecordShutdown records one provider process shutdown.
func (m *AdapterMarkers) RecordShutdown(pid int) {
	m.report(
		"write shutdown marker",
		appendMarkerLine(
			m.path(func(p adapterMarkerPaths) string { return p.shutdown }),
			fmt.Sprintf("pid=%d", pid),
		),
	)
}

// ShouldCrashOnce reports whether the integration harness requested one process crash.
func (m *AdapterMarkers) ShouldCrashOnce() bool {
	target := m.path(func(p adapterMarkerPaths) string { return p.crashOnce })
	if target == "" {
		return false
	}
	_, err := os.Stat(target)
	return os.IsNotExist(err)
}

// RecordCrash writes the one-shot crash marker payload.
func (m *AdapterMarkers) RecordCrash(value any) {
	m.report(
		"write crash marker",
		writeMarkerJSON(m.path(func(p adapterMarkerPaths) string { return p.crashOnce }), value),
	)
}

// Error returns any secondary writer failure encountered while reporting marker errors.
func (m *AdapterMarkers) Error() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reportErr
}

// ReportError preserves provider side-effect diagnostics on the shared stderr writer.
func (m *AdapterMarkers) ReportError(action string, err error) {
	m.report(action, err)
}

func (m *AdapterMarkers) path(selectPath func(adapterMarkerPaths) string) string {
	if m == nil {
		return ""
	}
	return selectPath(m.paths)
}

func (m *AdapterMarkers) report(action string, err error) {
	if m == nil || err == nil {
		return
	}
	_, writeErr := fmt.Fprintf(
		m.stderr,
		"%s: %s: %v\n",
		m.provider,
		strings.TrimSpace(action),
		err,
	)
	if writeErr != nil {
		m.mu.Lock()
		m.reportErr = errors.Join(m.reportErr, writeErr)
		m.mu.Unlock()
	}
}

func appendMarkerLine(path string, line string) (err error) {
	file, ok, err := openMarkerFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY)
	if err != nil || !ok {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	_, err = fmt.Fprintln(file, strings.TrimSpace(line))
	return err
}

func appendMarkerJSON(path string, value any) (err error) {
	file, ok, err := openMarkerFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY)
	if err != nil || !ok {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeMarkerJSON(path string, value any) error {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("bridgesdk: create marker directory: %w", err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("bridgesdk: marshal marker: %w", err)
	}
	if err := os.WriteFile(target, payload, 0o600); err != nil {
		return fmt.Errorf("bridgesdk: write marker: %w", err)
	}
	return nil
}

func openMarkerFile(path string, flags int) (*os.File, bool, error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil, false, nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, false, fmt.Errorf("bridgesdk: create marker directory: %w", err)
	}
	file, err := os.OpenFile(target, flags, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("bridgesdk: open marker: %w", err)
	}
	return file, true, nil
}
