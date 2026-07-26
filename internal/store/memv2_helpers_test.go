package store

import (
	"testing"
	"time"
)

func TestSessionFailureHelpers(t *testing.T) {
	t.Run("Should normalize validate and clone supported failures", func(t *testing.T) {
		t.Parallel()

		failure := SessionFailure{
			Kind:            FailureTimeout,
			Summary:         "  provider timeout  ",
			CrashBundlePath: "  /tmp/agh-crash  ",
		}
		normalized := failure.Normalize()
		if normalized.Summary != "provider timeout" {
			t.Fatalf("Normalize().Summary = %q, want provider timeout", normalized.Summary)
		}
		if normalized.CrashBundlePath != "/tmp/agh-crash" {
			t.Fatalf("Normalize().CrashBundlePath = %q, want /tmp/agh-crash", normalized.CrashBundlePath)
		}
		if normalized.IsZero() {
			t.Fatal("IsZero() = true, want false for populated failure")
		}
		if err := normalized.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		clone := CloneSessionFailure(&failure)
		if clone == nil {
			t.Fatal("CloneSessionFailure() = nil, want clone")
		}
		if clone == &failure {
			t.Fatal("CloneSessionFailure() returned original pointer")
		}
		if clone.Summary != "provider timeout" {
			t.Fatalf("CloneSessionFailure().Summary = %q, want normalized summary", clone.Summary)
		}
	})

	t.Run("Should reject malformed failure diagnostics", func(t *testing.T) {
		t.Parallel()

		if ValidFailureKind(FailureKind("bogus")) {
			t.Fatal("ValidFailureKind(bogus) = true, want false")
		}
		if err := (SessionFailure{Summary: "missing kind"}).Validate(); err == nil {
			t.Fatal("Validate(missing kind) error = nil, want validation error")
		}
		if err := (SessionFailure{Kind: FailureKind("bogus")}).Validate(); err == nil {
			t.Fatal("Validate(invalid kind) error = nil, want validation error")
		}
		if !(SessionFailure{}).IsZero() {
			t.Fatal("IsZero(empty) = false, want true")
		}
		if CloneSessionFailure(nil) != nil {
			t.Fatal("CloneSessionFailure(nil) != nil")
		}
	})
}

func TestStoreMemV2OptionalHelpers(t *testing.T) {
	t.Run("Should build negative string clauses and nullable timestamps", func(t *testing.T) {
		t.Parallel()

		clause := NotStringClause("state", " stopped ")
		where, args := BuildClauses(clause)
		if got, want := len(where), 1; got != want {
			t.Fatalf("len(where) = %d, want %d", got, want)
		}
		if where[0] != "state <> ?" {
			t.Fatalf("where[0] = %q, want state <> ?", where[0])
		}
		if got, want := args[0], any("stopped"); got != want {
			t.Fatalf("args[0] = %#v, want %#v", got, want)
		}
		if got, want := NotStringClause("state", "   "), (Clause{}); got != want {
			t.Fatalf("NotStringClause(blank) = %#v, want empty clause", got)
		}

		now := time.Date(2026, 5, 5, 12, 0, 0, 123, time.FixedZone("UTC-3", -3*60*60))
		formatted := FormatNullableTimestamp(now)
		parsed, err := ParseNullableTimestamp(formatted)
		if err != nil {
			t.Fatalf("ParseNullableTimestamp() error = %v", err)
		}
		if parsed == nil {
			t.Fatal("ParseNullableTimestamp() = nil, want timestamp")
		}
		if !parsed.Equal(now.UTC()) {
			t.Fatalf("ParseNullableTimestamp() = %s, want %s", parsed, now.UTC())
		}
		if got := FormatNullableTimestamp(time.Time{}); got != "" {
			t.Fatalf("FormatNullableTimestamp(zero) = %q, want empty", got)
		}
		empty, err := ParseNullableTimestamp("  ")
		if err != nil {
			t.Fatalf("ParseNullableTimestamp(blank) error = %v", err)
		}
		if empty != nil {
			t.Fatalf("ParseNullableTimestamp(blank) = %v, want nil", empty)
		}
	})

	t.Run("Should normalize correlation timestamps and generate identifiers", func(t *testing.T) {
		t.Parallel()

		leaseUntil := time.Date(2026, 5, 5, 12, 0, 0, 0, time.FixedZone("UTC-3", -3*60*60))
		correlation := EventCorrelation{
			TaskID:     " task-1 ",
			LeaseUntil: &leaseUntil,
			ActorKind:  " operator ",
		}.Normalize()
		if correlation.TaskID != "task-1" {
			t.Fatalf("Normalize().TaskID = %q, want task-1", correlation.TaskID)
		}
		if correlation.ActorKind != "operator" {
			t.Fatalf("Normalize().ActorKind = %q, want operator", correlation.ActorKind)
		}
		if correlation.LeaseUntil == nil {
			t.Fatal("Normalize().LeaseUntil = nil, want normalized timestamp")
		}
		if correlation.LeaseUntil.Location() != time.UTC {
			t.Fatalf("Normalize().LeaseUntil location = %s, want UTC", correlation.LeaseUntil.Location())
		}
		if correlation.IsZero() {
			t.Fatal("IsZero(populated correlation) = true, want false")
		}
		if !(EventCorrelation{}).IsZero() {
			t.Fatal("IsZero(empty correlation) = false, want true")
		}

		id := NewID("memv2")
		if len(id) <= len("memv2-") || id[:len("memv2-")] != "memv2-" {
			t.Fatalf("NewID(memv2) = %q, want memv2-*", id)
		}
		if NewID("") == "" {
			t.Fatal("NewID(empty prefix) = empty, want generated identifier")
		}
	})
}
