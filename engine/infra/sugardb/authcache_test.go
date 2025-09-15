package sugardb

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/core"
	sdk "github.com/echovault/sugardb/sugardb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockRepo struct{ mock.Mock }

func (m *mockRepo) CreateUser(ctx context.Context, u *model.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockRepo) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	a := m.Called(ctx, id)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.User), a.Error(1)
}
func (m *mockRepo) GetUserByEmail(ctx context.Context, e string) (*model.User, error) {
	a := m.Called(ctx, e)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(*model.User), a.Error(1)
}
func (m *mockRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	a := m.Called(ctx)
	return a.Get(0).([]*model.User), a.Error(1)
}
func (m *mockRepo) UpdateUser(ctx context.Context, u *model.User) error {
	return m.Called(ctx, u).Error(0)
}
func (m *mockRepo) DeleteUser(ctx context.Context, id core.ID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockRepo) CreateAPIKey(ctx context.Context, k *model.APIKey) error {
	return m.Called(ctx, k).Error(0)
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

func TestSugarAuthCache_BasicLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := sdk.NewSugarDB()
	require.NoError(t, err)
	repo := &mockRepo{}
	cache := NewAuthCachedRepository(repo, db, 200*time.Millisecond).(*AuthCachedRepository)

	key := &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      core.MustNewID(),
		Fingerprint: []byte("fp"),
		CreatedAt:   time.Now(),
	}

	repo.On("GetAPIKeyByHash", ctx, []byte("fp")).Return(key, nil).Once()
	out, err := cache.GetAPIKeyByHash(ctx, []byte("fp"))
	require.NoError(t, err)
	assert.Equal(t, key.ID, out.ID)

	// Next call uses mapping
	repo.On("GetAPIKeyByID", ctx, key.ID).Return(key, nil).Once()
	out2, err := cache.GetAPIKeyByHash(ctx, []byte("fp"))
	require.NoError(t, err)
	assert.Equal(t, key.ID, out2.ID)
	repo.AssertExpectations(t)
}
