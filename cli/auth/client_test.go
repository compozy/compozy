package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/cli/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Run("Should create client with valid config", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 8080,
			},
		}
		client, err := NewClient(cfg, "test-key")
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:8080/api/v0", client.baseURL)
		assert.Equal(t, "test-key", client.apiKey)
		assert.NotNil(t, client.httpClient)
	})

	t.Run("Should create client without API key", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: "localhost",
				Port: 3000,
			},
		}
		client, err := NewClient(cfg, "")
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "http://localhost:3000/api/v0", client.baseURL)
		assert.Equal(t, "", client.apiKey)
	})
}

func TestClientRetryLogic(t *testing.T) {
	t.Run("Should retry on server errors", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "success"})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL,
			apiKey:     "test-key",
		}

		ctx := context.Background()
		resp, err := client.doRequest(ctx, http.MethodGet, "/test", nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		defer resp.Body.Close()
		assert.Equal(t, 3, attempts)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Should not retry on client errors", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts++
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL,
			apiKey:     "test-key",
		}

		ctx := context.Background()
		resp, err := client.doRequest(ctx, http.MethodGet, "/test", nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		defer resp.Body.Close()
		assert.Equal(t, 1, attempts) // No retry on 4xx
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("Should respect context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// Always return server error to trigger retry
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL,
			apiKey:     "test-key",
		}

		ctx, cancel := context.WithCancel(context.Background())
		// Cancel immediately
		cancel()

		resp, err := client.doRequest(ctx, http.MethodGet, "/test", nil)
		if resp != nil {
			resp.Body.Close()
		}
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})
}

func TestGenerateKey(t *testing.T) {
	t.Run("Should generate key successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/auth/generate", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]string{
					"api_key": "cmp_1234567890",
				},
				"message": "API key generated successfully",
			})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		apiKey, err := client.GenerateKey(ctx, &GenerateKeyRequest{})
		require.NoError(t, err)
		assert.Equal(t, "cmp_1234567890", apiKey)
	})

	t.Run("Should handle error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(models.APIResponse{
				Error: "Unauthorized",
			})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "invalid-key",
		}

		ctx := context.Background()
		_, err := client.GenerateKey(ctx, &GenerateKeyRequest{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error: Unauthorized")
	})
}

func TestListKeys(t *testing.T) {
	t.Run("Should list keys successfully", func(t *testing.T) {
		lastUsed := "2024-01-03T00:00:00Z"
		expectedKeys := []KeyInfo{
			{
				ID:        "key1",
				Prefix:    "cmp_123",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			{
				ID:        "key2",
				Prefix:    "cmp_456",
				CreatedAt: "2024-01-02T00:00:00Z",
				LastUsed:  &lastUsed,
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/auth/keys", r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"data": map[string][]KeyInfo{
					"keys": expectedKeys,
				},
				"message": "Success",
			})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		keys, err := client.ListKeys(ctx)
		require.NoError(t, err)
		assert.Len(t, keys, 2)
		assert.Equal(t, expectedKeys, keys)
	})
}

func TestRevokeKey(t *testing.T) {
	t.Run("Should revoke key successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/auth/keys/key123", r.URL.Path)
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		err := client.RevokeKey(ctx, "key123")
		assert.NoError(t, err)
	})
}

func TestCreateUser(t *testing.T) {
	t.Run("Should create user successfully", func(t *testing.T) {
		expectedUser := &UserInfo{
			ID:        "user123",
			Email:     "test@example.com",
			Name:      "Test User",
			Role:      "user",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-01T00:00:00Z",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/users", r.URL.Path)
			assert.Equal(t, http.MethodPost, r.Method)

			var req CreateUserRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, "test@example.com", req.Email)
			assert.Equal(t, "Test User", req.Name)
			assert.Equal(t, "user", req.Role)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"data":    expectedUser,
				"message": "User created successfully",
			})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		user, err := client.CreateUser(ctx, CreateUserRequest{
			Email: "test@example.com",
			Name:  "Test User",
			Role:  "user",
		})
		require.NoError(t, err)
		assert.Equal(t, expectedUser, user)
	})
}

func TestUpdateUser(t *testing.T) {
	t.Run("Should update user successfully", func(t *testing.T) {
		newName := "Updated Name"
		expectedUser := &UserInfo{
			ID:        "user123",
			Email:     "test@example.com",
			Name:      newName,
			Role:      "user",
			CreatedAt: "2024-01-01T00:00:00Z",
			UpdatedAt: "2024-01-02T00:00:00Z",
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/users/user123", r.URL.Path)
			assert.Equal(t, http.MethodPatch, r.Method)

			var req UpdateUserRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)
			assert.Equal(t, &newName, req.Name)
			assert.Nil(t, req.Email)
			assert.Nil(t, req.Role)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"data":    expectedUser,
				"message": "User updated successfully",
			})
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		user, err := client.UpdateUser(ctx, "user123", UpdateUserRequest{
			Name: &newName,
		})
		require.NoError(t, err)
		assert.Equal(t, expectedUser, user)
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("Should delete user successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/users/user123", r.URL.Path)
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &Client{
			httpClient: &http.Client{Timeout: 5 * time.Second},
			baseURL:    server.URL + "/api/v0",
			apiKey:     "test-key",
		}

		ctx := context.Background()
		err := client.DeleteUser(ctx, "user123")
		assert.NoError(t, err)
	})
}

func TestParseResponse(t *testing.T) {
	t.Run("Should parse error response with API error", func(t *testing.T) {
		client := &Client{}
		body := `{"error": "Invalid request"}`
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		err := client.parseResponse(resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API error: Invalid request")
	})

	t.Run("Should parse error response without API error", func(t *testing.T) {
		client := &Client{}
		body := `not json`
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		err := client.parseResponse(resp, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server returned 500: not json")
	})

	t.Run("Should parse successful response", func(t *testing.T) {
		client := &Client{}
		body := `{"id": "123", "name": "test"}`
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
		}

		var result struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		err := client.parseResponse(resp, &result)
		assert.NoError(t, err)
		assert.Equal(t, "123", result.ID)
		assert.Equal(t, "test", result.Name)
	})
}
