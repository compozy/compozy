package resources

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

const testResourceKind = ResourceKind("tool")

func TestKernelPutRawCreateAndStaleVersionConflict(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testDaemonActor()

	record, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-alpha",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"alpha"}`),
	})
	if err != nil {
		t.Fatalf("PutRaw(create) error = %v", err)
	}
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("record.Version = %d, want %d", got, want)
	}

	if _, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-alpha",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"alpha-v2"}`),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("PutRaw(stale update) error = %v, want ErrConflict", err)
	}

	if err := kernel.DeleteRaw(ctx, actor, testResourceKind, "tool-alpha", 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteRaw(stale delete) error = %v, want ErrConflict", err)
	}
}

func TestKernelPutRawUpdateDeleteAndNotFound(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testDaemonActor()

	record, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-updatable",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"alpha"}`),
	})
	if err != nil {
		t.Fatalf("PutRaw(create) error = %v", err)
	}

	updated, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-updatable",
		Scope:           ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"},
		ExpectedVersion: record.Version,
		SpecJSON:        []byte(`{"name":"beta"}`),
	})
	if err != nil {
		t.Fatalf("PutRaw(update) error = %v", err)
	}
	if got, want := updated.Version, int64(2); got != want {
		t.Fatalf("updated.Version = %d, want %d", got, want)
	}
	if got, want := updated.Scope, (ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}); got != want {
		t.Fatalf("updated.Scope = %#v, want %#v", got, want)
	}

	if err := kernel.DeleteRaw(ctx, actor, testResourceKind, "tool-updatable", updated.Version); err != nil {
		t.Fatalf("DeleteRaw() error = %v", err)
	}

	if _, err := kernel.GetRaw(ctx, actor, testResourceKind, "tool-updatable"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRaw(after delete) error = %v, want ErrNotFound", err)
	}
}

