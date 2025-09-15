package sqlite

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/require"
)

func TestSQLiteAuthRepo_BasicCRUD(t *testing.T) {
	ctx := context.Background()
	st, err := NewStore(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close(ctx) })
	require.NoError(t, ApplyMigrations(ctx, st.DB()))

	repo := NewAuthRepo(st.DB())

	// Create user
	u := &model.User{ID: core.MustNewID(), Email: "admin@example.com", Role: model.RoleAdmin, CreatedAt: time.Now()}
	require.NoError(t, repo.CreateUser(ctx, u))

	// Create API key with fingerprint
	fp := sha256.Sum256([]byte("secret"))
	k := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      u.ID,
		Hash:        []byte("hash-bytes"),
		Prefix:      "cpzy_",
		Fingerprint: fp[:],
		CreatedAt:   time.Now(),
	}
	require.NoError(t, repo.CreateAPIKey(ctx, k))

	// Lookup by fingerprint
	k2, err := repo.GetAPIKeyByHash(ctx, k.Fingerprint)
	require.NoError(t, err)
	require.Equal(t, k.ID, k2.ID)

	// Update last used
	require.NoError(t, repo.UpdateAPIKeyLastUsed(ctx, k.ID))

	// Delete API key
	require.NoError(t, repo.DeleteAPIKey(ctx, k.ID))
}
