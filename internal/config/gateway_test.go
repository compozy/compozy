package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGatewayConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should return secure gateway defaults and private home paths [UT-001]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := defaultGatewayConfig(homePaths)
		if cfg.Enabled || cfg.PrivatePort != 0 || cfg.PublicPort != 0 {
			t.Fatalf("default gateway ceiling/ports = %#v", cfg)
		}
		if cfg.Pairing.TTL != 5*time.Minute || cfg.Pairing.MaxPending != 8 ||
			cfg.StreamTicket.TTL != 30*time.Second || cfg.Auth.RateLimit.Window != 60*time.Second ||
			cfg.Auth.RateLimit.MaxFails != 10 || cfg.Verify.Timeout != 10*time.Second {
			t.Fatalf("default gateway tunables = %#v", cfg)
		}
		wantCredentials := filepath.Join(homePaths.HomeDir, GatewayDirName, GatewayCredentialsDirName)
		if cfg.CredentialsDir != wantCredentials || homePaths.GatewayCredentialsDir != wantCredentials {
			t.Fatalf(
				"credential paths = %q/%q, want %q",
				cfg.CredentialsDir,
				homePaths.GatewayCredentialsDir,
				wantCredentials,
			)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		info, err := os.Stat(wantCredentials)
		if err != nil {
			t.Fatalf("os.Stat(credentials) error = %v", err)
		}
		if got := info.Mode().Perm(); got != privateDirMode {
			t.Fatalf("credentials mode = %#o, want %#o", got, privateDirMode)
		}
	})

	t.Run("Should return a path-aware error for an invalid private port [UT-002]", func(t *testing.T) {
		t.Parallel()
		cfg := defaultGatewayConfig(HomePaths{})
		cfg.PrivatePort = 70000
		assertGatewayValidationPath(t, cfg.Validate(), "gateway.private_port")
	})

	t.Run("Should return a path-aware error for a negative pairing bound [UT-003]", func(t *testing.T) {
		t.Parallel()
		cfg := defaultGatewayConfig(HomePaths{})
		cfg.Pairing.MaxPending = -1
		assertGatewayValidationPath(t, cfg.Validate(), "gateway.pairing.max_pending")
	})

	t.Run("Should reject every config key that attempts to enable a surface [UT-004]", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		path := filepath.Join(t.TempDir(), ConfigName)
		if err := os.WriteFile(path, []byte("[gateway.public_ui]\nenabled = true\n"), privateFileMode); err != nil {
			t.Fatalf("os.WriteFile() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		err = ApplyConfigOverlayFile(path, &cfg)
		if err == nil {
			t.Fatal("ApplyConfigOverlayFile() error = nil, want unknown-key rejection")
		}
	})

	t.Run("Should preserve omitted overlay fields and overwrite present fields [UT-005]", func(t *testing.T) {
		t.Parallel()
		cfg := defaultGatewayConfig(HomePaths{GatewayCredentialsDir: "/gateway/credentials"})
		original := cfg
		gatewayOverlay{}.Apply(&cfg)
		if !reflect.DeepEqual(cfg, original) {
			t.Fatalf("nil overlay changed config: got %#v want %#v", cfg, original)
		}
		enabled := true
		privatePort := 4242
		pairingTTL := 2 * time.Minute
		gatewayOverlay{
			Enabled: &enabled, PrivatePort: &privatePort,
			Pairing: gatewayPairingOverlay{TTL: &pairingTTL},
		}.Apply(&cfg)
		if !cfg.Enabled || cfg.PrivatePort != privatePort || cfg.Pairing.TTL != pairingTTL {
			t.Fatalf("set overlay config = %#v", cfg)
		}
		if cfg.PublicPort != original.PublicPort || cfg.CredentialsDir != original.CredentialsDir {
			t.Fatalf("set overlay changed omitted fields: %#v", cfg)
		}
	})

	t.Run("Should validate active non-secret connection metadata", func(t *testing.T) {
		t.Parallel()
		cfg := defaultGatewayConfig(HomePaths{})
		cfg.Connections = []GatewayConnectionConfig{{
			Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
			CredentialFile: "laptop.cred",
		}}
		cfg.ActiveConnection = "laptop"
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		cfg.Connections = append(cfg.Connections, cfg.Connections[0])
		assertGatewayValidationPath(t, cfg.Validate(), "gateway.connections")
	})

	t.Run("Should reject an active profile that is absent", func(t *testing.T) {
		t.Parallel()
		cfg := defaultGatewayConfig(HomePaths{})
		cfg.ActiveConnection = "missing"
		assertGatewayValidationPath(t, cfg.Validate(), "gateway.active_connection")
	})

	t.Run("Should reject gateway settings in workspace overlays", func(t *testing.T) {
		t.Parallel()
		for _, content := range []string{
			"[gateway]\nactive_connection = \"remote\"\n",
			"[[gateway.connections]]\nname = \"remote\"\nscheme = \"https\"\nhost = \"shadow.example\"\nport = 443\ncredential_file = \"remote.cred\"\n",
		} {
			overlay, err := loadConfigOverlayBytes([]byte(content), "workspace.toml")
			if err != nil {
				t.Fatalf("loadConfigOverlayBytes() error = %v", err)
			}
			err = validateWorkspaceConfigOverlay("workspace.toml", &overlay)
			if err == nil || !strings.Contains(err.Error(), "gateway settings are global-only") {
				t.Fatalf("validateWorkspaceConfigOverlay() error = %v, want global-only rejection", err)
			}
		}
	})

	t.Run("Should require canonical schemes and scheme-specific profile fields", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name       string
			connection GatewayConnectionConfig
		}{
			{
				name: "uppercase scheme",
				connection: GatewayConnectionConfig{
					Name: "remote", Scheme: "SSH", Host: "remote.example", Port: 22,
				},
			},
			{
				name: "HTTPS remote home",
				connection: GatewayConnectionConfig{
					Name: "remote", Scheme: "https", Host: "remote.example", Port: 443,
					CredentialFile: "remote.cred", RemoteHome: "/srv/compozy",
				},
			},
			{
				name: "SSH default workspace",
				connection: GatewayConnectionConfig{
					Name: "remote", Scheme: "ssh", Host: "remote.example", Port: 22,
					DefaultWorkspace: "workspace-1",
				},
			},
		}
		for _, testCase := range testCases {
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
				t.Parallel()
				if err := testCase.connection.Validate(); err == nil {
					t.Fatal("Validate() error = nil, want incompatible profile rejection")
				}
			})
		}
	})
}

func assertGatewayValidationPath(t *testing.T, err error, wantPath string) {
	t.Helper()
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("Validate() error = %v, want ValidationError", err)
	}
	if validationErr.Path != wantPath {
		t.Fatalf("ValidationError.Path = %q, want %q", validationErr.Path, wantPath)
	}
}
