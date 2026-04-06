package events

import (
	"context"
	"runtime"
	"sync"
	"testing"
)

func TestBusIntegrationConcurrentSubscribersMaintainOrder(t *testing.T) {
	restore := silenceDefaultLogger(t)
	defer restore()

	bus := New[Event](4)
	fast1ID, fast1Ch, fast1Unsub := bus.Subscribe()
	defer fast1Unsub()
	fast2ID, fast2Ch, fast2Unsub := bus.Subscribe()
	defer fast2Unsub()
	slowID, slowCh, slowUnsub := bus.Subscribe()
	defer slowUnsub()

	var (
		wg        sync.WaitGroup
		fast1Seen []uint64
		fast2Seen []uint64
	)
	fast1Ack := make(chan struct{}, 1)
	fast2Ack := make(chan struct{}, 1)

	collect := func(ch <-chan Event, dst *[]uint64, ack chan<- struct{}) {
		defer wg.Done()
		for evt := range ch {
			*dst = append(*dst, evt.Seq)
			ack <- struct{}{}
		}
	}

	wg.Add(2)
	go collect(fast1Ch, &fast1Seen, fast1Ack)
	go collect(fast2Ch, &fast2Seen, fast2Ack)

	for i := 1; i <= 50; i++ {
		bus.Publish(context.Background(), Event{
			SchemaVersion: SchemaVersion,
			RunID:         "run-1",
			Seq:           uint64(i),
			Kind:          EventKindJobStarted,
		})
		<-fast1Ack
		<-fast2Ack
	}

	if got := bus.DroppedFor(slowID); got != 46 {
		t.Fatalf("unexpected slow subscriber drops: %d", got)
	}
	if got := bus.DroppedFor(fast1ID); got != 0 {
		t.Fatalf("unexpected fast1 drops: %d", got)
	}
	if got := bus.DroppedFor(fast2ID); got != 0 {
		t.Fatalf("unexpected fast2 drops: %d", got)
	}

	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("close bus: %v", err)
	}
	wg.Wait()

	assertMonotonicSeq(t, fast1Seen, 50, "fast1")
	assertMonotonicSeq(t, fast2Seen, 50, "fast2")

	slowSeen := make([]uint64, 0, 4)
	for evt := range slowCh {
		slowSeen = append(slowSeen, evt.Seq)
	}
	assertStrictlyIncreasing(t, slowSeen, "slow")
}

func TestBusIntegrationNoGoroutineLeakAfterSubscribeCycles(t *testing.T) {
	before := runtime.NumGoroutine()
	bus := New[int](1)

	for i := 0; i < 10000; i++ {
		_, _, unsub := bus.Subscribe()
		unsub()
	}

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

func assertMonotonicSeq(t *testing.T, seqs []uint64, expected int, label string) {
	t.Helper()

	if len(seqs) != expected {
		t.Fatalf("%s subscriber received %d events, want %d", label, len(seqs), expected)
	}
	for i, seq := range seqs {
		want := uint64(i + 1)
		if seq != want {
			t.Fatalf("%s subscriber seq[%d] = %d, want %d", label, i, seq, want)
		}
	}
}

func assertStrictlyIncreasing(t *testing.T, seqs []uint64, label string) {
	t.Helper()

	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] {
			t.Fatalf("%s subscriber order broke at %d: %v", label, i, seqs)
		}
	}
}
