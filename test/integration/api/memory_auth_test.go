package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/auth/model"
	authuc "github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	authmw "github.com/compozy/compozy/engine/infra/server/middleware/auth"
	memrouter "github.com/compozy/compozy/engine/memory/router"
	"github.com/stretchr/testify/mock"
)

// MockAuthRepo is a mock for auth repository
type MockAuthRepo struct {
	mock.Mock
}

func (m *MockAuthRepo) GetAPIKeyByHash(ctx context.Context, hash []byte) (*model.APIKey, error) {
	args := m.Called(ctx, hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockAuthRepo) GetUserByID(ctx context.Context, userID core.ID) (*model.User, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthRepo) UpdateAPIKeyLastUsed(ctx context.Context, keyID core.ID) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

func (m *MockAuthRepo) CreateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockAuthRepo) CreateAPIKey(ctx context.Context, apiKey *model.APIKey) error {
	args := m.Called(ctx, apiKey)
	return args.Error(0)
}

func (m *MockAuthRepo) ListAPIKeys(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.APIKey), args.Error(1)
}

func (m *MockAuthRepo) ListAPIKeysByUserID(ctx context.Context, userID core.ID) ([]*model.APIKey, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.APIKey), args.Error(1)
}

func (m *MockAuthRepo) GetAPIKeyByID(ctx context.Context, keyID core.ID) (*model.APIKey, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.APIKey), args.Error(1)
}

func (m *MockAuthRepo) DeleteAPIKey(ctx context.Context, keyID core.ID) error {
	args := m.Called(ctx, keyID)
	return args.Error(0)
}

func (m *MockAuthRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockAuthRepo) ListUsers(ctx context.Context) ([]*model.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.User), args.Error(1)
}

func (m *MockAuthRepo) UpdateUser(ctx context.Context, user *model.User) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockAuthRepo) DeleteUser(ctx context.Context, userID core.ID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func TestMemoryAuthenticationRequired(t *testing.T) {
	// Setup
	mockRepo := new(MockAuthRepo)
	authFactory := authuc.NewFactory(mockRepo)

	// Setup router with auth
	router := setupAuthenticatedRouter(authFactory)

	// Test cases
	testCases := []struct {
		name           string
		method         string
		path           string
		body           any
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "GET /memory/:ref/read without auth",
			method:         "GET",
			path:           "/api/v0/memory/test_memory/read?key=test_key",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:   "POST /memory/:ref/write without auth",
			method: "POST",
			path:   "/api/v0/memory/test_memory/write",
			body: map[string]any{
				"key":      "test_key",
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:   "POST /memory/:ref/append without auth",
			method: "POST",
			path:   "/api/v0/memory/test_memory/append",
			body: map[string]any{
				"key":      "test_key",
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "POST /memory/:ref/delete without auth",
			method:         "POST",
			path:           "/api/v0/memory/test_memory/delete",
			body:           map[string]any{"key": "test_key"},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "GET /memory/:ref/health without auth",
			method:         "GET",
			path:           "/api/v0/memory/test_memory/health?key=test_key",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
		{
			name:           "GET /memory/:ref/stats without auth",
			method:         "GET",
			path:           "/api/v0/memory/test_memory/stats?key=test_key",
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authentication required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var req *http.Request
			if tc.body != nil {
				bodyBytes, err := json.Marshal(tc.body)
				require.NoError(t, err)
				req = httptest.NewRequest(tc.method, tc.path, bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, http.NoBody)
			}

			// Don't set Authorization header - we want to test unauthenticated access
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedStatus, rec.Code, "Expected status %d but got %d", tc.expectedStatus, rec.Code)

			if tc.expectedError != "" {
				var response map[string]any
				err := json.Unmarshal(rec.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["error"], tc.expectedError)
			}
		})
	}
}

func setupAuthenticatedRouter(authFactory *authuc.Factory) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(gin.Recovery())

	// Setup auth middleware
	authManager := authmw.NewManager(authFactory)
	router.Use(authManager.Middleware())

	// Register memory routes with auth
	apiGroup := router.Group("/api/v0")
	memrouter.Register(apiGroup, authFactory)

	return router
}
