package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDSN(t *testing.T) {
	t.Run("Should build DSN for file path with pragmas", func(t *testing.T) {
		d := buildDSN("/tmp/test.db")
		assert.Contains(t, d, "file:/tmp/test.db")
		assert.Contains(t, d, "_pragma=journal_mode(WAL)")
		assert.Contains(t, d, "_pragma=foreign_keys(ON)")
		assert.Contains(t, d, "_pragma=busy_timeout(5000)")
	})
	t.Run("Should build DSN for in-memory shared cache", func(t *testing.T) {
		d := buildDSN(":memory:")
		assert.Contains(t, d, "file::memory:?cache=shared")
	})
}

func TestMigrationsAndTimestamps(t *testing.T) {
	t.Run("Should apply migrations and maintain updated_at via trigger", func(t *testing.T) {
		ctx := context.Background()
		s, err := NewStore(ctx, ":memory:")
		require.NoError(t, err)
		defer s.Close(ctx)
		err = ApplyMigrations(ctx, s.DB())
		require.NoError(t, err)
		db := s.DB()
		_, err = db.ExecContext(
			ctx,
			`INSERT INTO workflow_states (workflow_exec_id, workflow_id, status) VALUES ('w1','wf','running')`,
		)
		require.NoError(t, err)
		var created1, updated1 string
		err = db.QueryRowContext(ctx, `SELECT datetime(created_at), datetime(updated_at) FROM workflow_states WHERE workflow_exec_id='w1'`).
			Scan(&created1, &updated1)
		require.NoError(t, err)
		time.Sleep(1100 * time.Millisecond)
		_, err = db.ExecContext(ctx, `UPDATE workflow_states SET status='completed' WHERE workflow_exec_id='w1'`)
		require.NoError(t, err)
		var created2, updated2 string
		err = db.QueryRowContext(ctx, `SELECT datetime(created_at), datetime(updated_at) FROM workflow_states WHERE workflow_exec_id='w1'`).
			Scan(&created2, &updated2)
		require.NoError(t, err)
		assert.Equal(t, created1, created2)
		assert.NotEqual(t, updated1, updated2)
	})
}

func TestJSONHelpers(t *testing.T) {
	t.Run("Should marshal and unmarshal JSON TEXT", func(t *testing.T) {
		type S struct {
			A int    `json:"a"`
			B string `json:"b"`
		}
		in := &S{A: 42, B: "x"}
		b, err := ToJSONText(in)
		require.NoError(t, err)
		var out *S
		err = FromJSONText(b, &out)
		require.NoError(t, err)
		require.NotNil(t, out)
		assert.Equal(t, in.A, out.A)
		assert.Equal(t, in.B, out.B)
	})
}

func TestPlaceholderBuilder(t *testing.T) {
	t.Run("Should build question list", func(t *testing.T) {
		got := questionList(3)
		assert.Equal(t, "?,?,?", got)
	})
}
