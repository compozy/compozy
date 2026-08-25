package terminal

// Suite: terminal output coalescer.
// Invariant: bulk output is bounded by size/time while latency-sensitive echo bypasses batching.
// Boundary IN: filtered process reads. Boundary OUT: ordered ring append chunks.

import (
	"bytes"
	"testing"
	"time"
)

func TestOutputCoalescerContract(t *testing.T) {
	t.Parallel()
	t.Log("[UT-032] output coalescing keeps bulk throughput bounded without delaying small echoes")

	t.Run("Should flush a small write into an empty coalescer immediately", func(t *testing.T) {
		t.Parallel()
		flushed := make(chan []byte, 1)
		coalescer := newOutputCoalescer(func(output []byte) { flushed <- append([]byte(nil), output...) })
		coalescer.Push([]byte("prompt> "))
		select {
		case got := <-flushed:
			if string(got) != "prompt> " {
				t.Fatalf("flush = %q", got)
			}
		case <-time.After(time.Millisecond):
			t.Fatal("small echo did not bypass coalescing")
		}
	})

	t.Run("Should flush bulk output at eight KiB", func(t *testing.T) {
		t.Parallel()
		flushed := make(chan []byte, 1)
		coalescer := newOutputCoalescer(func(output []byte) { flushed <- append([]byte(nil), output...) })
		input := bytes.Repeat([]byte("x"), outputCoalesceBytes)
		coalescer.Push(input)
		select {
		case got := <-flushed:
			if !bytes.Equal(got, input) {
				t.Fatalf("size flush bytes = %d, want %d", len(got), len(input))
			}
		case <-time.After(20 * time.Millisecond):
			t.Fatal("size threshold did not flush")
		}
	})

	t.Run("Should flush buffered bulk output after five milliseconds", func(t *testing.T) {
		t.Parallel()
		flushed := make(chan []byte, 1)
		coalescer := newOutputCoalescer(func(output []byte) { flushed <- append([]byte(nil), output...) })
		input := bytes.Repeat([]byte("x"), outputEchoBypass)
		started := time.Now()
		coalescer.Push(input)
		select {
		case <-coalescer.Ready():
			coalescer.Flush()
		case <-time.After(50 * time.Millisecond):
			t.Fatal("timer threshold did not become ready")
		}
		got := <-flushed
		elapsed := time.Since(started)
		if !bytes.Equal(got, input) || elapsed < outputCoalesceDelay {
			t.Fatalf("timer flush bytes=%d elapsed=%s", len(got), elapsed)
		}
	})
}
