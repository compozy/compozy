package events

import (
	"context"
	"io"
	"log/slog"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBusSubscribeReturnsUniqueMonotonicIDs(t *testing.T) {
	bus := New[int](4)

	var prev SubID
	for i := 0; i < 1000; i++ {
		id, _, unsub := bus.Subscribe()
		if id <= prev {
			t.Fatalf("subscription id %d did not increase after %d", id, prev)
		}
		prev = id
		unsub()
	}

	if count := bus.SubscriberCount(); count != 0 {
		t.Fatalf("unexpected subscriber count: %d", count)
	}
}

func TestBusPublishDeliversFanoutInOrder(t *testing.T) {
	bus := New[int](8)
	_, ch1, unsub1 := bus.Subscribe()
	defer unsub1()
	_, ch2, unsub2 := bus.Subscribe()
	defer unsub2()

	ctx := context.Background()
	for i := 1; i <= 5; i++ {
		bus.Publish(ctx, i)
	}

	for i := 1; i <= 5; i++ {
		if got := <-ch1; got != i {
			t.Fatalf("subscriber 1 event %d = %d", i, got)
		}
		if got := <-ch2; got != i {
			t.Fatalf("subscriber 2 event %d = %d", i, got)
		}
	}
}

func TestBusSlowSubscriberDropsWithoutBlockingFastSubscribers(t *testing.T) {
	restore := silenceDefaultLogger(t)
	defer restore()

	bus := New[int](1)
	slowID, _, slowUnsub := bus.Subscribe()
	defer slowUnsub()
	fastID, fastCh, fastUnsub := bus.Subscribe()
	defer fastUnsub()

	ctx := context.Background()
	for i := 1; i <= 10; i++ {
		bus.Publish(ctx, i)
		if got := <-fastCh; got != i {
			t.Fatalf("fast subscriber event %d = %d", i, got)
		}
	}

	if got := bus.DroppedFor(slowID); got != 9 {
		t.Fatalf("unexpected slow subscriber drops: %d", got)
	}
	if got := bus.DroppedFor(fastID); got != 0 {
		t.Fatalf("unexpected fast subscriber drops: %d", got)
	}
}

func TestBusDroppedForTracksSubscribersIndependently(t *testing.T) {
	restore := silenceDefaultLogger(t)
	defer restore()

	bus := New[int](1)
	firstID, _, firstUnsub := bus.Subscribe()
	defer firstUnsub()
	secondID, secondCh, secondUnsub := bus.Subscribe()
	defer secondUnsub()

	ctx := context.Background()
	bus.Publish(ctx, 1)
	bus.Publish(ctx, 2)
	if got := <-secondCh; got != 1 {
		t.Fatalf("unexpected buffered value: %d", got)
	}
	bus.Publish(ctx, 3)

	if got := bus.DroppedFor(firstID); got != 2 {
		t.Fatalf("unexpected first drops: %d", got)
	}
	if got := bus.DroppedFor(secondID); got != 1 {
		t.Fatalf("unexpected second drops: %d", got)
	}
}

func TestBusPublishCancelledContextReturnsImmediately(t *testing.T) {
	bus := New[int](1)
	_, ch, unsub := bus.Subscribe()
	defer unsub()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bus.Publish(ctx, 99)

	select {
	case got := <-ch:
		t.Fatalf("unexpected event received: %d", got)
	default:
	}
}

func TestBusUnsubscribeDuringPublishDoesNotPanicOrLeak(t *testing.T) {
	restore := silenceDefaultLogger(t)
	defer restore()

	before := runtime.NumGoroutine()
	bus := New[int](1)

	unsubs := make([]func(), 0, 100)
	for i := 0; i < 100; i++ {
		_, _, unsub := bus.Subscribe()
		unsubs = append(unsubs, unsub)
	}

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for j := 0; j < 500; j++ {
				bus.Publish(context.Background(), rng.Int())
			}
		}(int64(i + 1))
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for _, idx := range rng.Perm(len(unsubs)) {
				unsubs[idx]()
			}
		}(int64(i + 10))
	}

	wg.Wait()

	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("close bus: %v", err)
	}

	runtime.GC()
	runtime.GC()
	after := runtime.NumGoroutine()
	if delta := after - before; delta > 2 {
		t.Fatalf("goroutine leak detected: before=%d after=%d", before, after)
	}
}

func TestBusCloseClosesChannelsAndIsIdempotent(t *testing.T) {
	bus := New[int](1)
	_, ch1, _ := bus.Subscribe()
	_, ch2, _ := bus.Subscribe()

	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("second close: %v", err)
	}

	if _, ok := <-ch1; ok {
		t.Fatal("subscriber 1 channel should be closed")
	}
	if _, ok := <-ch2; ok {
		t.Fatal("subscriber 2 channel should be closed")
	}
}

func TestBusCloseExpiredContextReturnsError(t *testing.T) {
	bus := New[int](1)
	_, _, unsub := bus.Subscribe()
	defer unsub()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := bus.Close(ctx); err == nil {
		t.Fatal("expected close to fail for canceled context")
	}
}

func TestBusDropWarningsAreRateLimitedPerSubscriber(t *testing.T) {
	var warnings atomic.Int64
	previous := slog.Default()
	slog.SetDefault(slog.New(&countingHandler{count: &warnings}))
	defer slog.SetDefault(previous)

	bus := New[int](1)
	id, _, unsub := bus.Subscribe()
	defer unsub()

	ctx := context.Background()
	bus.Publish(ctx, 1)
	bus.Publish(ctx, 2)
	bus.Publish(ctx, 3)
	bus.Publish(ctx, 4)

	if got := warnings.Load(); got != 1 {
		t.Fatalf("unexpected warning count in burst: %d", got)
	}

	bus.mu.RLock()
	sub := bus.subs[id]
	bus.mu.RUnlock()
	if sub == nil {
		t.Fatal("missing subscription for warning test")
	}
	sub.lastWarnedAt.Store(time.Now().Add(-2 * time.Second).UnixNano())

	bus.Publish(ctx, 5)
	if got := warnings.Load(); got != 2 {
		t.Fatalf("unexpected warning count after reset: %d", got)
	}
}

type countingHandler struct {
	count *atomic.Int64
}

func (h *countingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *countingHandler) Handle(context.Context, slog.Record) error {
	h.count.Add(1)
	return nil
}

func (h *countingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *countingHandler) WithGroup(string) slog.Handler {
	return h
}

func silenceDefaultLogger(t *testing.T) func() {
	t.Helper()

	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return func() {
		slog.SetDefault(previous)
	}
}
