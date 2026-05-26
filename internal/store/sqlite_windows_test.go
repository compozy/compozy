//go:build windows

package store

import (
	"strings"
	"testing"
)

func TestSQLiteDSNWindowsDriveLetter(t *testing.T) {
	t.Parallel()
	t.Run("Should format Windows drive-letter absolute path as file URI", func(t *testing.T) {
		t.Parallel()
		path := `C:\Users\user\.compozy\db.sqlite`
		dsn := sqliteDSN(path)

		const wantPrefix = "file:///C:/Users/user/.compozy/db.sqlite"
		if !strings.HasPrefix(dsn, wantPrefix) {
			t.Errorf("sqliteDSN(%q) = %q, want prefix %q", path, dsn, wantPrefix)
		}
	})
}
