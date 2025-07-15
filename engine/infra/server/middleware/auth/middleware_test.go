package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
func (m *MockRepository) CreateUser(_ context.Context, _ *model.User) error {
	return errors.New("not implemented")
}
func (m *MockRepository) GetUserByEmail(_ context.Context, _ string) (*model.User, error) {
	return nil, errors.New("not implemented")
}
func (m *MockRepository) ListUsers(_ context.Context) ([]*model.User, error) {
	return nil, errors.New("not implemented")
}
func (m *MockRepository) UpdateUser(_ context.Context, _ *model.User) error {
	return errors.New("not implemented")
}
func (m *MockRepository) DeleteUser(_ context.Context, _ core.ID) error {
	return errors.New("not implemented")
}
func (m *MockRepository) CreateAPIKey(_ context.Context, _ *model.APIKey) error {
	return errors.New("not implemented")
}
func (m *MockRepository) GetAPIKeyByID(_ context.Context, _ core.ID) (*model.APIKey, error) {
	return nil, errors.New("not implemented")
}
func (m *MockRepository) ListAPIKeysByUserID(_ context.Context, _ core.ID) ([]*model.APIKey, error) {
	return nil, errors.New("not implemented")
}
func (m *MockRepository) DeleteAPIKey(_ context.Context, _ core.ID) error {
	return errors.New("not implemented")
}

func TestManager_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should allow request without authorization header", func(t *testing.T) {
		mockRepo := &MockRepository{}
		factory := uc.NewFactory(mockRepo)
		manager := NewManager(factory)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"message":"success"}`, w.Body.String())
	})

	t.Run("Should reject request with invalid authorization header format", func(t *testing.T) {
		mockRepo := &MockRepository{}
		factory := uc.NewFactory(mockRepo)
		manager := NewManager(factory)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "InvalidFormat")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Authentication failed", "details":"Invalid authorization header format"}`,
			w.Body.String(),
		)
	})

	t.Run("Should reject request with empty API key", func(t *testing.T) {
		mockRepo := &MockRepository{}
		factory := uc.NewFactory(mockRepo)
		manager := NewManager(factory)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Authentication failed", "details":"Invalid authorization header format"}`,
			w.Body.String(),
		)
	})

	t.Run("Should reject request with invalid API key", func(t *testing.T) {
		mockRepo := &MockRepository{}
		mockRepo.On("GetAPIKeyByHash", mock.Anything, mock.Anything).
			Return((*model.APIKey)(nil), errors.New("key not found"))
		factory := uc.NewFactory(mockRepo)
		manager := NewManager(factory)
		router := gin.New()
		router.Use(manager.Middleware())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Authorization", "Bearer invalid_key")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Authentication failed", "details":"Invalid or missing credentials"}`,
			w.Body.String(),
		)
	})
}

func TestManager_RequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should reject request without authentication", func(t *testing.T) {
		manager := NewManager(nil)
		router := gin.New()
		router.Use(manager.RequireAuth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Authentication required", "details":"This endpoint requires a valid API key"}`,
			w.Body.String(),
		)
	})

	t.Run("Should allow request with authentication", func(t *testing.T) {
		manager := NewManager(nil)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			// Create a test user and inject into request context
			userID, _ := core.NewID()
			user := &model.User{
				ID:   userID,
				Role: model.RoleUser,
			}
			ctx := userctx.WithUser(c.Request.Context(), user)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
		router.Use(manager.RequireAuth())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"message":"success"}`, w.Body.String())
	})
}

func TestManager_RequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should reject request without admin role", func(t *testing.T) {
		manager := NewManager(nil)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("userRole", "user")
			c.Next()
		})
		router.Use(manager.RequireAdmin())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Admin access required", "details":"This endpoint requires admin privileges"}`,
			w.Body.String(),
		)
	})

	t.Run("Should allow request with admin role", func(t *testing.T) {
		manager := NewManager(nil)
		router := gin.New()
		router.Use(func(c *gin.Context) {
			// Create a test admin user and inject into request context
			userID, _ := core.NewID()
			user := &model.User{
				ID:   userID,
				Role: model.RoleAdmin,
			}
			ctx := userctx.WithUser(c.Request.Context(), user)
			c.Request = c.Request.WithContext(ctx)
			c.Next()
		})
		router.Use(manager.RequireAdmin())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"message":"success"}`, w.Body.String())
	})

	t.Run("Should reject request without role", func(t *testing.T) {
		manager := NewManager(nil)
		router := gin.New()
		router.Use(manager.RequireAdmin())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Admin access required", "details":"This endpoint requires admin privileges"}`,
			w.Body.String(),
		)
	})
}

func TestManager_AdminOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Should be an alias for RequireAdmin", func(t *testing.T) {
		manager := NewManager(nil)
		// AdminOnly should return the same function as RequireAdmin
		router := gin.New()
		router.Use(func(c *gin.Context) {
			c.Set("userRole", "user")
			c.Next()
		})
		router.Use(manager.AdminOnly())
		router.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(
			t,
			`{"error":"Admin access required", "details":"This endpoint requires admin privileges"}`,
			w.Body.String(),
		)
	})
}
