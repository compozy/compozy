//go:build integration

package resources

import (
	"testing"

	"github.com/compozy/compozy/internal/testutil"
)

func TestTypedStoreIntegrationPersistLoadAndList(t *testing.T) {
	t.Parallel()

	kernel, _ := openTestKernel(t)
	ctx := testutil.Context(t)
	codec := mustJSONCodec(t, testResourceKind, 1024, validateTestTypedSpec)
	store, err := NewStore(kernel, codec)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	record, err := store.Put(ctx, testDaemonActor(), Draft[testTypedSpec]{
		ID:    "integration-tool",
		Scope: ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-integration"},
		Spec:  testTypedSpec{Name: "integration"},
	})
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("record.Version = %d, want %d", got, want)
	}

	loaded, err := store.Get(ctx, testDaemonActor(), record.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := loaded.Spec.Name, "integration"; got != want {
		t.Fatalf("loaded.Spec.Name = %q, want %q", got, want)
	}

	records, err := store.List(ctx, testDaemonActor(), ResourceFilter{
		Scope: &ResourceScope{Kind: ResourceScopeKindWorkspace, ID: "ws-integration"},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(List()) = %d, want %d", got, want)
	}

	rawRecords, err := kernel.ListRaw(ctx, testDaemonActor(), ResourceFilter{Kind: testResourceKind})
	if err != nil {
		t.Fatalf("ListRaw() error = %v", err)
	}
	if got, want := len(rawRecords), 1; got != want {
		t.Fatalf("len(ListRaw()) = %d, want %d", got, want)
	}
}
