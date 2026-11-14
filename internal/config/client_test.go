package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaultClientConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultClientConfig()
	if cfg.RequestTimeout != DefaultClientRequestTimeout {
		t.Fatalf("request timeout = %v, want %v", cfg.RequestTimeout, DefaultClientRequestTimeout)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestClientConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout time.Duration
		wantErr bool
	}{
		{name: "Should accept the floor", timeout: MinClientRequestTimeout},
		{name: "Should accept the ceiling", timeout: MaxClientRequestTimeout},
		{name: "Should reject a value below the floor", timeout: time.Second, wantErr: true},
		{name: "Should reject a value above the ceiling", timeout: time.Hour, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ClientConfig{RequestTimeout: tt.timeout}.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "client.request_timeout") {
				t.Fatalf("Validate() error = %v, want the client.request_timeout path", err)
			}
		})
	}
}

func TestApplyConfigOverlayFileAppliesClientRequestTimeout(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := DefaultWithHome(homePaths)
	overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
	writeFile(t, overlayPath, "[client]\nrequest_timeout = \"2m\"\n")
	if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
		t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
	}
	if want := 2 * time.Minute; cfg.Client.RequestTimeout != want {
		t.Fatalf("request timeout = %v, want %v", cfg.Client.RequestTimeout, want)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
