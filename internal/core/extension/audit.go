package extensions

import (
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

	"github.com/compozy/compozy/internal/core/run/journal"
)

const AuditLogFileName = "extensions.jsonl"

var (
	// ErrAuditLoggerNotOpen reports writes before Open.
	ErrAuditLoggerNotOpen = errors.New("audit logger not open")
	// ErrAuditLoggerClosed reports writes after Close begins.
	ErrAuditLoggerClosed = errors.New("audit logger closed")
	// ErrAuditLoggerOpen reports an attempt to open an already-open logger.
	ErrAuditLoggerOpen = errors.New("audit logger already open")
)

// AuditDirection identifies the direction of one extension RPC exchange.
type AuditDirection string

const (
	// AuditDirectionHostToExt identifies a host -> extension call such as execute_hook.
	AuditDirectionHostToExt AuditDirection = "host→ext"
	// AuditDirectionExtToHost identifies an extension -> host call such as host.tasks.create.
	AuditDirectionExtToHost AuditDirection = "ext→host"
)

// AuditResult identifies whether one audited call succeeded.
type AuditResult string

const (
	// AuditResultOK identifies a successful audited call.
	AuditResultOK AuditResult = "ok"
	// AuditResultError identifies a failed audited call.
	AuditResultError AuditResult = "error"
)

// AuditEntry is the in-memory representation of one extension audit record.
type AuditEntry struct {
	Timestamp   time.Time
	Extension   string
	Direction   AuditDirection
	Method      string
	Capability  Capability
	Latency     time.Duration
	Result      AuditResult
	ErrorDetail string
}

type auditEntryJSON struct {
	Timestamp   time.Time      `json:"ts"`
	Extension   string         `json:"extension"`
	Direction   AuditDirection `json:"direction"`
	Method      string         `json:"method"`
	Capability  string         `json:"capability"`
	LatencyMS   int64          `json:"latency_ms"`
	Result      AuditResult    `json:"result"`
	ErrorDetail string         `json:"error,omitempty"`
}

// AuditHandler is the runtime-facing interface for audit recording.
type AuditHandler interface {
	Record(entry AuditEntry) error
}

// AuditLogger owns the append-only extensions.jsonl writer for one run.
type AuditLogger struct {
	mu        sync.Mutex
	file      *os.File
	journal   *journal.Journal
	path      string
	closed    bool
	closeDone chan struct{}
	closeErr  error
}

var _ AuditHandler = (*AuditLogger)(nil)

// Open creates or truncates the run-scoped extensions audit log.
func (l *AuditLogger) Open(runArtifactsPath string) error {
	if l == nil {
		return errors.New("open audit logger: nil logger")
	}

	runArtifactsPath = strings.TrimSpace(runArtifactsPath)
	if runArtifactsPath == "" {
		return errors.New("open audit logger: missing run artifacts path")
	}
	runArtifactsPath = filepath.Clean(runArtifactsPath)
	resolvedRunArtifactsPath, err := filepath.Abs(runArtifactsPath)
	if err != nil {
		return fmt.Errorf("open audit logger: resolve artifacts path: %w", err)
	}

	info, err := os.Stat(resolvedRunArtifactsPath)
	if err != nil {
		return fmt.Errorf("open audit logger: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open audit logger: %q is not a directory", resolvedRunArtifactsPath)
	}

	path := filepath.Join(resolvedRunArtifactsPath, AuditLogFileName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open audit logger file: %w", err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		_ = file.Close()
		return fmt.Errorf("open audit logger: %w", ErrAuditLoggerOpen)
	}
	if l.closeDone != nil {
		select {
		case <-l.closeDone:
		default:
			_ = file.Close()
			return fmt.Errorf("open audit logger: %w", ErrAuditLoggerClosed)
		}
	}

	l.file = file
	l.path = path
	l.closed = false
	l.closeDone = nil
	l.closeErr = nil
	return nil
}

