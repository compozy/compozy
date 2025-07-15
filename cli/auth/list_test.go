package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/compozy/compozy/cli/auth/tui/models"
	"github.com/compozy/compozy/pkg/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCmd(t *testing.T) {
	t.Run("Should create list command with correct flags", func(t *testing.T) {
		cmd := ListCmd()
		assert.Equal(t, "list", cmd.Use)
		assert.Equal(t, "List API keys", cmd.Short)
		// Check flags
		assert.NotNil(t, cmd.Flags().Lookup("sort"))
		assert.NotNil(t, cmd.Flags().Lookup("filter"))
		assert.NotNil(t, cmd.Flags().Lookup("page"))
		assert.NotNil(t, cmd.Flags().Lookup("limit"))
		assert.NotNil(t, cmd.Flags().Lookup("json"))
		assert.NotNil(t, cmd.Flags().Lookup("tui"))
	})
}

func TestRunListJSON(t *testing.T) {
	t.Run("Should output JSON with keys and pagination metadata", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/v0/auth/keys", r.URL.Path)
			assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
			// Return test keys in structured format
			response := map[string]any{
				"data": map[string][]models.KeyInfo{
					"keys": {
						{
							ID:        "key1",
							Prefix:    "cpzy_abc",
							CreatedAt: "2024-01-01T00:00:00Z",
						},
						{
							ID:        "key2",
							Prefix:    "cpzy_def",
							CreatedAt: "2024-01-02T00:00:00Z",
						},
					},
				},
				"message": "Success",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		// Create test config
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: server.URL[7:], // Remove http://
				Port: 80,
			},
		}
		// Create client
		client, err := NewClient(cfg, "test-key")
		require.NoError(t, err)
		client.baseURL = server.URL + "/api/v0"
		// Create command
		cmd := &cobra.Command{}
		cmd.Flags().String("sort", "created", "")
		cmd.Flags().String("filter", "", "")
		cmd.Flags().Int("page", 1, "")
		cmd.Flags().Int("limit", 50, "")
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		// Run command
		err = runListJSON(context.Background(), cmd, client)
		require.NoError(t, err)
		// Restore stdout and read output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)
		// Parse output
		var output map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)
		// Verify output
		assert.Equal(t, float64(2), output["total"])
		assert.Equal(t, float64(1), output["page"])
		assert.Equal(t, float64(50), output["limit"])
		assert.Equal(t, float64(1), output["pages"])
		keys := output["keys"].([]any)
		assert.Len(t, keys, 2)
	})
	t.Run("Should apply filtering correctly", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			response := map[string]any{
				"data": map[string][]models.KeyInfo{
					"keys": {
						{
							ID:        "key1",
							Prefix:    "cpzy_abc123",
							CreatedAt: "2024-01-01T00:00:00Z",
						},
						{
							ID:        "key2",
							Prefix:    "cpzy_def456",
							CreatedAt: "2024-01-02T00:00:00Z",
						},
						{
							ID:        "key3",
							Prefix:    "cpzy_abc789",
							CreatedAt: "2024-01-03T00:00:00Z",
						},
					},
				},
				"message": "Success",
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()
		// Create test config
		cfg := &config.Config{
			Server: config.ServerConfig{
				Host: server.URL[7:],
				Port: 80,
			},
		}
		// Create client
		client, err := NewClient(cfg, "test-key")
		require.NoError(t, err)
		client.baseURL = server.URL + "/api/v0"
		// Create command with filter
		cmd := &cobra.Command{}
		cmd.Flags().String("sort", "created", "")
		cmd.Flags().String("filter", "abc", "")
		cmd.Flags().Int("page", 1, "")
		cmd.Flags().Int("limit", 50, "")
		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		// Run command
		err = runListJSON(context.Background(), cmd, client)
		require.NoError(t, err)
		// Restore stdout and read output
		w.Close()
		os.Stdout = oldStdout
		var buf bytes.Buffer
		buf.ReadFrom(r)
		// Parse output
		var output map[string]any
		err = json.Unmarshal(buf.Bytes(), &output)
		require.NoError(t, err)
		// Verify filtering worked
		assert.Equal(t, float64(2), output["total"]) // Only 2 keys match "abc"
		keys := output["keys"].([]any)
		assert.Len(t, keys, 2)
	})
}

func TestSortKeys(t *testing.T) {
	t.Run("Should sort by created date", func(t *testing.T) {
		keys := []models.KeyInfo{
			{ID: "1", CreatedAt: "2024-01-01"},
			{ID: "2", CreatedAt: "2024-01-03"},
			{ID: "3", CreatedAt: "2024-01-02"},
		}
		sortKeys(keys, "created")
		assert.Equal(t, "2", keys[0].ID)
		assert.Equal(t, "3", keys[1].ID)
		assert.Equal(t, "1", keys[2].ID)
	})
	t.Run("Should sort by prefix/name", func(t *testing.T) {
		keys := []models.KeyInfo{
			{ID: "1", Prefix: "cpzy_ccc"},
			{ID: "2", Prefix: "cpzy_aaa"},
			{ID: "3", Prefix: "cpzy_bbb"},
		}
		sortKeys(keys, "name")
		assert.Equal(t, "2", keys[0].ID)
		assert.Equal(t, "3", keys[1].ID)
		assert.Equal(t, "1", keys[2].ID)
	})
	t.Run("Should sort by last used with nil handling", func(t *testing.T) {
		time1 := "2024-01-01"
		time2 := "2024-01-02"
		keys := []models.KeyInfo{
			{ID: "1", LastUsed: &time1},
			{ID: "2", LastUsed: nil},
			{ID: "3", LastUsed: &time2},
		}
		sortKeys(keys, "last_used")
		assert.Equal(t, "3", keys[0].ID) // Most recent
		assert.Equal(t, "1", keys[1].ID)
		assert.Equal(t, "2", keys[2].ID) // nil last
	})
}
