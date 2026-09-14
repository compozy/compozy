package contextusage

import (
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

var reportAt = time.Date(2026, 9, 11, 10, 0, 0, 0, time.UTC)

func observation(t *testing.T, sequence int64, turn string, used, size *int64) UsageEvent {
	t.Helper()
	return UsageEvent{Sequence: sequence, At: reportAt.Add(time.Duration(sequence) * time.Second), TurnID: turn,
		Usage: store.TokenUsage{TurnID: turn, ContextUsed: used, ContextSize: size}}
}

func TestDerive(t *testing.T) {
	t.Parallel()
	t.Run("Should select observations by sequence and preserve unknown quantities", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name       string
			in         Input
			state      State
			used, size *int64
			ratio      *float64
			source     string
			threshold  *float64
		}{
			{name: "Should preserve unknown state before an observation", in: Input{Available: true, CatalogWindow: new(int64(100))}, state: StateUnknown},
			{name: "Should ignore counter-only and negative context observations", in: Input{Available: true, UsageEvents: []UsageEvent{{Sequence: 9, Usage: store.TokenUsage{InputTokens: new(int64(10))}}, observation(t, 10, "A", new(int64(-1)), new(int64(100)))}}, state: StateUnknown},
			{name: "Should use an agent window", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), new(int64(100)))}, CatalogWindow: new(int64(200)), Threshold: new(0.85)}, state: StateReported, used: new(int64(80)), size: new(int64(100)), ratio: new(0.8), source: "agent", threshold: new(0.85)},
			{name: "Should use a catalog window without enabling pressure compaction", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), nil)}, CatalogWindow: new(int64(100)), Threshold: new(0.85)}, state: StateEstimatedSize, used: new(int64(80)), size: new(int64(100)), ratio: new(0.8), source: "catalog"},
			{name: "Should retain reported usage without a size", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), nil)}}, state: StateReported, used: new(int64(80))},
			{name: "Should preserve a raw over-capacity ratio", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(110)), new(int64(100)))}}, state: StateReported, used: new(int64(110)), size: new(int64(100)), ratio: new(1.1), source: "agent"},
			{name: "Should preserve zero usage and threshold equality", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(0)), new(int64(100)))}}, state: StateReported, used: new(int64(0)), size: new(int64(100)), ratio: new(0.0), source: "agent"},
			{name: "Should keep an eligible threshold at equality", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(85)), new(int64(100)))}, Threshold: new(0.85)}, state: StateReported, used: new(int64(85)), size: new(int64(100)), ratio: new(0.85), source: "agent", threshold: new(0.85)},
			{name: "Should omit a disabled threshold", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), new(int64(100)))}, Threshold: new(0.0)}, state: StateReported, used: new(int64(80)), size: new(int64(100)), ratio: new(0.8), source: "agent"},
			{name: "Should reject invalid sizes", in: Input{Available: true, UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), new(int64(0)))}, CatalogWindow: new(int64(-1))}, state: StateReported, used: new(int64(80))},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := Derive(tc.in)
				if got.State != tc.state || !reflect.DeepEqual(got.Used, tc.used) ||
					!reflect.DeepEqual(got.Size, tc.size) ||
					!reflect.DeepEqual(got.Ratio, tc.ratio) ||
					got.SizeSource != tc.source ||
					!reflect.DeepEqual(got.PressureThreshold, tc.threshold) {
					t.Fatalf("context = %#v", got)
				}
				if tc.state == StateUnknown &&
					(got.Sequence != nil || got.Stale != nil || got.ReportedAt != nil || got.ReportedTurnID != "") {
					t.Fatalf("unknown observation was fabricated: %#v", got)
				}
			})
		}
		older := observation(t, 5, "B", new(int64(200)), new(int64(300)))
		older.At = reportAt.Add(time.Hour)
		latest := observation(t, 10, "A", new(int64(80)), new(int64(100)))
		got := Derive(Input{Available: true, UsageEvents: []UsageEvent{latest, older}})
		if *got.Used != 80 || *got.Sequence != 10 || got.ReportedTurnID != "A" || !got.ReportedAt.Equal(latest.At) {
			t.Fatalf("ledger order lost: %#v", got)
		}
	})
	t.Run("Should distinguish settled turns from observations still in flight", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name    string
			settled *SettledTurn
			stale   bool
		}{
			{"Should stale an older different turn", &SettledTurn{TurnID: "B", Sequence: 15}, true},
			{"Should retain an in-flight newer turn", &SettledTurn{TurnID: "B", Sequence: 5}, false},
			{"Should retain the same settled turn", &SettledTurn{TurnID: "A", Sequence: 12}, false},
			{"Should retain a first in-flight turn", nil, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := Derive(
					Input{
						Available:   true,
						UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(80)), nil)},
						Settled:     tc.settled,
					},
				)
				if got.Stale == nil || *got.Stale != tc.stale {
					t.Fatalf("stale = %v", got.Stale)
				}
			})
		}
	})
	t.Run("Should return only unavailable state when reads fail", func(t *testing.T) {
		t.Parallel()
		got := Derive(
			Input{
				UsageEvents:   []UsageEvent{observation(t, 10, "A", new(int64(80)), new(int64(100)))},
				Deliveries:    []Delivery{{Spans: []Span{{Key: "skills", Tokens: new(int64(10))}}}},
				CatalogWindow: new(int64(100)),
				Threshold:     new(0.85),
			},
		)
		if !reflect.DeepEqual(got, ContextUsage{State: StateUnavailable}) {
			t.Fatalf("unavailable leaked partial input: %#v", got)
		}
	})
}

