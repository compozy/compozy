package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestDetectInstallMethods(t *testing.T) {
	t.Run("Should recognize managed install methods from path heuristics and package ownership", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			cfg  Config
			want string
		}{
			{
				name: "Should detect Homebrew installs from the executable path",
				cfg: Config{
					ExecutablePath: func() (string, error) {
						return "/opt/homebrew/Cellar/compozy/1.0.0/bin/compozy", nil
					},
				},
				want: string(InstallMethodHomebrew),
			},
			{
				name: "Should detect NPM installs from the executable path",
				cfg: Config{
					ExecutablePath: func() (string, error) {
						return "/usr/local/lib/node_modules/@compozy/cli/bin/compozy", nil
					},
				},
				want: string(InstallMethodNPM),
			},
			{
				name: "Should detect Windows NPM installs from the executable path",
				cfg: Config{
					ExecutablePath: func() (string, error) {
						return "C:\\Users\\pedro\\AppData\\Roaming\\npm\\node_modules\\@compozy\\cli\\bin\\compozy.exe", nil
					},
				},
				want: string(InstallMethodNPM),
			},
			{
				name: "Should detect Scoop installs from the executable path",
				cfg: Config{
					ExecutablePath: func() (string, error) {
						return "C:\\Users\\pedro\\scoop\\apps\\compozy\\current\\compozy.exe", nil
					},
				},
				want: string(InstallMethodScoop),
			},
			{
				name: "Should detect apt installs from dpkg ownership",
				cfg: Config{
					RuntimeOS: runtimeOSLinux,
					ExecutablePath: func() (string, error) {
						return "/usr/bin/compozy", nil
					},
					LookPath: func(name string) (string, error) {
						if name == "dpkg" {
							return "/usr/bin/dpkg", nil
						}
						return "", errors.New("not found")
					},
					RunCommand: func(_ context.Context, name string, _ ...string) (string, error) {
						if name == "dpkg" {
							return "compozy: /usr/bin/compozy", nil
						}
						return "", errors.New("unexpected command")
					},
				},
				want: string(InstallMethodAPT),
			},
			{
				name: "Should detect dnf installs from rpm ownership when dnf is present",
				cfg: Config{
					RuntimeOS: runtimeOSLinux,
					ExecutablePath: func() (string, error) {
						return "/usr/bin/compozy", nil
					},
					LookPath: func(name string) (string, error) {
						switch name {
						case "rpm":
							return "/usr/bin/rpm", nil
						case "dnf":
							return "/usr/bin/dnf", nil
						default:
							return "", errors.New("not found")
						}
					},
					RunCommand: func(_ context.Context, name string, _ ...string) (string, error) {
						if name == "rpm" {
							return "compozy", nil
						}
						return "", errors.New("unexpected command")
					},
				},
				want: string(InstallMethodDNF),
			},
			{
				name: "Should detect generic rpm installs when dnf is absent",
				cfg: Config{
					RuntimeOS: runtimeOSLinux,
					ExecutablePath: func() (string, error) {
						return "/usr/bin/compozy", nil
					},
					LookPath: func(name string) (string, error) {
						if name == "rpm" {
							return "/usr/bin/rpm", nil
						}
						return "", errors.New("not found")
					},
					RunCommand: func(_ context.Context, name string, _ ...string) (string, error) {
						if name == "rpm" {
							return "compozy", nil
						}
						return "", errors.New("unexpected command")
					},
				},
				want: string(InstallMethodRPM),
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				manager := testManager(t, &tc.cfg)
				info, err := manager.detectInstall(t.Context())
				if err != nil {
					t.Fatalf("detectInstall() error = %v", err)
				}
				if !info.Managed {
					t.Fatal("detectInstall() managed = false, want true")
				}
				if info.Method != tc.want {
					t.Fatalf("detectInstall() method = %q, want %q", info.Method, tc.want)
				}
			})
		}
	})
	t.Run("Should recognize desktop app provenance", runDesktopProvenanceCases)
}

func runDesktopProvenanceCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		marker      func(string) []byte
		wantMethod  InstallMethod
		wantManaged bool
	}{
		{
			name: "Should detect matching desktop app provenance",
			marker: func(digest string) []byte {
				return []byte(`{"installed_by":"desktop-app","binary_sha256":"` + digest + `"}`)
			},
			wantMethod:  InstallMethodDesktopApp,
			wantManaged: false,
		},
		{
			name: "Should fall through when desktop provenance hash mismatches",
			marker: func(string) []byte {
				return []byte(`{"installed_by":"desktop-app","binary_sha256":"deadbeef"}`)
			},
			wantMethod: InstallMethodDirectBinary,
		},
		{
			name: "Should fall through when desktop provenance is corrupt",
			marker: func(string) []byte {
				return []byte(`{"installed_by":`)
			},
			wantMethod: InstallMethodDirectBinary,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}
			binaryPath := filepath.Join(homePaths.HomeDir, "bin", compozyBinaryName)
			if err := os.MkdirAll(filepath.Dir(binaryPath), 0o700); err != nil {
				t.Fatalf("MkdirAll() error = %v", err)
			}
			binary := []byte("desktop-owned-compozy")
			if err := os.WriteFile(binaryPath, binary, 0o700); err != nil {
				t.Fatalf("WriteFile(binary) error = %v", err)
			}
			digest := sha256.Sum256(binary)
			markerPath := filepath.Join(filepath.Dir(binaryPath), desktopProvenanceFileName)
			if err := os.WriteFile(markerPath, tc.marker(hex.EncodeToString(digest[:])), 0o600); err != nil {
				t.Fatalf("WriteFile(marker) error = %v", err)
			}
			manager := testManager(t, &Config{
				HomePaths:      homePaths,
				RuntimeOS:      runtimeOSLinux,
				ExecutablePath: func() (string, error) { return binaryPath, nil },
			})
			info, err := manager.detectInstall(t.Context())
			if err != nil {
				t.Fatalf("detectInstall() error = %v", err)
			}
			if info.Method != string(tc.wantMethod) || info.Managed != tc.wantManaged {
				t.Fatalf("detectInstall() = %#v, want method=%q managed=%t", info, tc.wantMethod, tc.wantManaged)
			}
		})
	}
}
