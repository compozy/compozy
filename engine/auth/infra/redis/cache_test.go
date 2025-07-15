package redis

import (
	"context"
	"encoding/json"
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

// MockRepository implements uc.Repository for testing
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) GetUserByID(ctx context.Context, id core.ID) (*model.User, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockRepository) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockRepository) ListUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *MockRepository) UpdateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockRepository) DeleteUser(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockRepository) GetAPIKeyByID(ctx context.Context, id core.ID) (*model.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockRepository) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockRepository) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*model.APIKey), args.Error(1)
}

func (m *MockRepository) UpdateAPIKeyLastUsed(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRepository) DeleteAPIKey(ctx context.Context, id core.ID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Test helpers
func setupTestCache(t *testing.T) (*CachedRepository, *MockRepository, *redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := &MockRepository{}
	cache := NewCachedRepository(mockRepo, client, 30*time.Second).(*CachedRepository)
	return cache, mockRepo, client, mr
}

func createTestAPIKey() *model.APIKey {
	return &model.APIKey{
		ID:          core.MustNewID(),
		UserID:      core.MustNewID(),
		Fingerprint: []byte("test-fingerprint"),
		CreatedAt:   time.Now(),
	}
}

func TestCachedRepository_GetAPIKeyByHash_Caching(t *testing.T) {
	cache, mockRepo, _, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()
	hash := testKey.Fingerprint

	t.Run("Should cache API key on first retrieval", func(t *testing.T) {
		mockRepo.On("GetAPIKeyByHash", ctx, hash).Return(testKey, nil).Once()
		result, err := cache.GetAPIKeyByHash(ctx, hash)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result.ID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return cached API key on second retrieval", func(t *testing.T) {
		// No mock expectation - should not call underlying repo
		result, err := cache.GetAPIKeyByHash(ctx, hash)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result.ID)
		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_GetAPIKeyByID_Caching(t *testing.T) {
	cache, mockRepo, _, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should cache API key by ID on first retrieval", func(t *testing.T) {
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		result, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result.ID)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return cached API key by ID on second retrieval", func(t *testing.T) {
		// No mock expectation - should not call underlying repo
		result, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result.ID)
		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_InvalidateAPIKeyCache(t *testing.T) {
	cache, mockRepo, client, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should invalidate both ID and hash-based cache entries", func(t *testing.T) {
		// First, cache the key by both ID and hash
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Twice()
		mockRepo.On("GetAPIKeyByHash", ctx, testKey.Fingerprint).Return(testKey, nil).Once()

		// Cache by ID
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Cache by hash
		_, err = cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
		require.NoError(t, err)

		// Verify cache entries exist
		idCacheKey := "auth:apikey:id:" + testKey.ID.String()
		hashCacheKey := cache.cacheKey(testKey.Fingerprint)

		idExists := client.Exists(ctx, idCacheKey).Val()
		hashExists := client.Exists(ctx, hashCacheKey).Val()
		assert.Equal(t, int64(1), idExists)
		assert.Equal(t, int64(1), hashExists)

		// Invalidate cache
		err = cache.invalidateAPIKeyCache(ctx, testKey.ID)
		require.NoError(t, err)

		// Verify cache entries are deleted
		idExists = client.Exists(ctx, idCacheKey).Val()
		hashExists = client.Exists(ctx, hashCacheKey).Val()
		assert.Equal(t, int64(0), idExists)
		assert.Equal(t, int64(0), hashExists)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle missing API key gracefully", func(t *testing.T) {
		nonExistentID := core.MustNewID()

		// Mock the GetAPIKeyByID to return an error (key not found)
		mockRepo.On("GetAPIKeyByID", ctx, nonExistentID).Return(nil, assert.AnError).Once()

		// Should not fail even if API key doesn't exist
		err := cache.invalidateAPIKeyCache(ctx, nonExistentID)
		require.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should handle Redis deletion errors", func(t *testing.T) {
		cache, mockRepo, _, mr := setupTestCache(t)

		// Cache a key first
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Simulate Redis error by closing the server
		mr.Close()

		// Invalidation should return an error when Redis is down
		err = cache.invalidateAPIKeyCache(ctx, testKey.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete ID-based cache entry")

		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_UpdateAPIKeyLastUsed_CacheInvalidation(t *testing.T) {
	cache, mockRepo, client, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should invalidate cache after successful update", func(t *testing.T) {
		// First cache the key
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Twice()
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Verify cache exists
		idCacheKey := "auth:apikey:id:" + testKey.ID.String()
		exists := client.Exists(ctx, idCacheKey).Val()
		assert.Equal(t, int64(1), exists)

		// Mock the update operation
		mockRepo.On("UpdateAPIKeyLastUsed", ctx, testKey.ID).Return(nil).Once()

		// Update last used - should invalidate cache
		err = cache.UpdateAPIKeyLastUsed(ctx, testKey.ID)
		require.NoError(t, err)

		// Verify cache was invalidated
		exists = client.Exists(ctx, idCacheKey).Val()
		assert.Equal(t, int64(0), exists)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should continue with update even if cache invalidation fails", func(t *testing.T) {
		cache, mockRepo, _, mr := setupTestCache(t)

		// Cache a key first
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Close Redis to simulate failure
		mr.Close()

		// Mock the update operation to succeed
		mockRepo.On("UpdateAPIKeyLastUsed", ctx, testKey.ID).Return(nil).Once()

		// Update should succeed even if cache invalidation fails
		err = cache.UpdateAPIKeyLastUsed(ctx, testKey.ID)
		require.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error if underlying update fails", func(t *testing.T) {
		updateErr := assert.AnError
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		mockRepo.On("UpdateAPIKeyLastUsed", ctx, testKey.ID).Return(updateErr).Once()

		err := cache.UpdateAPIKeyLastUsed(ctx, testKey.ID)
		require.Error(t, err)
		assert.Equal(t, updateErr, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_DeleteAPIKey_CacheInvalidation(t *testing.T) {
	cache, mockRepo, client, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should invalidate cache after successful deletion", func(t *testing.T) {
		// First cache the key
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Twice()
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Verify cache exists
		idCacheKey := "auth:apikey:id:" + testKey.ID.String()
		exists := client.Exists(ctx, idCacheKey).Val()
		assert.Equal(t, int64(1), exists)

		// Mock the delete operation
		mockRepo.On("DeleteAPIKey", ctx, testKey.ID).Return(nil).Once()

		// Delete API key - should invalidate cache
		err = cache.DeleteAPIKey(ctx, testKey.ID)
		require.NoError(t, err)

		// Verify cache was invalidated
		exists = client.Exists(ctx, idCacheKey).Val()
		assert.Equal(t, int64(0), exists)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should continue with deletion even if cache invalidation fails", func(t *testing.T) {
		cache, mockRepo, _, mr := setupTestCache(t)

		// Cache a key first
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		_, err := cache.GetAPIKeyByID(ctx, testKey.ID)
		require.NoError(t, err)

		// Close Redis to simulate failure
		mr.Close()

		// Mock the delete operation to succeed
		mockRepo.On("DeleteAPIKey", ctx, testKey.ID).Return(nil).Once()

		// Delete should succeed even if cache invalidation fails
		err = cache.DeleteAPIKey(ctx, testKey.ID)
		require.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Should return error if underlying deletion fails", func(t *testing.T) {
		deleteErr := assert.AnError
		mockRepo.On("GetAPIKeyByID", ctx, testKey.ID).Return(testKey, nil).Once()
		mockRepo.On("DeleteAPIKey", ctx, testKey.ID).Return(deleteErr).Once()

		err := cache.DeleteAPIKey(ctx, testKey.ID)
		require.Error(t, err)
		assert.Equal(t, deleteErr, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_CacheKeyGeneration(t *testing.T) {
	cache, _, client, _ := setupTestCache(t)
	_ = client // We only need cache for this test

	t.Run("Should generate consistent cache keys", func(t *testing.T) {
		hash1 := []byte("test-hash")
		hash2 := []byte("test-hash")

		key1 := cache.cacheKey(hash1)
		key2 := cache.cacheKey(hash2)

		assert.Equal(t, key1, key2)
		assert.Contains(t, key1, "auth:apikey:hash:")
	})

	t.Run("Should generate different cache keys for different hashes", func(t *testing.T) {
		hash1 := []byte("hash1")
		hash2 := []byte("hash2")

		key1 := cache.cacheKey(hash1)
		key2 := cache.cacheKey(hash2)

		assert.NotEqual(t, key1, key2)
	})
}

func TestCachedRepository_CacheExpiration(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	mockRepo := &MockRepository{}

	// Create cache with very short TTL for testing
	shortTTL := 100 * time.Millisecond
	cache := NewCachedRepository(mockRepo, client, shortTTL).(*CachedRepository)

	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should cache expire after TTL", func(t *testing.T) {
		// First call should cache
		mockRepo.On("GetAPIKeyByHash", ctx, testKey.Fingerprint).Return(testKey, nil).Once()
		result1, err := cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result1.ID)

		// Fast forward miniredis time to expire the cache
		mr.FastForward(shortTTL + 10*time.Millisecond)

		// Second call should hit database again due to expiration
		mockRepo.On("GetAPIKeyByHash", ctx, testKey.Fingerprint).Return(testKey, nil).Once()
		result2, err := cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, result2.ID)

		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_ConcurrentAccess(t *testing.T) {
	cache, mockRepo, _, _ := setupTestCache(t)
	ctx := context.Background()
	testKey := createTestAPIKey()

	t.Run("Should handle concurrent cache access safely", func(t *testing.T) {
		// Pre-cache the key to avoid race conditions with mock expectations
		mockRepo.On("GetAPIKeyByHash", ctx, testKey.Fingerprint).Return(testKey, nil).Once()
		_, err := cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
		require.NoError(t, err)

		const numGoroutines = 10
		results := make(chan *model.APIKey, numGoroutines)
		errors := make(chan error, numGoroutines)

		// Launch concurrent requests - these should all hit cache
		for i := 0; i < numGoroutines; i++ {
			go func() {
				result, err := cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}()
		}

		// Collect results
		var successCount int
		for i := 0; i < numGoroutines; i++ {
			select {
			case result := <-results:
				assert.Equal(t, testKey.ID, result.ID)
				successCount++
			case err := <-errors:
				t.Errorf("Unexpected error: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for results")
			}
		}

		assert.Equal(t, numGoroutines, successCount)
		mockRepo.AssertExpectations(t)
	})
}

func TestCachedRepository_JSONMarshaling(t *testing.T) {
	cache, mockRepo, client, _ := setupTestCache(t)
	ctx := context.Background()

	t.Run("Should handle complex API key data correctly", func(t *testing.T) {
		testKey := &model.APIKey{
			ID:          core.MustNewID(),
			UserID:      core.MustNewID(),
			Fingerprint: []byte("complex-fingerprint-with-special-chars-!@#$%"),
			CreatedAt:   time.Now(),
		}

		mockRepo.On("GetAPIKeyByHash", ctx, testKey.Fingerprint).Return(testKey, nil).Once()

		// Cache the key
		result, err := cache.GetAPIKeyByHash(ctx, testKey.Fingerprint)
		require.NoError(t, err)

		// Verify all fields are preserved
		assert.Equal(t, testKey.ID, result.ID)
		assert.Equal(t, testKey.UserID, result.UserID)
		assert.Equal(t, testKey.Fingerprint, result.Fingerprint)
		assert.WithinDuration(t, testKey.CreatedAt, result.CreatedAt, time.Second)

		// Verify cached data is valid JSON
		cacheKey := cache.cacheKey(testKey.Fingerprint)
		cachedData := client.Get(ctx, cacheKey).Val()

		var unmarshaledKey model.APIKey
		err = json.Unmarshal([]byte(cachedData), &unmarshaledKey)
		require.NoError(t, err)
		assert.Equal(t, testKey.ID, unmarshaledKey.ID)

		mockRepo.AssertExpectations(t)
	})
}
