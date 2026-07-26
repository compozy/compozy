package memory

import (
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func newOpenTestStore(t *testing.T, globalDir string, opts ...StoreOption) *Store {
	t.Helper()
	store := NewStore(globalDir, opts...)
	if err := store.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("Store.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.CloseCatalog(testutil.Context(t)); err != nil {
			t.Fatalf("Store.CloseCatalog() error = %v", err)
		}
	})
	return store
}
