package sessiondb

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// Not parallel: every case installs the process-wide write observer and global OTel providers.
func TestSessionWriterAttribution(t *testing.T) {
	// Invariant IT-008: a slow coalesced flush inside a turn is one db.write span under the turn, and the
	// duration histogram has its point. Owner: session writer with the telemetry write observer.
	t.Run("Should span a slow coalesced flush under its turn [IT-008]", func(t *testing.T) {
		writes := installSessionWriteTelemetry(t, 50)
		db := openTestSessionDB(t, "sess-slow-flush")
		ctx, turn, account := startAttributedTurn(t, writes)
		release, waits := holdSessionWriteConnection(t, db)
		recorded := make(chan error, 1)
		go func() { recorded <- db.Record(ctx, agentChunk(t, "turn-slow", "slow")) }()
		waitForBlockedWrite(t, db, waits)
		time.Sleep(60 * time.Millisecond)
		release()
		if err := <-recorded; err != nil {
			t.Fatalf("Record() error = %v", err)
		}
		turn.End()

		var spans []tracetest.SpanStub
		for _, span := range writes.spans.GetSpans() {
			if span.Name == telemetry.SpanDBWrite {
				spans = append(spans, span)
			}
		}
		if len(spans) != 1 {
			t.Fatalf("db.write spans = %d, want one for the slow flush", len(spans))
		}
		if spans[0].Parent.SpanID() != turn.SpanContext().SpanID() {
			t.Fatalf("db.write parent = %s, want the turn %s", spans[0].Parent.SpanID(), turn.SpanContext().SpanID())
		}
		if coalesced, ok := spanAttribute(spans[0], telemetry.AttrDBCoalesced); !ok || !coalesced.AsBool() {
			t.Fatalf("db.write attributes = %v, want compozy.db.coalesced=true", spans[0].Attributes)
		}
		if spans[0].EndTime.Sub(spans[0].StartTime) < 50*time.Millisecond {
			t.Fatalf("db.write lasted %s, want the injected latency", spans[0].EndTime.Sub(spans[0].StartTime))
		}
		if points := writes.durationPoints(t, true); points != 1 {
			t.Fatalf("coalesced %s points = %d, want 1", telemetry.MetricDBWriteDuration, points)
		}
		if flushes, _ := account.Close(); flushes != 1 {
			t.Fatalf("turn flushes = %d, want 1", flushes)
		}
	})

	// Invariant IT-015 (Safety Invariant 7): each executed flush updates the turn account once with its duration; a
	// member canceled before execution leaves its batch committing and attributed once. Owner: session writer.
	t.Run("Should account every turn flush once despite a canceled batch member [IT-015]", func(t *testing.T) {
		writes := installSessionWriteTelemetry(t, 50)
		db := openTestSessionDB(t, "sess-turn-flushes")
		ctx, turn, account := startAttributedTurn(t, writes)
		for _, text := range []string{"a", "b"} {
			if err := db.Record(ctx, agentChunk(t, "turn-flushes", text)); err != nil {
				t.Fatalf("Record(%q) error = %v", text, err)
			}
		}
		release, waits := holdSessionWriteConnection(t, db)
		blocked := make(chan error, 1)
		go func() { blocked <- db.Record(ctx, agentChunk(t, "turn-flushes", "c")) }()
		waitForBlockedWrite(t, db, waits)
		final := make(chan error, 1)
		go func() { final <- db.Record(ctx, agentChunk(t, "turn-flushes", "d")) }()
		waitForQueuedWrites(t, db, 1)
		canceledCtx, cancel := context.WithCancel(ctx)
		canceled := make(chan error, 1)
		go func() { canceled <- db.Record(canceledCtx, agentChunk(t, "turn-flushes", "x")) }()
		waitForQueuedWrites(t, db, 2)
		cancel()
		if err := <-canceled; !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Record() error = %v, want context.Canceled", err)
		}
		release()
		for name, done := range map[string]chan error{"blocked": blocked, "final": final} {
			if err := <-done; err != nil {
				t.Fatalf("%s Record() error = %v", name, err)
			}
		}
		turn.End()

		if text := storedTurnText(t, db); text != "abcd" {
			t.Fatalf("persisted text = %q, want the canceled member excluded", text)
		}
		flushes, total := account.Close()
		attributed := writes.attributedTo(account)
		var sum time.Duration
		for _, write := range attributed {
			sum += write.Duration
		}
		if flushes != 4 || len(attributed) != 4 || total != sum {
			t.Fatalf("turn account = %d flushes %s, observed %d writes %s, want 4 flushes with their summed duration",
				flushes, total, len(attributed), sum)
		}
	})

	// Invariant IT-021 (Safety Invariant 6): a non-coalesced write keeps its caller context, so a caller deadline
	// while blocked on write admission fails the write without committing or counting it. Owner: session writer.
	t.Run(
		"Should honor the caller deadline of a non-coalesced write blocked on admission [IT-021]",
		func(t *testing.T) {
			writes := installSessionWriteTelemetry(t, 50)
			db := openTestSessionDB(t, "sess-admission-deadline")
			ctx, turn, account := startAttributedTurn(t, writes)
			started, releaseAdmission := make(chan struct{}), make(chan struct{})
			holder := make(chan error, 1)
			go func() {
				holder <- store.ExecuteWriteOperation(t.Context(), db.db, "test.holder",
					func(context.Context, *store.WriteTx) error {
						close(started)
						<-releaseAdmission
						return nil
					})
			}()
			<-started
			deadlineCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
			defer cancel()
			err := db.Record(deadlineCtx, canonicalStoreEvent(t, acp.AgentEvent{
				Type: acp.EventTypeToolCall, TurnID: "turn-deadline", ToolCallID: "call-1",
			}, "coder"))
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Record() error = %v, want the caller deadline", err)
			}
			waitForObservation(t, writes, account)
			close(releaseAdmission)
			if err := <-holder; err != nil {
				t.Fatal(err)
			}
			turn.End()

			rows, err := db.Query(t.Context(), EventQuery{})
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 0 {
				t.Fatalf("persisted rows = %d, want the canceled write uncommitted", len(rows))
			}
			observed := writes.attributedTo(account)
			if len(observed) != 1 || observed[0].Coalesced || !errors.Is(observed[0].Err, context.DeadlineExceeded) {
				t.Fatalf("observed writes = %+v, want one non-coalesced write failed by the caller deadline", observed)
			}
			if flushes, _ := account.Close(); flushes != 0 {
				t.Fatalf("turn flushes = %d, want 0 for a write that never executed", flushes)
			}
		},
	)
}