func TestAttribution(t *testing.T) {
	t.Parallel()
	t.Run("Should retain raw estimates and attachment receipts independently of reported usage", func(t *testing.T) {
		t.Parallel()
		got := Derive(
			Input{
				Available:   true,
				UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(89700)), new(int64(256000)))},
				Deliveries: []Delivery{
					{Sequence: 3, TurnID: "A", SentAt: reportAt, Estimate: "bytes_div_4", Spans: []Span{
						{Key: "system_prompt", Kind: "text", Bytes: 24400, Tokens: new(int64(6100))},
						{Key: "skills", Kind: "text", Bytes: 12400, Tokens: new(int64(3100))},
						{Key: "memory", Kind: "text", Bytes: 9600, Tokens: new(int64(2400))},
						{Key: "tools", Kind: "text", Bytes: 3200, Tokens: new(int64(800))},
						{Key: "attachment", Kind: "binary", Bytes: 184320, Name: "screenshot.png"},
					}},
				},
			},
		)
		if got.Injected == nil || got.Injected.Tokens != 12400 || len(got.Injected.Rows) != 5 ||
			got.Injected.Rows[4].Name != "screenshot.png" ||
			got.Injected.Rows[4].Tokens != nil {
			t.Fatalf("injected = %#v", got.Injected)
		}
		got = Derive(
			Input{
				Available:   true,
				UsageEvents: []UsageEvent{observation(t, 10, "A", new(int64(89700)), new(int64(256000)))},
				Deliveries: []Delivery{
					{
						Sequence: 3,
						TurnID:   "A",
						SentAt:   reportAt,
						Estimate: "bytes_div_4",
						Spans:    []Span{{Key: "skills", Kind: "text", Tokens: new(int64(90000))}},
					},
				},
			},
		)
		if got.Injected.Tokens != 90000 {
			t.Fatalf("raw attribution was clamped: %#v", got.Injected)
		}
	})
	t.Run("Should keep owners across stubs and clear staleness only through full replacement", func(t *testing.T) {
		t.Parallel()
		full := Delivery{
			Sequence: 3,
			TurnID:   "A",
			SentAt:   reportAt,
			Estimate: "bytes_div_4",
			Spans: []Span{
				{Key: "skills", Kind: "text", Tokens: new(int64(3100)), Bytes: 12400},
				{Key: "memory", Kind: "text", Tokens: new(int64(2400)), Bytes: 9600},
			},
		}
		stub := Delivery{
			Sequence: 30,
			TurnID:   "B",
			SentAt:   reportAt.Add(30 * time.Second),
			Estimate: "bytes_div_4",
			Spans:    []Span{{Key: "skills", Kind: "text", Tokens: new(int64(9)), Unchanged: true}},
		}
		in := Input{
			Available:  true,
			Deliveries: []Delivery{stub, full},
			UsageEvents: []UsageEvent{
				observation(t, 10, "A", new(int64(200)), nil),
				observation(t, 20, "B", new(int64(120)), nil),
			},
		}
		got := Derive(in).Injected
		row := got.Rows[0]
		if !got.Stale || !row.Stale || !row.Unchanged || row.LastSeenTurnID != "B" || row.DeliveredTurnID != "A" ||
			row.DeliverySequence != 3 ||
			*row.Tokens != 3100 ||
			row.OwnerKind != "full" {
			t.Fatalf("owner = %#v", row)
		}
		in.Deliveries = append(
			in.Deliveries,
			Delivery{
				Sequence: 40,
				TurnID:   "C",
				SentAt:   reportAt.Add(40 * time.Second),
				Estimate: "bytes_div_4",
				Spans:    []Span{{Key: "skills", Kind: "text", Tokens: new(int64(3400)), Bytes: 13600}},
			},
		)
		got = Derive(in).Injected
		if got.Rows[0].Stale || got.Rows[0].Unchanged || *got.Rows[0].Tokens != 3400 || !got.Stale ||
			!got.Rows[1].Stale {
			t.Fatalf("replacement freshness = %#v", got)
		}
	})
	t.Run("Should attribute startup dedup once and replace opaque rows on a full delivery", func(t *testing.T) {
		t.Parallel()
		in := Input{
			Available: true,
			Deliveries: []Delivery{{Sequence: 3, TurnID: "A", SentAt: reportAt, Estimate: "bytes_div_4", Spans: []Span{
				{Key: "system_prompt", Kind: "text", Tokens: new(int64(6000)), HookModified: true},
				{Key: "skills", Kind: "text", Tokens: new(int64(8)), Unchanged: true, StartupDedup: true},
			}}},
		}
		got := Derive(in).Injected
		if got.Tokens != 6000 || len(got.Rows) != 2 || got.Rows[1].OwnerKind != "startup_opaque" ||
			got.Rows[1].Tokens != nil ||
			!got.Rows[1].Unchanged ||
			got.Rows[1].DeliveredTurnID != "A" {
			t.Fatalf("startup = %#v", got)
		}
		in.Deliveries = append(
			in.Deliveries,
			Delivery{
				Sequence: 10,
				TurnID:   "B",
				SentAt:   reportAt.Add(time.Minute),
				Estimate: "bytes_div_4",
				Spans:    []Span{{Key: "skills", Kind: "text", Tokens: new(int64(3100))}},
			},
		)
		got = Derive(in).Injected
		if got.Rows[1].OwnerKind != "full" || *got.Rows[1].Tokens != 3100 || got.Rows[1].Unchanged {
			t.Fatalf("opaque replacement = %#v", got.Rows[1])
		}
		in.Deliveries = in.Deliveries[:1]
		in.Deliveries[0].Spans[0] = Span{Key: "skills", Kind: "text", Tokens: new(int64(3100))}
		got = Derive(in).Injected
		if got.Tokens != 3100 || len(got.Rows) != 1 || got.Rows[0].OwnerKind != "full" || !got.Rows[0].Unchanged {
			t.Fatalf("section startup = %#v", got)
		}
	})
	t.Run("Should use sent time for late confirmations and ignore replay compaction markers", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name  string
			used  int64
			sent  time.Time
			stale bool
		}{
			{"Should stale delivery sent before a drop", 120, reportAt, true},
			{"Should retain delivery sent after a drop", 120, reportAt.Add(46 * time.Second), false},
			{"Should retain a late confirmation without a drop", 210, reportAt, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				got := Derive(
					Input{
						Available: true,
						UsageEvents: []UsageEvent{
							observation(t, 44, "A", new(int64(200)), nil),
							observation(t, 45, "B", new(tc.used), nil),
						},
						Compactions: []Compaction{{Sequence: 43, SpanArchived: true}},
						Deliveries: []Delivery{
							{
								Sequence: 50,
								TurnID:   "A",
								SentAt:   tc.sent,
								Estimate: "bytes_div_4",
								Spans:    []Span{{Key: "skills", Kind: "text", Tokens: new(int64(20))}},
							},
						},
					},
				).Injected
				if got.Stale != tc.stale || got.Rows[0].Stale != tc.stale {
					t.Fatalf("late confirmation = %#v", got)
				}
			})
		}
	})
}

