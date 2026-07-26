package resources

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

type benchmarkProjector struct {
	kind      ResourceKind
	dependsOn []ResourceKind
}

func (p benchmarkProjector) Kind() ResourceKind {
	return p.kind
}

func (p benchmarkProjector) DependsOn() []ResourceKind {
	return append([]ResourceKind(nil), p.dependsOn...)
}

func (benchmarkProjector) Build(context.Context, projectionInput) (ProjectionPlan, error) {
	return testPlan{}, nil
}

func (benchmarkProjector) Apply(context.Context, ProjectionPlan) error {
	return nil
}

type benchmarkRawStore struct {
	recordsByKind map[ResourceKind][]RawRecord
}

func (s benchmarkRawStore) PutRaw(context.Context, MutationActor, RawDraft) (RawRecord, error) {
	return RawRecord{}, errors.New("benchmark raw store: PutRaw is unsupported")
}

func (s benchmarkRawStore) DeleteRaw(context.Context, MutationActor, ResourceKind, string, int64) error {
	return errors.New("benchmark raw store: DeleteRaw is unsupported")
}

func (s benchmarkRawStore) ApplySourceSnapshotRaw(context.Context, MutationActor, SourceSnapshot) error {
	return errors.New("benchmark raw store: ApplySourceSnapshotRaw is unsupported")
}

func (s benchmarkRawStore) GetRaw(context.Context, MutationActor, ResourceKind, string) (RawRecord, error) {
	return RawRecord{}, errors.New("benchmark raw store: GetRaw is unsupported")
}

func (s benchmarkRawStore) ListRaw(_ context.Context, _ MutationActor, filter ResourceFilter) ([]RawRecord, error) {
	return append([]RawRecord(nil), s.recordsByKind[filter.Kind]...), nil
}

func BenchmarkBuildListRawQuery(b *testing.B) {
	b.ReportAllocs()

	actor := testExtensionActor("session-bench", "ext-bench", "nonce-bench")
	filter := ResourceFilter{
		Kind:  testResourceKind,
		Scope: &ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-bench"},
		Limit: 64,
	}

	var (
		query string
		args  []any
	)
	for b.Loop() {
		query, args = buildListRawQuery(actor, filter)
	}

	if query == "" || len(args) == 0 {
		b.Fatalf("buildListRawQuery() returned empty query=%q args=%d", query, len(args))
	}
}

func BenchmarkValidateAndCanonicalizeIfRegistered(b *testing.B) {
	b.ReportAllocs()

	registry := NewCodecRegistry()
	codec := mustJSONCodec(b, testResourceKind, 1024, validateTestTypedSpec)
	if err := RegisterCodec(registry, codec); err != nil {
		b.Fatalf("RegisterCodec() error = %v", err)
	}

	scope := ResourceScope{Kind: ResourceScopeKindGlobal}
	raw := []byte(`{"name":" bench-name "}`)

	var (
		canonical []byte
		validated bool
		err       error
	)
	for b.Loop() {
		canonical, validated, err = ValidateAndCanonicalizeIfRegistered(
			context.Background(),
			registry,
			testResourceKind,
			scope,
			raw,
		)
		if err != nil {
			b.Fatalf("ValidateAndCanonicalizeIfRegistered() error = %v", err)
		}
	}

	if !validated || len(canonical) == 0 {
		b.Fatalf(
			"ValidateAndCanonicalizeIfRegistered() validated=%v canonical=%q",
			validated,
			string(canonical),
		)
	}
}

func BenchmarkReconcileScheduleCascade(b *testing.B) {
	b.ReportAllocs()

	root := ResourceKind("tool")
	driver := &reconcileDriver{
		dependents: map[ResourceKind][]ResourceKind{
			root:                              {"agent", "bundle.activation"},
			ResourceKind("agent"):             {"skill"},
			ResourceKind("bundle.activation"): {"automation.job", "bridge.instance"},
			ResourceKind("automation.job"):    {"automation.trigger"},
		},
		topoRank: map[ResourceKind]int{
			root:                               0,
			ResourceKind("agent"):              1,
			ResourceKind("bundle.activation"):  2,
			ResourceKind("skill"):              3,
			ResourceKind("automation.job"):     4,
			ResourceKind("bridge.instance"):    5,
			ResourceKind("automation.trigger"): 6,
		},
	}

	var ordered []ResourceKind
	for b.Loop() {
		ordered = driver.scheduleCascade(root)
	}

	if got, want := len(ordered), 7; got != want {
		b.Fatalf("len(scheduleCascade()) = %d, want %d", got, want)
	}
}

