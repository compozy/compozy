package network

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/agh/internal/store"
)

const (
	// AuditDirectionSent records a successful outbound publish.
	AuditDirectionSent = "sent"
	// AuditDirectionReceived records an accepted inbound delivery.
	AuditDirectionReceived = "received"
	// AuditDirectionRejected records a rejected envelope.
	AuditDirectionRejected = "rejected"
	// AuditDirectionDelivered records a completed local delivery.
	AuditDirectionDelivered = "delivered"
)

// AuditStore is the persistence surface consumed by the network audit writer.
type AuditStore interface {
	WriteNetworkAudit(ctx context.Context, entry store.NetworkAuditEntry) error
}

// AuditWriter records network activity into the configured sinks.
type AuditWriter interface {
	RecordSent(ctx context.Context, sessionID string, envelope Envelope) error
	RecordReceived(ctx context.Context, sessionID string, envelope Envelope) error
	RecordRejected(ctx context.Context, sessionID string, envelope Envelope, reason string) error
	RecordDelivered(ctx context.Context, sessionID string, envelope Envelope) error
}

// TaskIngressAudit captures one task-domain ingress decision originating from a
// validated network peer.
type TaskIngressAudit struct {
	Action      string
	Direction   string
	PeerID      string
	WorkspaceID string
	Channel     string
	RequestID   string
	Reason      string
	Payload     any
}

// TaskIngressAuditWriter is the optional audit extension used by task-aware
// network ingress. Existing protocol-message auditing remains unchanged.
type TaskIngressAuditWriter interface {
	RecordTaskIngress(ctx context.Context, audit TaskIngressAudit) error
}

// FileAuditWriter writes normalized network audit records to a JSONL file and
// optionally mirrors them into a persistent store.
type FileAuditWriter struct {
	path  string
	store AuditStore
	now   func() time.Time

	fileMu sync.Mutex
}

type AuditWriterOption func(*FileAuditWriter)

// NewAuditWriter constructs the dual-path network audit writer.
func NewAuditWriter(path string, auditStore AuditStore, opts ...AuditWriterOption) (*FileAuditWriter, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" && auditStore == nil {
		return nil, errors.New("network: audit sink is required")
	}

	writer := &FileAuditWriter{
		path:  cleanPath,
		store: auditStore,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(writer)
		}
	}
	return writer, nil
}

var _ AuditWriter = (*FileAuditWriter)(nil)
var _ TaskIngressAuditWriter = (*FileAuditWriter)(nil)

// RecordSent stores a sent network audit record.
func (w *FileAuditWriter) RecordSent(ctx context.Context, sessionID string, envelope Envelope) error {
	return w.record(ctx, sessionID, AuditDirectionSent, envelope, "")
}

// RecordCommittedSent appends the post-commit sent record only to the file
// sink because atomic conversation acceptance already owns the durable row.
func (w *FileAuditWriter) RecordCommittedSent(
	ctx context.Context,
	sessionID string,
	envelope Envelope,
) error {
	if ctx == nil {
		return errors.New("network: audit context is required")
	}
	if w == nil {
		return errors.New("network: audit writer is required")
	}
	if w.path == "" {
		return nil
	}
	entry, err := NormalizeAuditEntry(sessionID, AuditDirectionSent, envelope, "", w.now())
	if err != nil {
		return err
	}
	return w.appendFile(entry)
}

// RecordReceived stores a received network audit record.
func (w *FileAuditWriter) RecordReceived(ctx context.Context, sessionID string, envelope Envelope) error {
	return w.record(ctx, sessionID, AuditDirectionReceived, envelope, "")
}

// RecordRejected stores a rejected network audit record.
func (w *FileAuditWriter) RecordRejected(
	ctx context.Context,
	sessionID string,
	envelope Envelope,
	reason string,
) error {
	return w.record(ctx, sessionID, AuditDirectionRejected, envelope, reason)
}

// RecordDelivered stores a delivered network audit record.
func (w *FileAuditWriter) RecordDelivered(ctx context.Context, sessionID string, envelope Envelope) error {
	return w.record(ctx, sessionID, AuditDirectionDelivered, envelope, "")
}

