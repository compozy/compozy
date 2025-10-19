package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) CreateUser(ctx context.Context, user *model.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.User), a.Error(1)
}
func (m *mockRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	a := m.Called(ctx, email)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.User), a.Error(1)
}
func (m *mockRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	a := m.Called(ctx)
	return a.Get(0).([]*model.User), a.Error(1)
}
func (m *mockRepo) UpdateUser(ctx context.Context, user *model.User) error {
	return m.Called(ctx, user).Error(0)
}
func (m *mockRepo) DeleteUser(ctx context.Context, id core.ID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	return m.Called(ctx, key).Error(0)
}
func (m *mockRepo) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.APIKey), a.Error(1)
}
func (m *mockRepo) GetAPIKeyByHash(ctx context.Context, h []byte) (*model.APIKey, error) {
	a := m.Called(ctx, h)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.APIKey), a.Error(1)
}
func (m *mockRepo) ListAPIKeysByUserID(ctx context.Context, id core.ID) ([]*model.APIKey, error) {
	a := m.Called(ctx, id)
	return a.Get(0).([]*model.APIKey), a.Error(1)
}
func (m *mockRepo) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) DeleteAPIKey(ctx context.Context, id core.ID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) CreateInitialAdminIfNone(ctx context.Context, u *model.User) error {
	return m.Called(ctx, u).Error(0)
}

func newCache(t *testing.T) (*CachedRepository, *mockRepo, *redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mr.FlushAll()
	repo := &mockRepo{}
	c := NewCachedRepository(repo, client, 200*time.Millisecond).(*CachedRepository)
	return c, repo, client, mr
}

func TestAuthCache_FPMappingAndSanitizedIDCache(t *testing.T) {
	cache, repo, _, _ := newCache(t)
	ctx := t.Context()
	key := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      core.MustNewID(),
		Fingerprint: []byte("fp"),
		CreatedAt:   time.Now(),
	}

	// On miss, fetch from repo and map fp->ID; ID cache stores sanitized (no Hash)
	repo.On("GetAPIKeyByHash", ctx, []byte("fp")).Return(key, nil).Once()
	out, err := cache.GetAPIKeyByHash(ctx, []byte("fp"))
	require.NoError(t, err)
	assert.Equal(t, key.ID, out.ID)

	// Second call hits mapping then GetAPIKeyByID
	repo.On("GetAPIKeyByID", ctx, key.ID).Return(key, nil).Once()
	out2, err := cache.GetAPIKeyByHash(ctx, []byte("fp"))
	require.NoError(t, err)
	assert.Equal(t, key.ID, out2.ID)
	repo.AssertExpectations(t)
}

func TestAuthCache_InvalidateOnUpdateDelete(t *testing.T) {
	cache, repo, _, _ := newCache(t)
	ctx := t.Context()
	key := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      core.MustNewID(),
		Fingerprint: []byte("fp"),
		CreatedAt:   time.Now(),
	}

	repo.On("GetAPIKeyByID", ctx, key.ID).Return(key, nil).Once()
	_, _ = cache.GetAPIKeyByID(ctx, key.ID)

	repo.On("UpdateAPIKeyLastUsed", ctx, key.ID).Return(nil).Once()
	require.NoError(t, cache.UpdateAPIKeyLastUsed(ctx, key.ID))

	// Delete will attempt to fetch key by ID to remove fp mapping best-effort
	repo.On("DeleteAPIKey", ctx, key.ID).Return(nil).Once()
	repo.On("GetAPIKeyByID", ctx, key.ID).Return(key, nil).Once()
	require.NoError(t, cache.DeleteAPIKey(ctx, key.ID))
	repo.AssertExpectations(t)
}
