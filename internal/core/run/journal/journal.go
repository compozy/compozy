package journal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/store/rundb"
	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/runs/layout"
)

const (
	defaultBufferCapacity = 1024
	defaultBatchSize      = 32
	defaultFlushInterval  = 100 * time.Millisecond
	defaultSubmitTimeout  = 5 * time.Second
	writerBufferSize      = 16 << 10
)

var (
	// ErrClosed reports submits to a closed journal.
	ErrClosed = errors.New("journal closed")
	// ErrSubmitTimeout reports a dropped submit after the backpressure window expires.
	ErrSubmitTimeout = errors.New("journal submit timeout")
)

// Journal persists per-run events before forwarding them to live subscribers.
type Journal struct {
	path   string
	dbPath string
	runID  string
	inbox  chan submitRequest
	done   chan struct{}
	store  *rundb.RunDB

	busMu sync.RWMutex
	bus   *events.Bus[events.Event]

	closeOnce sync.Once
	submitMu  sync.RWMutex
	closing   bool

	submitTimeout time.Duration
	flushInterval time.Duration
	batchSize     int
	flushHook     func()
	afterSync     func()

	eventsWritten atomic.Uint64
	dropsOnSubmit atomic.Uint64

	resultMu  sync.RWMutex
	resultErr error
}

type openOptions struct {
	batchSize     int
	flushInterval time.Duration
	submitTimeout time.Duration
	flushHook     func()
	afterSync     func()
}

type writeState struct {
	file         *os.File
	writer       *bufio.Writer
	encoder      *json.Encoder
	pending      []events.Event
	pendingHooks []rundb.HookRunRecord
}

type submitRequestKind uint8

const (
	submitRequestEvent submitRequestKind = iota + 1
	submitRequestHook
)

type submitRequest struct {
	kind  submitRequestKind
	event events.Event
	hook  rundb.HookRunRecord
	ack   chan submitResult
}

type submitResult struct {
	seq uint64
	err error
}

// HookRunRecord carries one hook audit row into the run store.
type HookRunRecord struct {
	ID          string
	HookName    string
	Source      string
	Outcome     string
	Duration    time.Duration
	PayloadJSON string
	RecordedAt  time.Time
}

// Owner retains explicit close ownership for a journal that is passed through
// preparation objects.
type Owner struct {
	mu      sync.Mutex
	journal *Journal
}

// NewOwner wraps a journal with explicit cleanup ownership.
func NewOwner(j *Journal) *Owner {
	return &Owner{journal: j}
}

// Journal returns the owned journal instance, if any.
func (o *Owner) Journal() *Journal {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.journal
}

// Close closes the owned journal at most once and releases ownership.
func (o *Owner) Close(ctx context.Context) error {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	j := o.journal
	o.journal = nil
	o.mu.Unlock()
	if j == nil {
		return nil
	}
	return j.Close(ctx)
}

// Open creates a new journal writer for one run.
func Open(path string, bus *events.Bus[events.Event], bufCap int) (*Journal, error) {
	return openWithOptions(path, bus, bufCap, openOptions{})
}

func openWithOptions(path string, bus *events.Bus[events.Event], bufCap int, opts openOptions) (*Journal, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("open journal: missing path")
	}
	if bufCap <= 0 {
		bufCap = defaultBufferCapacity
	}
	if opts.batchSize <= 0 {
		opts.batchSize = defaultBatchSize
	}
	if opts.flushInterval <= 0 {
		opts.flushInterval = defaultFlushInterval
	}
	if opts.submitTimeout <= 0 {
		opts.submitTimeout = defaultSubmitTimeout
	}

	dbPath := layout.RunDBPath(filepath.Dir(path))
	store, err := rundb.Open(context.Background(), dbPath)
	if err != nil {
		return nil, err
	}
	lastSeq, err := store.CurrentMaxSequence(context.Background())
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("open journal store sequence: %w", err)
	}
	file, err := openJournalFile(path)
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	j := &Journal{
		path:          path,
		dbPath:        dbPath,
		runID:         filepath.Base(filepath.Dir(path)),
		inbox:         make(chan submitRequest, bufCap),
		bus:           bus,
		done:          make(chan struct{}),
		store:         store,
		submitTimeout: opts.submitTimeout,
		flushInterval: opts.flushInterval,
		batchSize:     opts.batchSize,
		flushHook:     opts.flushHook,
		afterSync:     opts.afterSync,
	}
	go j.writeLoop(file, lastSeq)
	return j, nil
}