type sessionWriteTelemetry struct {
	spans  *tracetest.InMemoryExporter
	reader *sdkmetric.ManualReader
	tracer trace.Tracer

	mu       sync.Mutex
	observed []observedSessionWrite
}

type observedSessionWrite struct {
	store.WriteObservation
	turn *store.TurnWriteAccount
}

// installSessionWriteTelemetry installs global providers and a write observer that records like the daemon's and keeps
// each observation with its turn account, until the test ends.
func installSessionWriteTelemetry(t *testing.T, slowDBWriteMS int) *sessionWriteTelemetry {
	t.Helper()
	recorded := &sessionWriteTelemetry{spans: tracetest.NewInMemoryExporter(), reader: sdkmetric.NewManualReader()}
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(recorded.spans))
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(recorded.reader))
	recorded.tracer = tracerProvider.Tracer("test")
	previousTracer, previousMeter := otel.GetTracerProvider(), otel.GetMeterProvider()
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	store.SetWriteObserver(func(ctx context.Context, write store.WriteObservation) {
		telemetry.RecordDBWrite(ctx, telemetry.DBWrite(write), slowDBWriteMS)
		attribution, _ := store.WriteAttributionFrom(ctx)
		recorded.mu.Lock()
		defer recorded.mu.Unlock()
		recorded.observed = append(recorded.observed, observedSessionWrite{write, attribution.Turn})
	})
	t.Cleanup(func() {
		store.SetWriteObserver(nil)
		otel.SetTracerProvider(previousTracer)
		otel.SetMeterProvider(previousMeter)
		if err := tracerProvider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
		if err := meterProvider.Shutdown(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return recorded
}

func (w *sessionWriteTelemetry) attributedTo(account *store.TurnWriteAccount) []store.WriteObservation {
	w.mu.Lock()
	defer w.mu.Unlock()
	var matched []store.WriteObservation
	for _, write := range w.observed {
		if write.turn == account {
			matched = append(matched, write.WriteObservation)
		}
	}
	return matched
}

func (w *sessionWriteTelemetry) durationPoints(t *testing.T, coalesced bool) uint64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := w.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	var count uint64
	for _, scope := range collected.ScopeMetrics {
		for _, metric := range scope.Metrics {
			histogram, ok := metric.Data.(metricdata.Histogram[float64])
			if metric.Name != telemetry.MetricDBWriteDuration || !ok {
				continue
			}
			for _, point := range histogram.DataPoints {
				if value, _ := point.Attributes.Value(telemetry.AttrDBCoalesced); value.AsBool() == coalesced {
					count += point.Count
				}
			}
		}
	}
	return count
}

// startAttributedTurn starts a recording turn span and returns the context a persistence pump writes with.
func startAttributedTurn(
	t *testing.T,
	writes *sessionWriteTelemetry,
) (context.Context, trace.Span, *store.TurnWriteAccount) {
	t.Helper()
	ctx, turn := writes.tracer.Start(t.Context(), telemetry.SpanSessionTurn)
	account := store.NewTurnWriteAccount()
	return store.WithWriteAttribution(ctx, store.WriteAttribution{Turn: account}), turn, account
}

// holdSessionWriteConnection limits the writer pool to one connection and takes it, so the next flush waits for
// release. It returns the pool's wait count once held.
func holdSessionWriteConnection(t *testing.T, db *SessionDB) (func(), int64) {
	t.Helper()
	db.db.SetMaxOpenConns(1)
	held, err := db.db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	release := sync.OnceFunc(func() {
		if err := held.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Cleanup(release)
	return release, db.db.Stats().WaitCount
}

// waitForBlockedWrite waits until a flush waits for the held writer connection.
func waitForBlockedWrite(t *testing.T, db *SessionDB, before int64) {
	t.Helper()
	waitForSessionWriter(t, "a flush waiting for the writer connection", func() bool {
		return db.db.Stats().WaitCount > before && len(db.writeCh) == 0
	})
}

func waitForQueuedWrites(t *testing.T, db *SessionDB, queued int) {
	t.Helper()
	waitForSessionWriter(t, "queued writes", func() bool { return len(db.writeCh) == queued })
}

func waitForObservation(t *testing.T, writes *sessionWriteTelemetry, account *store.TurnWriteAccount) {
	t.Helper()
	waitForSessionWriter(t, "the write observation", func() bool { return len(writes.attributedTo(account)) > 0 })
}

func waitForSessionWriter(t *testing.T, what string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func agentChunk(t *testing.T, turnID string, text string) SessionEvent {
	t.Helper()
	return canonicalStoreEvent(t, acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: turnID, Text: text}, "coder")
}

func storedTurnText(t *testing.T, db *SessionDB) string {
	t.Helper()
	rows, err := db.Query(t.Context(), EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, row := range rows {
		text += storedAgentText(t, row)
	}
	return text
}

func spanAttribute(span tracetest.SpanStub, key attribute.Key) (attribute.Value, bool) {
	for _, kv := range span.Attributes {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}
