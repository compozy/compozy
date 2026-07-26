//go:build integration

package resources

import (
	"errors"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestKernelSnapshotSequenceConflictAndResetIntegration(t *testing.T) {
	t.Parallel()

	kernel, db := openTestKernel(t)
	ctx := testutil.Context(t)

	sourceAlpha := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-alpha"}
	sourceBravo := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-bravo"}

	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), sourceAlpha, "nonce-alpha"); err != nil {
		t.Fatalf("ActivateSourceSession(alpha) error = %v", err)
	}
	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), sourceBravo, "nonce-bravo"); err != nil {
		t.Fatalf("ActivateSourceSession(bravo) error = %v", err)
	}

	alphaActor := testExtensionActor("session-alpha", sourceAlpha.ID, "nonce-alpha")
	bravoActor := testExtensionActor("session-bravo", sourceBravo.ID, "nonce-bravo")

	if _, err := kernel.PutRaw(ctx, testDaemonActor(), RawDraft{
		Kind:            testResourceKind,
		ID:              "daemon-owned",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"daemon-owned"}`),
	}); err != nil {
		t.Fatalf("PutRaw(daemon-owned) error = %v", err)
	}

	if err := kernel.ApplySourceSnapshotRaw(ctx, bravoActor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "foreign-owned",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"foreign-owned"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(bravo/v1) error = %v", err)
	}

	if err := kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "alpha-v1",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"alpha-v1"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(alpha/v1) error = %v", err)
	}
	if err := kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 2,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "alpha-v2",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"alpha-v2"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(alpha/v2) error = %v", err)
	}

	alphaRecords, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Source: &sourceAlpha})
	if err != nil {
		t.Fatalf("ListRaw(alpha source) error = %v", err)
	}
	if got, want := len(alphaRecords), 1; got != want {
		t.Fatalf("len(alpha source records) = %d, want %d", got, want)
	}
	if got, want := alphaRecords[0].ID, "alpha-v2"; got != want {
		t.Fatalf("alpha source record ID = %q, want %q", got, want)
	}

	err = kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 3,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "daemon-owned",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"conflict-daemon"}`),
		}},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("ApplySourceSnapshotRaw(daemon collision) error = %v, want ErrConflict", err)
	}

	err = kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 3,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "foreign-owned",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"conflict-foreign"}`),
		}},
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("ApplySourceSnapshotRaw(foreign collision) error = %v, want ErrConflict", err)
	}

	if err := kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 3,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "alpha-v2",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"alpha-v2"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(alpha/v3 retry) error = %v", err)
	}

	if err := kernel.ResetSource(ctx, testOperatorActor(), sourceAlpha); err != nil {
		t.Fatalf("ResetSource(alpha) error = %v", err)
	}

	alphaRecordsAfterReset, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Source: &sourceAlpha})
	if err != nil {
		t.Fatalf("ListRaw(alpha source after reset) error = %v", err)
	}
	if len(alphaRecordsAfterReset) != 0 {
		t.Fatalf("alpha records after reset = %#v, want none", alphaRecordsAfterReset)
	}

	var sourceStateCount int
	if err := db.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM resource_source_state WHERE source_kind = ? AND source_id = ?`,
		sourceAlpha.Kind,
		sourceAlpha.ID,
	).Scan(&sourceStateCount); err != nil {
		t.Fatalf("QueryRowContext(resource_source_state) error = %v", err)
	}
	if got, want := sourceStateCount, 0; got != want {
		t.Fatalf("resource_source_state rows after reset = %d, want %d", got, want)
	}
}