func TestKernelWriteTransactionsRetryBusyLocks(t *testing.T) {
	t.Run("Should retry PutRaw until the competing write lock is released", func(t *testing.T) {
		t.Parallel()

		kernel, db := openLowBusyTestKernel(t)
		ctx := testutil.Context(t)
		holdResourceWriteLock(ctx, t, db)

		record, err := kernel.PutRaw(ctx, testDaemonActor(), RawDraft{
			Kind:            testResourceKind,
			ID:              "busy-put",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"busy-put"}`),
		})
		if err != nil {
			t.Fatalf("PutRaw() error = %v", err)
		}
		if got, want := record.ID, "busy-put"; got != want {
			t.Fatalf("record.ID = %q, want %q", got, want)
		}
	})

	t.Run("Should retry DeleteRaw until the competing write lock is released", func(t *testing.T) {
		t.Parallel()

		kernel, db := openLowBusyTestKernel(t)
		ctx := testutil.Context(t)
		record, err := kernel.PutRaw(ctx, testDaemonActor(), RawDraft{
			Kind:            testResourceKind,
			ID:              "busy-delete",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"busy-delete"}`),
		})
		if err != nil {
			t.Fatalf("PutRaw(seed) error = %v", err)
		}
		holdResourceWriteLock(ctx, t, db)

		if err := kernel.DeleteRaw(ctx, testDaemonActor(), testResourceKind, record.ID, record.Version); err != nil {
			t.Fatalf("DeleteRaw() error = %v", err)
		}
	})

	t.Run("Should retry source snapshot writes until the competing write lock is released", func(t *testing.T) {
		t.Parallel()

		kernel, db := openLowBusyTestKernel(t)
		ctx := testutil.Context(t)
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "busy-extension"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-busy"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}
		holdResourceWriteLock(ctx, t, db)

		err := kernel.ApplySourceSnapshotRaw(
			ctx,
			testExtensionActor("session-busy", source.ID, "nonce-busy"),
			SourceSnapshot{
				SourceVersion: 1,
				Records: []RawDraft{{
					Kind:            testResourceKind,
					ID:              "busy-snapshot",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"name":"busy-snapshot"}`),
				}},
			},
		)
		if err != nil {
			t.Fatalf("ApplySourceSnapshotRaw() error = %v", err)
		}
	})
}

func TestKernelPutRawStampsDaemonOwnerOverride(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testDaemonActor()
	actor.Owner = ResourceOwner{Kind: ResourceOwnerKind("bundle.activation"), ID: "act-owner"}

	record, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "owned-tool",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"owned"}`),
	})
	if err != nil {
		t.Fatalf("PutRaw() error = %v", err)
	}
	if got, want := record.Owner, actor.Owner; got != want {
		t.Fatalf("record.Owner = %#v, want %#v", got, want)
	}
}

func TestKernelPutRawRejectsExtensionOwnerOverride(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-owned"}
	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-owned"); err != nil {
		t.Fatalf("ActivateSourceSession() error = %v", err)
	}
	actor := testExtensionActor("session-owned", source.ID, "nonce-owned")
	actor.Owner = ResourceOwner{Kind: ResourceOwnerKind("bundle.activation"), ID: "act-owner"}

	err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "extension-owned-tool",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"owned"}`),
		}},
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("ApplySourceSnapshotRaw() error = %v, want ErrPermissionDenied", err)
	}
}

func TestKernelPutRawRejectsInvalidScopeBinding(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testDaemonActor()

	testCases := []struct {
		name    string
		scope   ResourceScope
		wantErr error
	}{
		{
			name:    "omitted scope",
			scope:   ResourceScope{},
			wantErr: ErrValidation,
		},
		{
			name:    "global with scope id",
			scope:   ResourceScope{Kind: ResourceScopeKindGlobal, ID: "ws-1"},
			wantErr: ErrInvalidScopeBinding,
		},
		{
			name:    "workspace without scope id",
			scope:   ResourceScope{Kind: ResourceScopeKindWorkspace},
			wantErr: ErrInvalidScopeBinding,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := kernel.PutRaw(ctx, actor, RawDraft{
				Kind:            testResourceKind,
				ID:              "tool-invalid-" + tc.name,
				Scope:           tc.scope,
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"invalid"}`),
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PutRaw(%s) error = %v, want %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestKernelApplySourceSnapshotRejectsInvalidNonceVersionAndPayloadLimits(t *testing.T) {
	t.Parallel()

	t.Run("Should non-active nonce", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-alpha"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-active"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		err := kernel.ApplySourceSnapshotRaw(
			ctx,
			testExtensionActor("session-1", source.ID, "nonce-stale"),
			SourceSnapshot{
				SourceVersion: 1,
				Records: []RawDraft{{
					Kind:            testResourceKind,
					ID:              "tool-alpha",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"name":"alpha"}`),
				}},
			},
		)
		if !errors.Is(err, ErrSessionNotActive) {
			t.Fatalf("ApplySourceSnapshotRaw(non-active nonce) error = %v, want ErrSessionNotActive", err)
		}
	})

	t.Run("Should stale source version", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-bravo"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-1"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		actor := testExtensionActor("session-2", source.ID, "nonce-1")
		if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{{
				Kind:            testResourceKind,
				ID:              "tool-bravo",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"bravo"}`),
			}},
		}); err != nil {
			t.Fatalf("ApplySourceSnapshotRaw(version 1) error = %v", err)
		}

		err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{{
				Kind:            testResourceKind,
				ID:              "tool-bravo",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"bravo-v2"}`),
			}},
		})
		if !errors.Is(err, ErrStaleSourceVersion) {
			t.Fatalf("ApplySourceSnapshotRaw(stale version) error = %v, want ErrStaleSourceVersion", err)
		}
	})

	t.Run("Should per-record payload limit", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t, WithMaxSpecBytes(8))
		ctx := testutil.Context(t)
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-charlie"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-2"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		err := kernel.ApplySourceSnapshotRaw(ctx, testExtensionActor("session-3", source.ID, "nonce-2"), SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{{
				Kind:            testResourceKind,
				ID:              "tool-charlie",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"too-large"}`),
			}},
		})
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("ApplySourceSnapshotRaw(per-record limit) error = %v, want ErrPayloadTooLarge", err)
		}
	})

	t.Run("Should per-call payload limit", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t, WithMaxSpecBytes(64), WithMaxSnapshotBytes(18))
		ctx := testutil.Context(t)
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-delta"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-3"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		err := kernel.ApplySourceSnapshotRaw(ctx, testExtensionActor("session-4", source.ID, "nonce-3"), SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{
				{
					Kind:            testResourceKind,
					ID:              "tool-delta-1",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"n":"12345"}`),
				},
				{
					Kind:            testResourceKind,
					ID:              "tool-delta-2",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"n":"67890"}`),
				},
			},
		})
		if !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("ApplySourceSnapshotRaw(per-call limit) error = %v, want ErrPayloadTooLarge", err)
		}
	})
}