// Path reports the absolute path of the audit log file, when opened.
func (l *AuditLogger) Path() string {
	if l == nil {
		return ""
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.path
}

// Record appends one JSONL audit record.
func (l *AuditLogger) Record(entry AuditEntry) error {
	if l == nil {
		return errors.New("record audit entry: nil logger")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		if l.closed || l.closeDone != nil {
			return ErrAuditLoggerClosed
		}
		return ErrAuditLoggerNotOpen
	}

	line, err := marshalAuditEntry(entry)
	if err != nil {
		return fmt.Errorf("record audit entry: %w", err)
	}

	if err := writeAll(l.file, line); err != nil {
		return fmt.Errorf("record audit entry: %w", err)
	}
	if l.journal != nil {
		payloadJSON := strings.TrimSpace(string(line))
		if err := l.journal.RecordHookRun(context.Background(), journal.HookRunRecord{
			HookName:    strings.TrimSpace(entry.Method),
			Source:      strings.TrimSpace(entry.Extension),
			Outcome:     string(entry.Result),
			Duration:    entry.Latency,
			PayloadJSON: payloadJSON,
			RecordedAt:  entry.Timestamp,
		}); err != nil {
			return fmt.Errorf("record audit entry: %w", err)
		}
	}
	return nil
}

// Close performs the final fsync and closes the audit file, respecting the
// caller's deadline or cancellation signal.
func (l *AuditLogger) Close(ctx context.Context) error {
	if l == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	l.mu.Lock()
	if l.closeDone != nil {
		done := l.closeDone
		l.mu.Unlock()
		select {
		case <-done:
			l.mu.Lock()
			err := l.closeErr
			l.mu.Unlock()
			return err
		case <-ctx.Done():
			return fmt.Errorf("close audit logger: %w", ctx.Err())
		}
	}

	file := l.file
	if file == nil {
		l.closed = true
		l.mu.Unlock()
		return nil
	}

	done := make(chan struct{})
	l.closeDone = done
	l.file = nil
	l.closed = true
	l.mu.Unlock()

	go func(fileToClose *os.File) {
		err := syncAndCloseFile(fileToClose)

		l.mu.Lock()
		l.closeErr = err
		l.path = ""
		close(done)
		l.mu.Unlock()
	}(file)

	select {
	case <-done:
		l.mu.Lock()
		err := l.closeErr
		l.mu.Unlock()
		return err
	case <-ctx.Done():
		return fmt.Errorf("close audit logger: %w", ctx.Err())
	}
}

func marshalAuditEntry(entry AuditEntry) ([]byte, error) {
	payload, err := entry.toJSON()
	if err != nil {
		return nil, err
	}

	line, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal audit entry: %w", err)
	}
	return append(line, '\n'), nil
}

func (e AuditEntry) toJSON() (auditEntryJSON, error) {
	extension := strings.TrimSpace(e.Extension)
	if extension == "" {
		return auditEntryJSON{}, errors.New("extension is required")
	}

	method := strings.TrimSpace(e.Method)
	if method == "" {
		return auditEntryJSON{}, errors.New("method is required")
	}

	switch e.Direction {
	case AuditDirectionHostToExt, AuditDirectionExtToHost:
	default:
		return auditEntryJSON{}, fmt.Errorf("unsupported audit direction %q", e.Direction)
	}

	result := e.Result
	switch result {
	case "":
		if strings.TrimSpace(e.ErrorDetail) != "" {
			result = AuditResultError
		} else {
			result = AuditResultOK
		}
	case AuditResultOK, AuditResultError:
	default:
		return auditEntryJSON{}, fmt.Errorf("unsupported audit result %q", e.Result)
	}

	timestamp := e.Timestamp.UTC()
	if e.Timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	latencyMS := e.Latency.Milliseconds()
	if latencyMS < 0 {
		latencyMS = 0
	}

	return auditEntryJSON{
		Timestamp:   timestamp,
		Extension:   extension,
		Direction:   e.Direction,
		Method:      method,
		Capability:  strings.TrimSpace(string(e.Capability)),
		LatencyMS:   latencyMS,
		Result:      result,
		ErrorDetail: strings.TrimSpace(e.ErrorDetail),
	}, nil
}

func writeAll(file *os.File, data []byte) error {
	for written := 0; written < len(data); {
		n, err := file.Write(data[written:])
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func syncAndCloseFile(file *os.File) error {
	if file == nil {
		return nil
	}

	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync audit logger file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close audit logger file: %w", err)
	}
	return nil
}
