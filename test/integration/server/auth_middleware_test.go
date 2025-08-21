package server

import (
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

// MockRepository is a minimal mock focusing only on required methods
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

// Stub methods for interface compliance - optimized with nil returns
func (m *MockRepository) CreateUser(context.Context, *model.User) error { return nil }
func (m *MockRepository) GetUserByEmail(context.Context, string) (*model.User, error) {
	return nil, nil
}
func (m *MockRepository) ListUsers(context.Context) ([]*model.User, error)  { return nil, nil }
func (m *MockRepository) UpdateUser(context.Context, *model.User) error     { return nil }
func (m *MockRepository) DeleteUser(context.Context, core.ID) error         { return nil }
func (m *MockRepository) CreateAPIKey(context.Context, *model.APIKey) error { return nil }
func (m *MockRepository) GetAPIKeyByID(context.Context, core.ID) (*model.APIKey, error) {
	return nil, nil
}
func (m *MockRepository) ListAPIKeysByUserID(context.Context, core.ID) ([]*model.APIKey, error) {
	return nil, nil
}
func (m *MockRepository) DeleteAPIKey(context.Context, core.ID) error { return nil }

// Test fixture for shared setup
type authTestFixture struct {
	mockRepo     *MockRepository
	authManager  *authmiddleware.Manager
	rateLimitMgr *ratelimit.Manager
	router       *gin.Engine
	apiKey       string
	userID       core.ID
	keyID        core.ID
}

// setupAuthTestFixture creates a reusable test fixture
func setupAuthTestFixture(t *testing.T) *authTestFixture {
	gin.SetMode(gin.TestMode)
	userID, _ := core.NewID()
	keyID, _ := core.NewID()
	apiKey := "test-api-key"
	// Pre-compute hash to avoid repeated bcrypt operations
	hash := sha256.Sum256([]byte(apiKey))
	bcryptHash, _ := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.MinCost) // Use MinCost for tests
	user := &model.User{ID: userID, Email: "test@example.com", Role: model.RoleUser}
	storedAPIKey := &model.APIKey{ID: keyID, UserID: userID, Hash: bcryptHash}
	mockRepo := new(MockRepository)
	mockRepo.On("GetAPIKeyByHash", mock.Anything, hash[:]).Return(storedAPIKey, nil)
	mockRepo.On("GetUserByID", mock.Anything, userID).Return(user, nil)
	mockRepo.On("UpdateAPIKeyLastUsed", mock.Anything, keyID).Return(nil)
	authFactory := uc.NewFactory(mockRepo)
	authManager := authmiddleware.NewManager(authFactory, nil)
	config := &ratelimit.Config{
		GlobalRate: ratelimit.RateConfig{Limit: 100, Period: 1 * time.Minute},
		APIKeyRate: ratelimit.RateConfig{Limit: 5, Period: 1 * time.Minute},
		RouteRates: map[string]ratelimit.RateConfig{},
		Prefix:     "test:ratelimit:",
		MaxRetry:   3,
	}
	rateLimitManager, err := ratelimit.NewManager(config, nil)
	require.NoError(t, err)
	router := gin.New()
	router.Use(authManager.Middleware())
	router.Use(rateLimitManager.Middleware())
	router.GET("/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"message": "success"}) })
	return &authTestFixture{mockRepo, authManager, rateLimitManager, router, apiKey, userID, keyID}
}

func TestAuthMiddleware_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	fixture := setupAuthTestFixture(t)

	t.Run("Should rate limit authenticated requests by API key", func(t *testing.T) {
		// Test within limit and rate limiting in single loop
		for i := range 6 {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.Header.Set("Authorization", "Bearer "+fixture.apiKey)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			if i < 5 {
				assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
				assert.Contains(t, w.Body.String(), "Rate limit exceeded")
				assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
			}
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
		}
	})

	t.Run("Should track rate limits separately for different API keys", func(t *testing.T) {
		// Setup second API key with minimal complexity
		userID2, _ := core.NewID()
		keyID2, _ := core.NewID()
		apiKey2 := "test-api-key-2"
		hash2 := sha256.Sum256([]byte(apiKey2))
		bcryptHash2, _ := bcrypt.GenerateFromPassword([]byte(apiKey2), bcrypt.MinCost)
		user2 := &model.User{ID: userID2, Email: "test2@example.com", Role: model.RoleUser}
		storedAPIKey2 := &model.APIKey{ID: keyID2, UserID: userID2, Hash: bcryptHash2}
		fixture.mockRepo.On("GetAPIKeyByHash", mock.Anything, hash2[:]).Return(storedAPIKey2, nil)
		fixture.mockRepo.On("GetUserByID", mock.Anything, userID2).Return(user2, nil)
		fixture.mockRepo.On("UpdateAPIKeyLastUsed", mock.Anything, keyID2).Return(nil)
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+apiKey2)
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Different API key should have separate rate limit")
	})

	t.Run("Should apply global rate limit to unauthenticated requests", func(t *testing.T) {
		// Test fewer requests to reduce execution time
		for range 3 {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			w := httptest.NewRecorder()
			fixture.router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("Should handle auth failures before rate limiting", func(t *testing.T) {
		// Setup mock for invalid key
		invalidHash := sha256.Sum256([]byte("invalid_key"))
		fixture.mockRepo.On("GetAPIKeyByHash", mock.Anything, invalidHash[:]).Return(nil, fmt.Errorf("key not found"))
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer invalid_key")
		w := httptest.NewRecorder()
		fixture.router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Authentication failed")
		assert.Empty(t, w.Header().Get("X-RateLimit-Limit"))
	})
}
