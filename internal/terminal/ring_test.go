package terminal

// Suite: terminal replay ring.
// Invariant: bounded retention preserves absolute byte positions and never replays an escape suffix as plain text.
// Boundary IN: filtered terminal output. Boundary OUT: attachment replay payloads.

import (
	"bytes"
	"testing"
)

func TestRingReplayContract(t *testing.T) {
	t.Parallel()

	t.Run("Should trim only at an escape-safe boundary [UT-011]", func(t *testing.T) {
		t.Parallel()
		ring := NewRing(12)
		ring.Append([]byte("old line\n\x1b[31mred\x1b[0m"))
		got, _ := ring.Snapshot()
		if len(got) > 12 {
			t.Fatalf("ring bytes = %d, want <= 12", len(got))
		}
		if len(got) > 0 && got[0] != 0x1b && !bytes.HasPrefix(got, []byte("red")) {
			t.Fatalf("ring head = %q, want ESC or text after a complete escape", got)
		}

		incomplete := NewRing(4)
		incomplete.Append([]byte("\x1b]unterminated"))
		got, _ = incomplete.Snapshot()
		if len(got) > 4 || (len(got) > 0 && got[0] != 0x1b) {
			t.Fatalf("incomplete escape retained as %q", got)
		}
	})

	t.Run("Should retain exact absolute byte offsets [UT-012]", func(t *testing.T) {
		t.Parallel()
		ring := NewRing(64)
		start, end := ring.Append([]byte("hello"))
		if start != 0 || end != 5 {
			t.Fatalf("Append(hello) = %d..%d, want 0..5", start, end)
		}
		start, end = ring.Append([]byte(" world"))
		if start != 5 || end != 11 {
			t.Fatalf("Append(world) = %d..%d, want 5..11", start, end)
		}
		replay := ring.ReplayFrom(6)
		if got, want := string(replay.Payload), "world"; got != want || replay.Seq != 11 || replay.Truncated {
			t.Fatalf("ReplayFrom(6) = %#v %q, want seq 11 world", replay, got)
		}
	})

	t.Run(
		"Should prepend reset and mode state after truncation without mouse tracking [UT-013][UT-014]",
		func(t *testing.T) {
			t.Parallel()
			ring := NewRing(8)
			preamble := []byte("\x1b[?1h\x1b[?2004h\x1b[?1049h")
			ring.SetModePreamble(preamble)
			ring.Append([]byte("discarded\nframe"))
			replay := ring.ReplayFrom(0)
			if !replay.Truncated || !bytes.HasPrefix(replay.Payload, []byte(resetSequence)) {
				t.Fatalf("ReplayFrom(0) = %#v, want reset + mode preamble resync", replay)
			}
			for _, sequence := range [][]byte{[]byte("\x1b[?1h"), []byte("\x1b[?1049h"), []byte("\x1b[?2004h")} {
				if !bytes.Contains(replay.Payload, sequence) {
					t.Fatalf("replay = %q, want canonical mode %q", replay.Payload, sequence)
				}
			}
			if bytes.Contains(replay.Payload, []byte("?1000")) || bytes.Contains(replay.Payload, []byte("?1006")) {
				t.Fatalf("replay contains mouse tracking: %q", replay.Payload)
			}
		},
	)

	t.Run("Should derive the safe mode preamble from split production output", func(t *testing.T) {
		t.Parallel()
		tracker := newModePreambleTracker()
		tracker.Observe([]byte("\x1b[?1;20"))
		preamble := tracker.Observe([]byte("04h\x1b=\x1b[?1000;1006h\x1b[?25l"))
		for _, sequence := range [][]byte{[]byte("\x1b[?1h"), []byte("\x1b[?2004h"), []byte("\x1b[?25l"), []byte("\x1b=")} {
			if !bytes.Contains(preamble, sequence) {
				t.Fatalf("preamble = %q, want %q", preamble, sequence)
			}
		}
		if bytes.Contains(preamble, []byte("?1000")) || bytes.Contains(preamble, []byte("?1006")) {
			t.Fatalf("preamble contains mouse tracking: %q", preamble)
		}
	})

	t.Run("Should preserve alternate-screen transitions in replay [UT-015]", func(t *testing.T) {
		t.Parallel()
		ring := NewRing(40)
		ring.SetModePreamble([]byte("\x1b[?1049h"))
		ring.Append([]byte("discarded discarded discarded discarded\n\x1b[2J\x1b[Halternate\x1b[?1049lprimary"))
		replay := ring.ReplayFrom(0)
		if !bytes.Contains(replay.Payload, []byte("\x1b[?1049h")) ||
			!bytes.Contains(replay.Payload, []byte("\x1b[?1049l")) {
			t.Fatalf("replay = %q, want enter and leave alternate screen", replay.Payload)
		}
	})

	t.Run("Should enforce the byte cap with oldest-first eviction [UT-016]", func(t *testing.T) {
		t.Parallel()
		ring := NewRing(5)
		ring.Append([]byte("12345"))
		ring.Append([]byte("67890"))
		got, seq := ring.Snapshot()
		oldest, next := ring.Bounds()
		if string(got) != "67890" || seq != 10 || next != 10 || oldest != 5 {
			t.Fatalf("ring = %q bounds=%d..%d seq=%d, want exact newest suffix 67890 at 5..10", got, oldest, next, seq)
		}
	})
}