func TestKernelStampsOwnerAndSourceFromActor(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testOperatorActor()

	record, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-owned",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"owner_kind":"fake","owner_id":"fake","source_kind":"fake","source_id":"fake"}`),
	})
	if err != nil {
		t.Fatalf("PutRaw() error = %v", err)
	}

	if got, want := record.Owner, (ResourceOwner{Kind: ResourceOwnerKind(actor.Kind), ID: actor.ID}); got != want {
		t.Fatalf("record.Owner = %#v, want %#v", got, want)
	}
	if got, want := record.Source, actor.Source; got != want {
		t.Fatalf("record.Source = %#v, want %#v", got, want)
	}
}

func TestKernelActivateSessionNoOpSnapshotAndReset(t *testing.T) {
	t.Parallel()

	kernel, db := openTestKernel(t)
	ctx := testutil.Context(t)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-noop"}

	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-noop"); err != nil {
		t.Fatalf("ActivateSourceSession() error = %v", err)
	}

	actor := testExtensionActor("session-noop", source.ID, "nonce-noop")
	firstSnapshot := SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "tool-noop",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"same"}`),
		}},
	}
	if err := kernel.ApplySourceSnapshotRaw(ctx, actor, firstSnapshot); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(v1) error = %v", err)
	}
	if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
		SourceVersion: 2,
		Records:       firstSnapshot.Records,
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(v2 no-op) error = %v", err)
	}

	record, err := kernel.GetRaw(ctx, actor, testResourceKind, "tool-noop")
	if err != nil {
		t.Fatalf("GetRaw() error = %v", err)
	}
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("record.Version after no-op snapshot = %d, want %d", got, want)
	}

	var lastSnapshotVersion int64
	if err := db.QueryRowContext(
		ctx,
		`SELECT last_snapshot_version FROM resource_source_state WHERE source_kind = ? AND source_id = ?`,
		source.Kind,
		source.ID,
	).Scan(&lastSnapshotVersion); err != nil {
		t.Fatalf("QueryRowContext(resource_source_state) error = %v", err)
	}
	if got, want := lastSnapshotVersion, int64(2); got != want {
		t.Fatalf("last_snapshot_version = %d, want %d", got, want)
	}

	if err := kernel.ResetSource(ctx, testOperatorActor(), source); err != nil {
		t.Fatalf("ResetSource() error = %v", err)
	}

	records, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Source: &source})
	if err != nil {
		t.Fatalf("ListRaw(after reset) error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("records after reset = %#v, want none", records)
	}
}

func TestKernelGetAndListEnforceSourceAndScope(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
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

	if err := kernel.ApplySourceSnapshotRaw(ctx, alphaActor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{
			{
				Kind:            testResourceKind,
				ID:              "alpha-global",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"alpha-global"}`),
			},
			{
				Kind:            testResourceKind,
				ID:              "alpha-workspace",
				Scope:           ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"alpha-workspace"}`),
			},
		},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(alpha) error = %v", err)
	}
	if err := kernel.ApplySourceSnapshotRaw(ctx, bravoActor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "bravo-global",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"bravo-global"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(bravo) error = %v", err)
	}

	records, err := kernel.ListRaw(ctx, alphaActor, ResourceFilter{})
	if err != nil {
		t.Fatalf("ListRaw(alpha) error = %v", err)
	}
	if got, want := len(records), 2; got != want {
		t.Fatalf("len(ListRaw(alpha)) = %d, want %d", got, want)
	}

	if _, err := kernel.GetRaw(
		ctx,
		alphaActor,
		testResourceKind,
		"bravo-global",
	); !errors.Is(
		err,
		ErrPermissionDenied,
	) {
		t.Fatalf("GetRaw(foreign source) error = %v, want ErrPermissionDenied", err)
	}

	workspaceActor := alphaActor
	workspaceActor.MaxScope = ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}
	workspaceActor.GrantedScopes = []ResourceScopeKind{ResourceScopeKindWorkspace}

	workspaceRecords, err := kernel.ListRaw(ctx, workspaceActor, ResourceFilter{})
	if err != nil {
		t.Fatalf("ListRaw(workspace) error = %v", err)
	}
	if got, want := len(workspaceRecords), 1; got != want {
		t.Fatalf("len(ListRaw(workspace)) = %d, want %d", got, want)
	}
	if got, want := workspaceRecords[0].ID, "alpha-workspace"; got != want {
		t.Fatalf("workspace record ID = %q, want %q", got, want)
	}
}

func TestKernelListRawFilterValidationAndSourceOwnerFilters(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	actor := testOperatorActor()

	if _, err := kernel.PutRaw(ctx, actor, RawDraft{
		Kind:            testResourceKind,
		ID:              "tool-filtered",
		Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
		ExpectedVersion: 0,
		SpecJSON:        []byte(`{"name":"filtered"}`),
	}); err != nil {
		t.Fatalf("PutRaw() error = %v", err)
	}

	if _, err := kernel.ListRaw(ctx, actor, ResourceFilter{Limit: -1}); !errors.Is(err, ErrValidation) {
		t.Fatalf("ListRaw(negative limit) error = %v, want ErrValidation", err)
	}

	owner := ResourceOwner{Kind: ResourceOwnerKind(" operator "), ID: " operator-1 "}
	source := ResourceSource{Kind: ResourceSourceKind(" daemon "), ID: " operator-control "}
	records, err := kernel.ListRaw(ctx, actor, ResourceFilter{
		Owner:  &owner,
		Source: &source,
	})
	if err != nil {
		t.Fatalf("ListRaw(owner+source filter) error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(ListRaw(owner+source filter)) = %d, want %d", got, want)
	}

	emptyGrantActor := testExtensionActor("session-empty", "ext-empty", "nonce-empty")
	emptyGrantActor.GrantedKinds = nil
	emptyGrantActor.GrantedScopes = nil
	records, err = kernel.ListRaw(ctx, emptyGrantActor, ResourceFilter{})
	if err != nil {
		t.Fatalf("ListRaw(empty grants) error = %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("ListRaw(empty grants) = %#v, want none", records)
	}
}

func TestKernelApplySourceSnapshotRejectsRecordCountAndEmptyGrants(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t, WithMaxSnapshotRecords(1))
	ctx := testutil.Context(t)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-limits"}
	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-limits"); err != nil {
		t.Fatalf("ActivateSourceSession() error = %v", err)
	}

	actor := testExtensionActor("session-limits", source.ID, "nonce-limits")
	err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{
			{
				Kind:            testResourceKind,
				ID:              "tool-limit-1",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"one"}`),
			},
			{
				Kind:            testResourceKind,
				ID:              "tool-limit-2",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"two"}`),
			},
		},
	})
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("ApplySourceSnapshotRaw(too many records) error = %v, want ErrPayloadTooLarge", err)
	}

	noGrantActor := actor
	noGrantActor.GrantedKinds = nil
	noGrantActor.GrantedScopes = nil
	err = kernel.ApplySourceSnapshotRaw(ctx, noGrantActor, SourceSnapshot{
		SourceVersion: 1,
		Records:       nil,
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("ApplySourceSnapshotRaw(empty grants) error = %v, want ErrPermissionDenied", err)
	}
}

func TestKernelApplySourceSnapshotUpdatesAndDeletesExistingRecords(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-update-delete"}
	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-update-delete"); err != nil {
		t.Fatalf("ActivateSourceSession() error = %v", err)
	}

	actor := testExtensionActor("session-update-delete", source.ID, "nonce-update-delete")
	if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
		SourceVersion: 1,
		Records: []RawDraft{
			{
				Kind:            testResourceKind,
				ID:              "tool-keep",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"old"}`),
			},
			{
				Kind:            testResourceKind,
				ID:              "tool-drop",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"drop"}`),
			},
		},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(v1) error = %v", err)
	}

	if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
		SourceVersion: 2,
		Records: []RawDraft{{
			Kind:            testResourceKind,
			ID:              "tool-keep",
			Scope:           ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"new"}`),
		}},
	}); err != nil {
		t.Fatalf("ApplySourceSnapshotRaw(v2) error = %v", err)
	}

	updated, err := kernel.GetRaw(ctx, actor, testResourceKind, "tool-keep")
	if err != nil {
		t.Fatalf("GetRaw(updated) error = %v", err)
	}
	if got, want := updated.Version, int64(2); got != want {
		t.Fatalf("updated.Version = %d, want %d", got, want)
	}
	if got, want := updated.Scope, (ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}); got != want {
		t.Fatalf("updated.Scope = %#v, want %#v", got, want)
	}

	if _, err := kernel.GetRaw(ctx, actor, testResourceKind, "tool-drop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRaw(deleted) error = %v, want ErrNotFound", err)
	}
}