// Submit enqueues one event for durable append, respecting caller cancellation.
func (j *Journal) Submit(ctx context.Context, ev events.Event) error {
	return j.submit(ctx, submitRequest{kind: submitRequestEvent, event: ev})
}

// SubmitWithSeq enqueues one event and waits for the assigned journal sequence.
func (j *Journal) SubmitWithSeq(ctx context.Context, ev events.Event) (uint64, error) {
	ack := make(chan submitResult, 1)
	if err := j.submit(ctx, submitRequest{kind: submitRequestEvent, event: ev, ack: ack}); err != nil {
		return 0, err
	}

	select {
	case result := <-ack:
		return result.seq, result.err
	case <-j.done:
		if err := j.result(); err != nil {
			return 0, err
		}
		return 0, ErrClosed
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

// RecordHookRun persists one hook audit record through the serialized journal loop.
func (j *Journal) RecordHookRun(ctx context.Context, record HookRunRecord) error {
	return j.submit(ctx, submitRequest{
		kind: submitRequestHook,
		hook: rundb.HookRunRecord{
			ID:          record.ID,
			HookName:    record.HookName,
			Source:      record.Source,
			Outcome:     record.Outcome,
			DurationNS:  record.Duration.Nanoseconds(),
			PayloadJSON: record.PayloadJSON,
			RecordedAt:  record.RecordedAt,
		},
	})
}

func (j *Journal) submit(ctx context.Context, req submitRequest) error {
	if j == nil {
		return errors.New("submit journal: nil journal")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.closedError(); err != nil {
		return err
	}

	j.submitMu.RLock()
	if j.closing {
		j.submitMu.RUnlock()
		return j.closedError()
	}
	defer j.submitMu.RUnlock()

	select {
	case <-j.done:
		return j.closedError()
	case j.inbox <- req:
		return nil
	default:
	}

	timer := time.NewTimer(j.submitTimeout)
	defer timer.Stop()

	select {
	case <-j.done:
		return j.closedError()
	case j.inbox <- req:
		return nil
	case <-timer.C:
		droppedTotal := j.dropsOnSubmit.Add(1)
		slog.Warn(
			"journal submit timed out",
			"component", "journal",
			"run_id", j.runID,
			"buffer_depth", len(j.inbox),
			"drops_total", droppedTotal,
		)
		return ErrSubmitTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close drains the queue, performs a final flush+sync, and closes the file.
func (j *Journal) Close(ctx context.Context) error {
	if j == nil {
		return nil
	}
	j.closeOnce.Do(j.beginClose)
	select {
	case <-j.done:
		return j.result()
	case <-ctx.Done():
		return fmt.Errorf("close journal: %w", ctx.Err())
	}
}

// EventsWritten reports the number of events durably flushed to disk.
func (j *Journal) EventsWritten() uint64 {
	if j == nil {
		return 0
	}
	return j.eventsWritten.Load()
}

// Path reports the events.jsonl path owned by the journal.
func (j *Journal) Path() string {
	if j == nil {
		return ""
	}
	return j.path
}

// DBPath reports the run.db path owned by the journal.
func (j *Journal) DBPath() string {
	if j == nil {
		return ""
	}
	return j.dbPath
}

// DropsOnSubmit reports the number of submits dropped after backpressure timeout.
func (j *Journal) DropsOnSubmit() uint64 {
	if j == nil {
		return 0
	}
	return j.dropsOnSubmit.Load()
}

// SetBus updates the live fan-out bus used for published journal events.
func (j *Journal) SetBus(bus *events.Bus[events.Event]) {
	if j == nil {
		return
	}
	j.busMu.Lock()
	j.bus = bus
	j.busMu.Unlock()
}

// CurrentBufferDepth reports the current enqueue depth.
func (j *Journal) CurrentBufferDepth() int {
	if j == nil {
		return 0
	}
	return len(j.inbox)
}

func (j *Journal) closedError() error {
	j.submitMu.RLock()
	closing := j.closing
	j.submitMu.RUnlock()

	select {
	case <-j.done:
		if err := j.result(); err != nil {
			return err
		}
		return ErrClosed
	default:
		if closing {
			return ErrClosed
		}
		return nil
	}
}

func (j *Journal) liveBus() *events.Bus[events.Event] {
	if j == nil {
		return nil
	}
	j.busMu.RLock()
	defer j.busMu.RUnlock()
	return j.bus
}

func (j *Journal) writeLoop(file *os.File, lastSeq uint64) {
	defer func() {
		j.beginClose()
		if err := file.Close(); err != nil {
			j.storeResult(fmt.Errorf("close journal file: %w", err))
		}
		if j.store != nil {
			if err := j.store.Close(); err != nil {
				j.storeResult(fmt.Errorf("close run store: %w", err))
			}
		}
		close(j.done)
	}()

	ticker := time.NewTicker(j.flushInterval)
	defer ticker.Stop()

	state := &writeState{
		file:         file,
		writer:       bufio.NewWriterSize(file, writerBufferSize),
		pending:      make([]events.Event, 0, j.batchSize),
		pendingHooks: make([]rundb.HookRunRecord, 0, j.batchSize),
	}
	state.encoder = json.NewEncoder(state.writer)

	seq := lastSeq
	if err := j.runActiveLoop(state, &seq, ticker.C); err != nil {
		j.storeResult(err)
	}
}

func (j *Journal) runActiveLoop(state *writeState, seq *uint64, ticks <-chan time.Time) error {
	for {
		select {
		case req, ok := <-j.inbox:
			if !ok {
				return j.flushPending(state)
			}
			assignedSeq, err := j.handleRequest(state, req, seq)
			if req.ack != nil {
				req.ack <- submitResult{seq: assignedSeq, err: err}
			}
			if err != nil {
				return err
			}
		case <-ticks:
			if len(state.pending) == 0 {
				continue
			}
			if err := j.flushPending(state); err != nil {
				return err
			}
		}
	}
}

func (j *Journal) handleRequest(state *writeState, req submitRequest, seq *uint64) (uint64, error) {
	switch req.kind {
	case submitRequestHook:
		return 0, j.handleHook(state, req.hook)
	case submitRequestEvent:
		return j.handleEvent(state, req.event, seq)
	default:
		return 0, fmt.Errorf("handle journal request: unsupported kind %d", req.kind)
	}
}

func (j *Journal) handleEvent(state *writeState, ev events.Event, seq *uint64) (uint64, error) {
	enriched, err := j.encodeEvent(state.encoder, ev, seq)
	if err != nil {
		return 0, err
	}
	state.pending = append(state.pending, enriched)
	if !j.shouldFlushAfterAppend(state.pending, enriched.Kind) {
		return enriched.Seq, nil
	}
	return enriched.Seq, j.flushPending(state)
}

func (j *Journal) handleHook(state *writeState, record rundb.HookRunRecord) error {
	state.pendingHooks = append(state.pendingHooks, record)
	return j.flushPending(state)
}

func (j *Journal) shouldFlushAfterAppend(pending []events.Event, kind events.EventKind) bool {
	return isTerminalEvent(kind) || len(pending) >= j.batchSize
}

func (j *Journal) flushPending(state *writeState) error {
	if len(state.pending) == 0 && len(state.pendingHooks) == 0 {
		return nil
	}
	if j.store != nil {
		ctx := context.Background()
		if len(state.pending) > 0 {
			if err := j.store.StoreEventBatch(ctx, state.pending); err != nil {
				return err
			}
		}
		for _, hook := range state.pendingHooks {
			if err := j.store.RecordHookRun(ctx, hook); err != nil {
				return err
			}
		}
	}
	if err := j.flushBatch(state.writer, state.file, state.pending); err != nil {
		return err
	}
	state.pending = state.pending[:0]
	state.pendingHooks = state.pendingHooks[:0]
	return nil
}

func (j *Journal) beginClose() {
	j.submitMu.Lock()
	if !j.closing {
		j.closing = true
		close(j.inbox)
	}
	j.submitMu.Unlock()
}

func openJournalFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open journal file: %w", err)
	}

	if err := recoverJournalFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func recoverJournalFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat journal file: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}

	truncateOffset, partialTail, err := lastCompleteLineOffset(file, info.Size())
	if err != nil {
		return fmt.Errorf("inspect journal file: %w", err)
	}
	if partialTail {
		if err := file.Truncate(truncateOffset); err != nil {
			return fmt.Errorf("truncate journal partial tail: %w", err)
		}
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek journal start: %w", err)
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek journal end: %w", err)
	}
	return nil
}

func lastCompleteLineOffset(file *os.File, size int64) (int64, bool, error) {
	if size == 0 {
		return 0, false, nil
	}

	var lastByte [1]byte
	if _, err := file.ReadAt(lastByte[:], size-1); err != nil {
		return 0, false, err
	}
	if lastByte[0] == '\n' {
		return size, false, nil
	}

	const chunkSize = 64 * 1024
	for chunkEnd := size; chunkEnd > 0; {
		chunkStart := chunkEnd - chunkSize
		if chunkStart < 0 {
			chunkStart = 0
		}

		buf := make([]byte, chunkEnd-chunkStart)
		if _, err := file.ReadAt(buf, chunkStart); err != nil {
			return 0, false, err
		}
		if idx := bytes.LastIndexByte(buf, '\n'); idx >= 0 {
			return chunkStart + int64(idx+1), true, nil
		}
		if chunkStart == 0 {
			break
		}
		chunkEnd = chunkStart
	}
	return 0, true, nil
}

func (j *Journal) encodeEvent(encoder *json.Encoder, ev events.Event, seq *uint64) (events.Event, error) {
	*seq++
	enriched := ev
	if strings.TrimSpace(enriched.SchemaVersion) == "" {
		enriched.SchemaVersion = events.SchemaVersion
	}
	if strings.TrimSpace(enriched.RunID) == "" {
		enriched.RunID = j.runID
	}
	if enriched.Timestamp.IsZero() {
		enriched.Timestamp = time.Now().UTC()
	}
	enriched.Seq = *seq

	if err := encoder.Encode(enriched); err != nil {
		return events.Event{}, fmt.Errorf("encode journal event: %w", err)
	}
	return enriched, nil
}

func (j *Journal) flushBatch(writer *bufio.Writer, file *os.File, pending []events.Event) error {
	startedAt := time.Now()

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush journal buffer: %w", err)
	}
	if j.flushHook != nil {
		j.flushHook()
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync journal file: %w", err)
	}
	if j.afterSync != nil {
		j.afterSync()
	}

	latency := time.Since(startedAt)
	if len(pending) > 0 {
		j.eventsWritten.Add(uint64(len(pending)))
		lastSeq := pending[len(pending)-1].Seq
		slog.Debug(
			"journal batch flushed",
			"component", "journal",
			"run_id", j.runID,
			"seq", lastSeq,
			"flush_latency_ms", latency.Milliseconds(),
		)
		if bus := j.liveBus(); bus != nil {
			ctx := context.Background()
			for _, ev := range pending {
				bus.Publish(ctx, ev)
			}
		}
	}

	return nil
}

func (j *Journal) storeResult(err error) {
	if err == nil {
		return
	}
	j.resultMu.Lock()
	defer j.resultMu.Unlock()
	if j.resultErr == nil {
		j.resultErr = err
	}
}

func (j *Journal) result() error {
	j.resultMu.RLock()
	defer j.resultMu.RUnlock()
	return j.resultErr
}

func isTerminalEvent(kind events.EventKind) bool {
	switch kind {
	case events.EventKindRunCrashed,
		events.EventKindRunCompleted,
		events.EventKindRunFailed,
		events.EventKindRunCancelled:
		return true
	default:
		return false
	}
}
