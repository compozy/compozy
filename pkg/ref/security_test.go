package ref

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// File Path Validation Tests
// -----------------------------------------------------------------------------

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		baseDir string
		wantErr bool
		errMsg  string
	}{
		// Valid relative paths
		{
			name:    "valid relative path",
			path:    "./config.yaml",
			baseDir: "/project",
			wantErr: false,
		},
		{
			name:    "valid relative path with parent dir",
			path:    "../shared/config.yaml",
			baseDir: "/project/sub",
			wantErr: false,
		},
		{
			name:    "valid absolute path",
			path:    "/absolute/path/config.yaml",
			baseDir: "/project",
			wantErr: false,
		},

		// Security violations - sensitive directories
		{
			name:    "blocked .git access",
			path:    "./.git/config",
			baseDir: "/project",
			wantErr: true,
			errMsg:  "sensitive directory",
		},
		{
			name:    "blocked .env access",
			path:    "./.env",
			baseDir: "/project",
			wantErr: true,
			errMsg:  "sensitive directory",
		},
		{
			name:    "blocked node_modules access",
			path:    "./node_modules/package/config.js",
			baseDir: "/project",
			wantErr: true,
			errMsg:  "sensitive directory",
		},

		// Edge cases
		{
			name:    "root path",
			path:    "/",
			baseDir: "/project",
			wantErr: true,
			errMsg:  "invalid absolute path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFilePath(tt.path, tt.baseDir)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errMsg))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// URL Validation Tests
// -----------------------------------------------------------------------------

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		// Valid URLs
		{
			name:    "valid HTTPS URL",
			url:     "https://api.example.com/config.yaml",
			wantErr: false,
		},
		{
			name:    "valid HTTP URL",
			url:     "http://api.example.com/config.yaml",
			wantErr: false,
		},
		{
			name:    "HTTPS with port",
			url:     "https://api.example.com:8443/config.yaml",
			wantErr: false,
		},

		// Invalid schemes
		{
			name:    "FTP scheme blocked",
			url:     "ftp://files.example.com/config.yaml",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "file scheme blocked",
			url:     "file:///etc/passwd",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "custom scheme blocked",
			url:     "custom://api.example.com/config",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},

		// Security violations - localhost/private IPs
		{
			name:    "localhost blocked",
			url:     "https://localhost:8080/config.yaml",
			wantErr: true,
			errMsg:  "local host",
		},
		{
			name:    "127.0.0.1 blocked",
			url:     "https://127.0.0.1/config.yaml",
			wantErr: true,
			errMsg:  "local host",
		},
		{
			name:    "0.0.0.0 blocked",
			url:     "https://0.0.0.0/config.yaml",
			wantErr: true,
			errMsg:  "local host",
		},
		{
			name:    "private IP 10.x blocked",
			url:     "https://10.0.0.1/config.yaml",
			wantErr: true,
			errMsg:  "private network",
		},
		{
			name:    "private IP 192.168.x blocked",
			url:     "https://192.168.1.1/config.yaml",
			wantErr: true,
			errMsg:  "private network",
		},
		{
			name:    "private IP 172.x blocked",
			url:     "https://172.16.0.1/config.yaml",
			wantErr: true,
			errMsg:  "private network",
		},

		// Malformed URLs
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "malformed URL",
			url:     "not-a-url",
			wantErr: true,
			errMsg:  "unsupported URL scheme",
		},
		{
			name:    "missing host",
			url:     "https:///config.yaml",
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errMsg))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Data Size Validation Tests
// -----------------------------------------------------------------------------

func TestValidateDataSize(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: false,
		},
		{
			name:    "small data",
			data:    []byte("small config"),
			wantErr: false,
		},
		{
			name:    "exactly at limit",
			data:    make([]byte, MaxResolvedDataSize),
			wantErr: false,
		},
		{
			name:    "exceeds limit by 1 byte",
			data:    make([]byte, MaxResolvedDataSize+1),
			wantErr: true,
		},
		{
			name:    "way too large",
			data:    make([]byte, MaxResolvedDataSize*2),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDataSize(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds maximum")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Max Depth Protection Tests
// -----------------------------------------------------------------------------

func TestMaxDepthProtection(t *testing.T) {
	t.Run("Should prevent infinite recursion with max depth", func(t *testing.T) {
		// Create a synthetic chain that exceeds max depth
		baseDir := t.TempDir()

		// Create chain of 25 files (more than DefaultMaxDepth = 20)
		for i := 0; i < 25; i++ {
			var content string
			if i == 24 {
				// Final file with actual content
				content = "final: value"
			} else {
				// Reference to next file
				content = "$ref: ./file" + fmt.Sprintf("%d", i+1) + ".yaml"
			}

			filename := fmt.Sprintf("file%d.yaml", i)
			err := os.WriteFile(filepath.Join(baseDir, filename), []byte(content), 0644)
			require.NoError(t, err)
		}

		// Try to resolve the chain starting from file0.yaml
		ref := &Ref{
			Type: TypeFile,
			File: "./file0.yaml",
			Path: "",
			Mode: ModeMerge,
		}

		ctx := context.Background()
		_, err := ref.Resolve(ctx, map[string]any{}, filepath.Join(baseDir, "start.yaml"), baseDir)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "max resolution depth exceeded")
	})
}