func TestNewKernelAndAuxiliaryTypesValidation(t *testing.T) {
	t.Parallel()

	if _, err := NewKernel(nil); err == nil {
		t.Fatal("NewKernel(nil) error = nil, want non-nil")
	}

	normalizedSource := (ResourceSource{
		Kind: ResourceSourceKind(" extension "),
		ID:   " ext-alpha ",
	}).Normalize()
	if got, want := normalizedSource, (ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-alpha"}); got != want {
		t.Fatalf("ResourceSource.Normalize() = %#v, want %#v", got, want)
	}
	if err := (ResourceSource{}).Validate("source"); !errors.Is(err, ErrValidation) {
		t.Fatalf("ResourceSource.Validate() error = %v, want ErrValidation", err)
	}

	normalizedOwner := (ResourceOwner{
		Kind: ResourceOwnerKind(" daemon "),
		ID:   " control ",
	}).Normalize()
	if got, want := normalizedOwner, (ResourceOwner{Kind: ResourceOwnerKind("daemon"), ID: "control"}); got != want {
		t.Fatalf("ResourceOwner.Normalize() = %#v, want %#v", got, want)
	}
	if err := (ResourceOwner{}).Validate("owner"); !errors.Is(err, ErrValidation) {
		t.Fatalf("ResourceOwner.Validate() error = %v, want ErrValidation", err)
	}
}

