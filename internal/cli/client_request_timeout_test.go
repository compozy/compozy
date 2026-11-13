package cli

import (
	"errors"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestResolveClientRequestTimeout(t *testing.T) {
	t.Parallel()

	staticConfig := func(timeout time.Duration) func() (compozyconfig.Config, error) {
		return func() (compozyconfig.Config, error) {
			return compozyconfig.Config{Client: compozyconfig.ClientConfig{RequestTimeout: timeout}}, nil
		}
	}
	failingConfig := func() (compozyconfig.Config, error) {
		return compozyconfig.Config{}, errors.New("config unavailable")
	}
	env := func(value string) func(string) string {
		return func(string) string { return value }
	}

	tests := []struct {
		name        string
		getenv      func(string) string
		loadConfig  func() (compozyconfig.Config, error)
		want        time.Duration
		wantErr     bool
		wantClamped time.Duration
	}{
		{
			name:       "Should fall back to the default without config or env",
			getenv:     env(""),
			loadConfig: staticConfig(0),
			want:       defaultUnixSocketClientTimeout,
		},
		{
			name:       "Should read the config key",
			getenv:     env(""),
			loadConfig: staticConfig(2 * time.Minute),
			want:       2 * time.Minute,
		},
		{
			name:       "Should let the environment override the config key",
			getenv:     env("4m"),
			loadConfig: staticConfig(2 * time.Minute),
			want:       4 * time.Minute,
		},
		{
			name:       "Should ignore a config load failure and keep the default",
			getenv:     env(""),
			loadConfig: failingConfig,
			want:       defaultUnixSocketClientTimeout,
		},
		{
			name:   "Should clamp a config value below the floor",
			getenv: env(""),
			loadConfig: staticConfig(
				time.Second,
			),
			want: compozyconfig.MinClientRequestTimeout,
		},
		{
			name:       "Should clamp a config value above the ceiling",
			getenv:     env(""),
			loadConfig: staticConfig(2 * time.Hour),
			want:       compozyconfig.MaxClientRequestTimeout,
		},
		{
			name:       "Should reject an unparsable environment value",
			getenv:     env("soon"),
			loadConfig: staticConfig(0),
			wantErr:    true,
		},
		{
			name:       "Should reject a non-positive environment value",
			getenv:     env("-1m"),
			loadConfig: staticConfig(0),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveClientRequestTimeout(tt.getenv, tt.loadConfig)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resolveClientRequestTimeout() error = nil, want a parse rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveClientRequestTimeout() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("timeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandDepsApplyResolvedRequestTimeout(t *testing.T) {
	t.Parallel()

	deps := commandDeps{
		getenv: func(string) string { return "3m" },
		loadConfig: func() (compozyconfig.Config, error) {
			return compozyconfig.Config{}, nil
		},
	}

	client, err := deps.withDefaults().newClient(LocalClientTarget("/tmp/compozy.sock"))
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	udsClient, ok := client.(*daemonClient)
	if !ok {
		t.Fatalf("newClient() = %T, want *daemonClient", client)
	}
	if want := 3 * time.Minute; udsClient.httpClient.Timeout != want {
		t.Fatalf("httpClient timeout = %v, want %v", udsClient.httpClient.Timeout, want)
	}
	if udsClient.streamClient.Timeout != 0 {
		t.Fatalf("streamClient timeout = %v, want streams left untimed", udsClient.streamClient.Timeout)
	}
}