// RecordTaskIngress stores one accepted or rejected task-ingress audit record
// using the existing network audit sinks.
func (w *FileAuditWriter) RecordTaskIngress(ctx context.Context, audit TaskIngressAudit) error {
	if w == nil {
		return errors.New("network: audit writer is required")
	}
	if ctx == nil {
		return errors.New("network: audit context is required")
	}
	if w.path == "" && w.store == nil {
		return errors.New("network: audit sink is required")
	}

	now := w.now
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}

	entry, err := normalizeTaskIngressAuditEntry(audit, now())
	if err != nil {
		return fmt.Errorf("network: normalize task ingress audit entry: %w", err)
	}

	var recordErr error
	if w.path != "" {
		if err := w.appendFile(entry); err != nil {
			recordErr = errors.Join(recordErr, fmt.Errorf("network: append file audit entry: %w", err))
		}
	}
	if w.store != nil {
		if err := w.store.WriteNetworkAudit(ctx, entry); err != nil {
			recordErr = errors.Join(recordErr, fmt.Errorf("network: persist audit entry: %w", err))
		}
	}

	return recordErr
}

func (w *FileAuditWriter) record(
	ctx context.Context,
	sessionID string,
	direction string,
	envelope Envelope,
	reason string,
) error {
	if ctx == nil {
		return errors.New("network: audit context is required")
	}
	if w == nil {
		return errors.New("network: audit writer is required")
	}

	entry, err := NormalizeAuditEntry(sessionID, direction, envelope, reason, w.now())
	if err != nil {
		return err
	}

	var recordErr error
	if w.path != "" {
		recordErr = errors.Join(recordErr, w.appendFile(entry))
	}
	var auditWriteErr error
	if w.store != nil {
		auditWriteErr = w.store.WriteNetworkAudit(ctx, entry)
		recordErr = errors.Join(recordErr, auditWriteErr)
	}

	return recordErr
}