func TestTurns(t *testing.T) {
	t.Parallel()
	t.Run("Should return the ordered union of usage and delivery turns with archive facts", func(t *testing.T) {
		t.Parallel()
		in := Input{Available: true, UsageEvents: []UsageEvent{
			{Sequence: 77, TurnID: "D", Usage: store.TokenUsage{TurnID: "D", InputTokens: new(int64(10))}},
			observation(t, 41, "B", new(int64(80)), nil), observation(t, 9, "A", new(int64(60)), new(int64(100))),
			{Sequence: 7, TurnID: "A", Usage: store.TokenUsage{TurnID: "A", CacheReadTokens: new(int64(4))}},
		}, Deliveries: []Delivery{
			{
				Sequence: 58,
				TurnID:   "C",
				Estimate: "bytes_div_4",
				Spans:    []Span{{Key: "skills", Unchanged: true, StartupDedup: true}},
			},
			{Sequence: 3, TurnID: "A", Estimate: "bytes_div_4", Spans: []Span{{Key: "skills", Tokens: new(int64(20))}}},
		}, Compactions: []Compaction{{Sequence: 60, SpanArchived: true}, {Sequence: 12, SpanArchived: false}}}
		turns, compactions := Turns(in)
		if len(turns) != 4 || turns[0].Sequence != 9 || turns[1].Sequence != 41 || turns[2].Sequence != 58 ||
			turns[3].Sequence != 77 {
			t.Fatalf("turn order = %#v", turns)
		}
		if turns[0].Usage == nil || turns[0].Injected == nil || *turns[0].Usage.CacheReadTokens != 4 ||
			*turns[0].UsageSequence != 9 ||
			turns[1].Injected != nil ||
			turns[2].Usage != nil ||
			turns[3].Usage.ContextUsed != nil {
			t.Fatalf("turn union = %#v", turns)
		}
		if turns[2].Injected.Estimate != "bytes_div_4" || !turns[2].Injected.Spans[0].Unchanged ||
			!turns[2].Injected.Spans[0].StartupDedup {
			t.Fatalf("delivery = %#v", turns[2].Injected)
		}
		if len(compactions) != 2 || compactions[0].Sequence != 12 || compactions[0].SpanArchived ||
			!compactions[1].SpanArchived {
			t.Fatalf("compactions = %#v", compactions)
		}
		if in.UsageEvents[0].Sequence != 77 || in.Compactions[0].Sequence != 60 {
			t.Fatal("reduction reordered caller input")
		}
	})
}