func BenchmarkReconcileBuildProjectionInput(b *testing.B) {
	b.ReportAllocs()

	primaryKind := testResourceKind
	dependencyKind := ResourceKind("bundle")
	driver := &reconcileDriver{
		raw: benchmarkRawStore{recordsByKind: map[ResourceKind][]RawRecord{
			primaryKind:    benchmarkRawRecords(primaryKind, 32),
			dependencyKind: benchmarkRawRecords(dependencyKind, 16),
		}},
		actor: testDaemonActor(),
		projectors: map[ResourceKind]projector{
			primaryKind: benchmarkProjector{
				kind:      primaryKind,
				dependsOn: []ResourceKind{dependencyKind},
			},
		},
	}

	ctx := context.Background()
	var input projectionInput
	for b.Loop() {
		var err error
		input, err = driver.buildProjectionInput(ctx, primaryKind)
		if err != nil {
			b.Fatalf("buildProjectionInput() error = %v", err)
		}
	}

	if got, want := len(input.records), 32; got != want {
		b.Fatalf("len(input.records) = %d, want %d", got, want)
	}
	if got, want := len(input.dependencies[dependencyKind]), 16; got != want {
		b.Fatalf("len(input.dependencies[%q]) = %d, want %d", dependencyKind, got, want)
	}
}

func BenchmarkKernelListRaw(b *testing.B) {
	b.ReportAllocs()

	kernel, _ := openBenchmarkKernel(b)
	ctx := testutil.Context(b)
	actor := testDaemonActor()

	for idx := range 64 {
		if _, err := kernel.PutRaw(ctx, actor, RawDraft{
			Kind:            testResourceKind,
			ID:              fmt.Sprintf("tool-%03d", idx),
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        fmt.Appendf(nil, `{"name":"tool-%03d"}`, idx),
		}); err != nil {
			b.Fatalf("PutRaw(seed %d) error = %v", idx, err)
		}
	}

	filter := ResourceFilter{Kind: testResourceKind, Limit: 50}

	b.ResetTimer()
	for b.Loop() {
		records, err := kernel.ListRaw(ctx, actor, filter)
		if err != nil {
			b.Fatalf("ListRaw() error = %v", err)
		}
		if got, want := len(records), 50; got != want {
			b.Fatalf("len(ListRaw()) = %d, want %d", got, want)
		}
	}
}

func BenchmarkKernelApplySourceSnapshotRaw(b *testing.B) {
	b.ReportAllocs()

	kernel, _ := openBenchmarkKernel(b)
	ctx := testutil.Context(b)
	source := ResourceSource{Kind: ResourceSourceKind("extension"), ID: "bench-source"}
	if err := kernel.ActivateSourceSession(ctx, testDaemonActor(), source, "nonce-bench"); err != nil {
		b.Fatalf("ActivateSourceSession() error = %v", err)
	}

	actor := testExtensionActor("session-bench", source.ID, "nonce-bench")
	drafts := makeBenchmarkSnapshotDrafts(8)

	b.ResetTimer()
	for idx := range b.N {
		drafts[0].SpecJSON = fmt.Appendf(nil, `{"name":"tool-%06d"}`, idx)
		if err := kernel.ApplySourceSnapshotRaw(ctx, actor, SourceSnapshot{
			SourceVersion: int64(idx + 1),
			Records:       drafts,
		}); err != nil {
			b.Fatalf("ApplySourceSnapshotRaw() error = %v", err)
		}
	}
}

func openBenchmarkKernel(b *testing.B, opts ...Option) (*Kernel, *sql.DB) {
	b.Helper()

	db, err := store.OpenSQLiteDatabase(
		testutil.Context(b),
		filepath.Join(b.TempDir(), store.GlobalDatabaseName),
		func(ctx context.Context, db *sql.DB) error {
			return store.Apply(ctx, db, globalTestMigrationStream())
		},
	)
	if err != nil {
		b.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	b.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			b.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	options := append([]Option{
		WithNow(func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		}),
	}, opts...)
	kernel, err := NewKernel(db, options...)
	if err != nil {
		b.Fatalf("NewKernel() error = %v", err)
	}
	return kernel, db
}

func benchmarkRawRecords(kind ResourceKind, count int) []RawRecord {
	records := make([]RawRecord, 0, count)
	for idx := range count {
		records = append(records, RawRecord{
			Kind:      kind,
			ID:        fmt.Sprintf("%s-%03d", kind, idx),
			Version:   int64(idx + 1),
			Scope:     ResourceScope{Kind: ResourceScopeKindGlobal},
			Owner:     ResourceOwner{Kind: ResourceOwnerKind("daemon"), ID: "daemon-control"},
			Source:    ResourceSource{Kind: ResourceSourceKind("daemon"), ID: "system"},
			SpecJSON:  fmt.Appendf(nil, `{"name":"%s-%03d"}`, kind, idx),
			CreatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		})
	}
	return records
}

func makeBenchmarkSnapshotDrafts(count int) []RawDraft {
	drafts := make([]RawDraft, 0, count)
	for idx := range count {
		drafts = append(drafts, RawDraft{
			Kind:            testResourceKind,
			ID:              fmt.Sprintf("tool-%03d", idx),
			Scope:           ResourceScope{Kind: ResourceScopeKindGlobal},
			ExpectedVersion: 0,
			SpecJSON:        fmt.Appendf(nil, `{"name":"tool-%03d"}`, idx),
		})
	}
	return drafts
}