// NormalizeAuditEntry derives a consistent audit row from envelope metadata.
func NormalizeAuditEntry(
	sessionID string,
	direction string,
	envelope Envelope,
	reason string,
	at time.Time,
) (store.NetworkAuditEntry, error) {
	canonicalEnvelope, err := json.Marshal(envelope)
	if err != nil {
		return store.NetworkAuditEntry{}, fmt.Errorf("network: marshal audit envelope: %w", err)
	}

	peerTo := ""
	if envelope.To != nil {
		peerTo = strings.TrimSpace(*envelope.To)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	entry := store.NetworkAuditEntry{
		ID:          store.NewID("naud"),
		SessionID:   strings.TrimSpace(sessionID),
		WorkspaceID: strings.TrimSpace(envelope.WorkspaceID),
		Direction:   strings.TrimSpace(direction),
		Kind:        strings.TrimSpace(string(envelope.Kind)),
		Channel:     strings.TrimSpace(envelope.Channel),
		Surface:     trimmedSurfaceValue(envelope.Surface),
		ThreadID:    trimmedPointerValue(envelope.ThreadID),
		DirectID:    trimmedPointerValue(envelope.DirectID),
		WorkID:      trimmedPointerValue(envelope.WorkID),
		PeerFrom:    strings.TrimSpace(envelope.From),
		PeerTo:      peerTo,
		MessageID:   strings.TrimSpace(envelope.ID),
		Reason:      strings.TrimSpace(reason),
		Size:        len(canonicalEnvelope),
		Timestamp:   at.UTC(),
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkAuditEntry{}, err
	}

	return entry, nil
}

func normalizeTimelineMessageEntry(
	sessionID string,
	direction string,
	envelope Envelope,
	at time.Time,
) (store.NetworkMessageEntry, bool, error) {
	switch strings.TrimSpace(direction) {
	case AuditDirectionSent, AuditDirectionReceived:
	default:
		return store.NetworkMessageEntry{}, false, nil
	}

	body, err := envelope.DecodeBody()
	if err != nil {
		return store.NetworkMessageEntry{}, false, fmt.Errorf("network: decode timeline envelope body: %w", err)
	}
	extJSON, err := timelineExtensionJSON(envelope.Ext)
	if err != nil {
		return store.NetworkMessageEntry{}, false, err
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	peerTo := ""
	if envelope.To != nil {
		peerTo = strings.TrimSpace(*envelope.To)
	}
	entry := store.NetworkMessageEntry{
		MessageID:   strings.TrimSpace(envelope.ID),
		SessionID:   strings.TrimSpace(sessionID),
		WorkspaceID: strings.TrimSpace(envelope.WorkspaceID),
		Channel:     strings.TrimSpace(envelope.Channel),
		Surface:     trimmedSurfaceValue(envelope.Surface),
		ThreadID:    trimmedPointerValue(envelope.ThreadID),
		DirectID:    trimmedPointerValue(envelope.DirectID),
		Direction:   strings.TrimSpace(direction),
		PeerFrom:    strings.TrimSpace(envelope.From),
		PeerTo:      peerTo,
		Mentions:    normalizeEnvelopeMentions(envelope.Mentions),
		Kind:        strings.TrimSpace(string(envelope.Kind)),
		PreviewText: previewForBody(body),
		Body:        cloneRawMessage(envelope.Body),
		Timestamp:   at.UTC(),
		WorkID:      trimmedPointerValue(envelope.WorkID),
		ReplyTo:     trimmedPointerValue(envelope.ReplyTo),
		TraceID:     trimmedPointerValue(envelope.TraceID),
		CausationID: trimmedPointerValue(envelope.CausationID),
	}
	if value, ok := body.(SayBody); ok {
		entry.Intent = strings.TrimSpace(value.Intent)
		entry.Text = value.Text
	}
	entry.ExtJSON = extJSON
	if err := entry.Validate(); err != nil {
		return store.NetworkMessageEntry{}, false, err
	}
	return entry, true, nil
}

func timelineExtensionJSON(ext ExtensionMap) (json.RawMessage, error) {
	if len(ext) == 0 {
		return json.RawMessage("{}"), nil
	}
	raw, err := json.Marshal(ext)
	if err != nil {
		return nil, fmt.Errorf("network: marshal timeline envelope extensions: %w", err)
	}
	return raw, nil
}

func normalizeTaskIngressAuditEntry(audit TaskIngressAudit, at time.Time) (store.NetworkAuditEntry, error) {
	payloadSize := 0
	if audit.Payload != nil {
		payload, err := json.Marshal(audit.Payload)
		if err != nil {
			return store.NetworkAuditEntry{}, fmt.Errorf("network: marshal task ingress audit payload: %w", err)
		}
		payloadSize = len(payload)
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}

	entry := store.NetworkAuditEntry{
		ID:          store.NewID("naud"),
		SessionID:   "netpeer:" + strings.TrimSpace(audit.PeerID),
		WorkspaceID: strings.TrimSpace(audit.WorkspaceID),
		Direction:   strings.TrimSpace(audit.Direction),
		Kind:        strings.TrimSpace(audit.Action),
		Channel:     strings.TrimSpace(audit.Channel),
		PeerFrom:    strings.TrimSpace(audit.PeerID),
		MessageID:   strings.TrimSpace(audit.RequestID),
		Reason:      strings.TrimSpace(audit.Reason),
		Size:        payloadSize,
		Timestamp:   at.UTC(),
	}
	if err := entry.Validate(); err != nil {
		return store.NetworkAuditEntry{}, fmt.Errorf("network: validate audit entry: %w", err)
	}
	return entry, nil
}

func trimmedPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func trimmedSurfaceValue(value *Surface) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(string(*value))
}

func (w *FileAuditWriter) appendFile(entry store.NetworkAuditEntry) error {
	w.fileMu.Lock()
	defer w.fileMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return fmt.Errorf("network: create audit log directory: %w", err)
	}

	file, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("network: open audit log file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	payload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("network: marshal audit log entry: %w", err)
	}
	payload = append(payload, '\n')

	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("network: append audit log entry: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("network: sync audit log file: %w", err)
	}

	return nil
}
