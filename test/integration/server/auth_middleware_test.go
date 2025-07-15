package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	authmiddleware "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	"github.com/compozy/compozy/engine/infra/server/middleware/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// MockRepository is a mock implementation of the repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockRepository) GetUserByID(ctx context.Context, userID core.ID) (*model.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockRepository) UpdateAPIKeyLastUsed(ctx context.Context, keyID core.ID) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

// Add stub methods to satisfy the Repository interface
func (m *MockRepository) CreateUser(_ context.Context, _ *model.User) error { return nil }
func (m *MockRepository) GetUserByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, nil
}
func (m *MockRepository) ListUsers(_ context.Context) ([]*model.User, error)    { return nil, nil }
func (m *MockRepository) UpdateUser(_ context.Context, _ *model.User) error     { return nil }
func (m *MockRepository) DeleteUser(_ context.Context, _ core.ID) error         { return nil }
func (m *MockRepository) CreateAPIKey(_ context.Context, _ *model.APIKey) error { return nil }
func (m *MockRepository) GetAPIKeyByID(_ context.Context, _ core.ID) (*model.APIKey, error) {
	return nil, nil
}
func (m *MockRepository) ListAPIKeysByUserID(_ context.Context, _ core.ID) ([]*model.APIKey, error) {
	return nil, nil
}
func (m *MockRepository) DeleteAPIKey(_ context.Context, _ core.ID) error { return nil }

func TestAuthMiddleware_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gin.SetMode(gin.TestMode)

	// Create test user
	userID, _ := core.NewID()
	user := &model.User{
		ID:    userID,
		Email: "test@example.com",
		Role:  model.RoleUser,
	}
	apiKey := "test-api-key"

	// Create API key hash for storage
	hash := sha256.Sum256([]byte(apiKey))
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	keyID, _ := core.NewID()
	storedAPIKey := &model.APIKey{
		ID:     keyID,
		UserID: userID,
		Hash:   bcryptHash,
	}

	// Create mock repository
	mockRepo := new(MockRepository)
	mockRepo.On("GetAPIKeyByHash", mock.Anything, hash[:]).Return(storedAPIKey, nil)
	mockRepo.On("GetAPIKeyByHash", mock.Anything, mock.MatchedBy(func(h []byte) bool {
		invalidHash := sha256.Sum256([]byte("invalid_key"))
		return bytes.Equal(h, invalidHash[:])
	})).Return(nil, fmt.Errorf("key not found"))
	mockRepo.On("GetUserByID", mock.Anything, userID).Return(user, nil)
	mockRepo.On("UpdateAPIKeyLastUsed", mock.Anything, keyID).Return(nil)

	// Create auth factory and middleware
	authFactory := uc.NewFactory(mockRepo)
	authManager := authmiddleware.NewManager(authFactory)

	// Create rate limit config with low limits for testing
	config := &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{
			Limit:  100,
			Period: 1 * time.Minute,
		},
		APIKeyRate: ratelimit.RateConfig{
			Limit:  5,
			Period: 1 * time.Minute,
		},
		RouteRates: map[string]ratelimit.RateConfig{},
		Prefix:     "test:ratelimit:",
		MaxRetry:   3,
	}

	rateLimitManager, err := ratelimit.NewManager(config, nil)
	require.NoError(t, err)

	// Create router with auth and rate limit middleware
	router := gin.New()
	router.Use(authManager.Middleware())
	router.Use(rateLimitManager.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	t.Run("Should rate limit authenticated requests by API key", func(t *testing.T) {
		// Make 5 requests (within limit)
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}

		// Make 6th request (should be rate limited)
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		assert.Contains(t, w.Body.String(), "Rate limit exceeded")

		// Verify rate limit headers
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
		assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("Should track rate limits separately for different API keys", func(t *testing.T) {
		// Create another user and API key
		userID2, _ := core.NewID()
		user2 := &model.User{
			ID:    userID2,
			Email: "test2@example.com",
			Role:  model.RoleUser,
		}
		apiKey2 := "test-api-key-2"

		// Set up mock for second API key
		hash2 := sha256.Sum256([]byte(apiKey2))
		bcryptHash2, _ := bcrypt.GenerateFromPassword([]byte(apiKey2), bcrypt.DefaultCost)
		keyID2, _ := core.NewID()
		storedAPIKey2 := &model.APIKey{
			ID:     keyID2,
			UserID: userID2,
			Hash:   bcryptHash2,
		}
		mockRepo.On("GetAPIKeyByHash", mock.Anything, hash2[:]).Return(storedAPIKey2, nil)
		mockRepo.On("GetUserByID", mock.Anything, userID2).Return(user2, nil)
		mockRepo.On("UpdateAPIKeyLastUsed", mock.Anything, keyID2).Return(nil)

		// Since we're using in-memory store, rate limits are tracked separately per key
		// Make requests with second API key (should succeed)
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+apiKey2)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Different API key should have separate rate limit")
	})

	t.Run("Should apply global rate limit to unauthenticated requests", func(t *testing.T) {
		// Make requests without authentication (should use global limit)
		// Global limit is 100, so all should succeed
		for i := 0; i < 10; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Should handle auth failures before rate limiting", func(t *testing.T) {
		// Make request with invalid API key
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer invalid_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should get 401 Unauthorized, not rate limit error
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Authentication failed")

		// Rate limit headers should not be present
		assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))
	})
}