func TestKernelValidationAndAuthorityEdgeCases(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)

	t.Run("Should activate source session rejects extension actor and blank nonce", func(t *testing.T) {
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-edge"}
		if err := kernel.ActivateSourceSession(
			ctx,
			testExtensionActor("session-edge", source.ID, "nonce"),
			source,
			"nonce",
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("ActivateSourceSession(extension actor) error = %v, want ErrPermissionDenied", err)
		}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, " "); !errors.Is(err, ErrValidation) {
			t.Fatalf("ActivateSourceSession(blank nonce) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reset source rejects extension actor", func(t *testing.T) {
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-reset"}
		if err := kernel.ResetSource(
			ctx,
			testExtensionActor("session-reset", source.ID, "nonce"),
			source,
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("ResetSource(extension actor) error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should put rejects scope escalation and source mismatch", func(t *testing.T) {
		workspaceActor := testOperatorActor()
		workspaceActor.MaxScope = ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}
		workspaceActor.GrantedScopes = []ResourceScopeKind{ResourceScopeKindWorkspace}
		workspaceActor.GrantedKinds = []ResourceKind{testResourceKind}

		if _, err := kernel.PutRaw(ctx, workspaceActor, RawDraft{
			Kind:            testResourceKind,
			ID:              "tool-scope-denied",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"denied"}`),
		}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("PutRaw(scope escalation) error = %v, want ErrPermissionDenied", err)
		}

		creator := testDaemonActor()
		record, err := kernel.PutRaw(ctx, creator, RawDraft{
			Kind:            testResourceKind,
			ID:              "tool-source-mismatch",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"alpha"}`),
		})
		if err != nil {
			t.Fatalf("PutRaw(create source mismatch seed) error = %v", err)
		}

		otherSourceActor := testDaemonActor()
		otherSourceActor.Source = ResourceSource{Kind: ResourceSourceKind("daemon"), ID: "other-system"}
		if _, err := kernel.PutRaw(ctx, otherSourceActor, RawDraft{
			Kind:            testResourceKind,
			ID:              "tool-source-mismatch",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: record.Version,
			SpecJSON:        []byte(`{"name":"beta"}`),
		}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("PutRaw(source mismatch) error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should list and get reject invalid source and scope access", func(t *testing.T) {
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-read"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-read"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		actor := testExtensionActor("session-read", source.ID, "nonce-read")
		if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{{
				Kind:            testResourceKind,
				ID:              "tool-read",
				Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
				ExpectedVersion: 0,
				SpecJSON:        []byte(`{"name":"read"}`),
			}},
		}); err != nil {
			t.Fatalf("ApplySourceSnapshotRaw(read seed) error = %v", err)
		}

		foreignSource := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-foreign"}
		if _, err := kernel.ListRaw(
			ctx,
			actor,
			ResourceFilter{Source: &foreignSource},
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("ListRaw(foreign source filter) error = %v, want ErrPermissionDenied", err)
		}

		workspaceActor := actor
		workspaceActor.MaxScope = ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-1"}
		workspaceActor.GrantedScopes = []ResourceScopeKind{ResourceScopeKindWorkspace}
		if _, err := kernel.GetRaw(
			ctx,
			workspaceActor,
			testResourceKind,
			"tool-read",
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("GetRaw(workspace actor reading global record) error = %v, want ErrPermissionDenied", err)
		}
	})

	t.Run("Should snapshot rejects non-extension actor duplicate keys and blank nonce", func(t *testing.T) {
		source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-snapshot-edge"}
		if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-snapshot"); err != nil {
			t.Fatalf("ActivateSourceSession() error = %v", err)
		}

		if err := kernel.ApplySourceSnapshotRaw(
			ctx,
			testDaemonActor(),
			SourceSnapshot{SourceVersion: 1},
		); !errors.Is(
			err,
			ErrPermissionDenied,
		) {
			t.Fatalf("ApplySourceSnapshotRaw(non-extension actor) error = %v, want ErrPermissionDenied", err)
		}

		blankNonceActor := testExtensionActor("session-blank", source.ID, "")
		if err := kernel.ApplySourceSnapshotRaw(
			ctx,
			blankNonceActor,
			SourceSnapshot{SourceVersion: 1},
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("ApplySourceSnapshotRaw(blank nonce) error = %v, want ErrValidation", err)
		}

		actor := testExtensionActor("session-snapshot", source.ID, "nonce-snapshot")
		err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
			SourceVersion: 1,
			Records: []RawDraft{
				{
					Kind:            testResourceKind,
					ID:              "tool-dup",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"name":"dup-1"}`),
				},
				{
					Kind:            testResourceKind,
					ID:              "tool-dup",
					Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
					ExpectedVersion: 0,
					SpecJSON:        []byte(`{"name":"dup-2"}`),
				},
			},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("ApplySourceSnapshotRaw(duplicate keys) error = %v, want ErrValidation", err)
		}
	})
}

func TestKernelHelperPathsAndMissingState(t *testing.T) {
	t.Parallel()

	kernel, db := openTestKernel(t)
	ctx := testutil.Context(t)

	if err := kernel.DeleteRaw(
		ctx,
		testDaemonActor(),
		testResourceKind,
		"missing-record",
		1,
	); !errors.Is(
		err,
		ErrNotFound,
	) {
		t.Fatalf("DeleteRaw(missing) error = %v, want ErrNotFound", err)
	}

	if _, err := normalizeFilter(ResourceFilter{
		Scope: &ResourceScope{Kind: ResourceScopeKindGlobal, ID: "ws-1"},
	}); !errors.Is(err, ErrInvalidScopeBinding) {
		t.Fatalf("normalizeFilter(invalid scope) error = %v, want ErrInvalidScopeBinding", err)
	}

	if _, err := normalizeFilter(ResourceFilter{
		Source: &ResourceSource{},
	}); !errors.Is(err, ErrValidation) {
		t.Fatalf("normalizeFilter(invalid source) error = %v, want ErrValidation", err)
	}

	state, found, err := lookupSourceState(ctx, db, ResourceSource{
		Kind: ResourceSourceKind("extension"),
		ID:   "missing-source",
	})
	if err != nil {
		t.Fatalf("lookupSourceState(missing) error = %v", err)
	}
	if found {
		t.Fatalf("lookupSourceState(missing) found = true, state = %#v, want false", state)
	}

	if err := rollbackTx(nil); err != nil {
		t.Fatalf("rollbackTx(nil) error = %v", err)
	}
	if err := rollbackImmediate(ctx, nil); err != nil {
		t.Fatalf("rollbackImmediate(nil) error = %v", err)
	}
}

func TestKernelLockSourceReleasesIdleEntries(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "ext-lock"}

	unlock := kernel.lockSource(source)

	kernel.sourceLocksMu.Lock()
	if got, want := len(kernel.sourceLocks), 1; got != want {
		kernel.sourceLocksMu.Unlock()
		t.Fatalf("len(sourceLocks) while held = %d, want %d", got, want)
	}
	kernel.sourceLocksMu.Unlock()

	unlock()

	kernel.sourceLocksMu.Lock()
	defer kernel.sourceLocksMu.Unlock()
	if got := len(kernel.sourceLocks); got != 0 {
		t.Fatalf("len(sourceLocks) after unlock = %d, want 0", got)
	}
}

func TestKernelAdditionalValidationBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should constructor rejects invalid option state", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name string
			opt  Option
		}{
			{
				name: "nil clock",
				opt: func(k *Kernel) {
					k.now = nil
				},
			},
			{
				name: "zero max spec",
				opt: func(k *Kernel) {
					k.maxSpecBytes = 0
				},
			},
			{
				name: "zero max snapshot records",
				opt: func(k *Kernel) {
					k.maxSnapshotRecords = 0
				},
			},
			{
				name: "zero max snapshot bytes",
				opt: func(k *Kernel) {
					k.maxSnapshotBytes = 0
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if _, err := NewKernel(&sql.DB{}, tc.opt); err == nil {
					t.Fatalf("NewKernel(%s) error = nil, want non-nil", tc.name)
				}
			})
		}
	})

	t.Run("Should direct mutation validation", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)

		invalidSourceActor := testDaemonActor()
		invalidSourceActor.Source = ResourceSource{}
		if _, err := kernel.PutRaw(ctx, invalidSourceActor, RawDraft{
			Kind:            testResourceKind,
			ID:              "tool-invalid-source",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        []byte(`{"name":"invalid-source"}`),
		}); !errors.Is(err, ErrValidation) {
			t.Fatalf("PutRaw(invalid actor source) error = %v, want ErrValidation", err)
		}

		if _, err := kernel.PutRaw(ctx, testDaemonActor(), RawDraft{
			Kind:            testResourceKind,
			ID:              "tool-missing-update",
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 1,
			SpecJSON:        []byte(`{"name":"missing"}`),
		}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("PutRaw(missing update) error = %v, want ErrNotFound", err)
		}

		if err := kernel.DeleteRaw(
			ctx,
			testDaemonActor(),
			testResourceKind,
			"tool-negative",
			-1,
		); !errors.Is(
			err,
			ErrValidation,
		) {
			t.Fatalf("DeleteRaw(negative version) error = %v, want ErrValidation", err)
		}

		if err := kernel.DeleteRaw(
			ctx,
			testExtensionActor("session-direct-delete", "ext-delete", "nonce-delete"),
			testResourceKind,
			"tool-direct-delete",
			1,
		); !errors.Is(err, ErrDirectMutationNotAllowed) {
			t.Fatalf("DeleteRaw(extension actor) error = %v, want ErrDirectMutationNotAllowed", err)
		}
	})

	t.Run("Should read and source management validation", func(t *testing.T) {
		t.Parallel()

		kernel, _ := openTestKernel(t)
		ctx := testutil.Context(t)

		if _, err := kernel.GetRaw(ctx, testDaemonActor(), testResourceKind, " "); !errors.Is(err, ErrValidation) {
			t.Fatalf("GetRaw(blank id) error = %v, want ErrValidation", err)
		}

		limitedActor := testExtensionActor("session-read-filter", "ext-read-filter", "nonce-read-filter")
		limitedActor.GrantedKinds = []ResourceKind{ResourceKind("other")}
		if _, err := kernel.ListRaw(ctx, limitedActor, ResourceFilter{
			Kind: testResourceKind,
		}); !errors.Is(err, ErrPermissionDenied) {
			t.Fatalf("ListRaw(denied kind filter) error = %v, want ErrPermissionDenied", err)
		}

		if err := kernel.ResetSource(ctx, testDaemonActor(), ResourceSource{}); !errors.Is(err, ErrValidation) {
			t.Fatalf("ResetSource(invalid source) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should join cleanup error combines errors", func(t *testing.T) {
		t.Parallel()

		cleanupErr := errors.New("cleanup")

		var err error
		joinCleanupError(&err, cleanupErr)
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("joinCleanupError(nil target) error = %v, want cleanup error", err)
		}

		primaryErr := errors.New("primary")
		err = primaryErr
		joinCleanupError(&err, cleanupErr)
		if !errors.Is(err, primaryErr) || !errors.Is(err, cleanupErr) {
			t.Fatalf("joinCleanupError(joined) error = %v, want both primary and cleanup errors", err)
		}
	})
}

func openTestKernel(t *testing.T, opts ...Option) (*Kernel, *sql.DB) {
	t.Helper()

	db, err := store.OpenSQLiteDatabase(
		testutil.Context(t),
		filepath.Join(t.TempDir(), store.GlobalDatabaseName),
		func(ctx context.Context, db *sql.DB) error {
			return store.Apply(ctx, db, globalTestMigrationStream())
		},
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	options := append([]Option{
		WithNow(func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		}),
	}, opts...)
	kernel, err := NewKernel(db, options...)
	if err != nil {
		t.Fatalf("NewKernel() error = %v", err)
	}
	return kernel, db
}

func openLowBusyTestKernel(t *testing.T, opts ...Option) (*Kernel, *sql.DB) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	dsnURL := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(dbPath),
	}
	query := dsnURL.Query()
	query.Add("_pragma", "busy_timeout(1)")
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")
	dsnURL.RawQuery = query.Encode()

	db, err := sql.Open("sqlite", dsnURL.String())
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(8)
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	ctx := testutil.Context(t)
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("db.PingContext() error = %v", err)
	}
	if err := store.Apply(ctx, db, globalTestMigrationStream()); err != nil {
		t.Fatalf("store.Apply() error = %v", err)
	}

	options := append([]Option{
		WithNow(func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		}),
	}, opts...)
	kernel, err := NewKernel(db, options...)
	if err != nil {
		t.Fatalf("NewKernel() error = %v", err)
	}
	return kernel, db
}

func holdResourceWriteLock(ctx context.Context, t *testing.T, db *sql.DB) {
	t.Helper()

	lockConn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("db.Conn() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := lockConn.Close(); closeErr != nil {
			t.Fatalf("lockConn.Close() error = %v", closeErr)
		}
	})

	if _, err := lockConn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE lock error = %v", err)
	}

	releaseDone := make(chan error, 1)
	timer := time.AfterFunc(10*time.Millisecond, func() {
		_, commitErr := lockConn.ExecContext(ctx, "COMMIT")
		releaseDone <- commitErr
	})
	t.Cleanup(func() {
		if timer.Stop() {
			if _, err := lockConn.ExecContext(ctx, "COMMIT"); err != nil {
				t.Fatalf("manual lock release error = %v", err)
			}
			return
		}
		if err := <-releaseDone; err != nil {
			t.Fatalf("timed lock release error = %v", err)
		}
	})
}

func testDaemonActor() MutationActor {
	return MutationActor{
		Kind:     MutationActorKindDaemon,
		ID:       "daemon-control",
		Source:   ResourceSource{Kind: ResourceSourceKind("daemon"), ID: "system"},
		MaxScope: ResourceScope{Kind: ResourceScopeKindGlobal},
	}
}

func testOperatorActor() MutationActor {
	return MutationActor{
		Kind:     MutationActorKindOperator,
		ID:       "operator-1",
		Source:   ResourceSource{Kind: ResourceSourceKind("daemon"), ID: "operator-control"},
		MaxScope: ResourceScope{Kind: ResourceScopeKindGlobal},
	}
}

func testExtensionActor(sessionID string, sourceID string, nonce string) MutationActor {
	return MutationActor{
		Kind:          MutationActorKindExtension,
		ID:            sessionID,
		SessionNonce:  nonce,
		Source:        ResourceSource{Kind: ResourceSourceKind("extension"), ID: sourceID},
		MaxScope:      ResourceScope{Kind: ResourceScopeKindGlobal},
		GrantedKinds:  []ResourceKind{testResourceKind},
		GrantedScopes: []ResourceScopeKind{ResourceScopeKindGlobal, ResourceScopeKindWorkspace},
	}
}
