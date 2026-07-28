package update

import (
	"context"
	"errors"
	"testing"
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

				manager := testManager(t, tc.cfg)
				info := manager.detectInstall(context.Background())
				if !info.Managed {
					t.Fatal("detectInstall() managed = false, want true")
				}
				if info.Method != tc.want {
					t.Fatalf("detectInstall() method = %q, want %q", info.Method, tc.want)
				}
			})
		}
	})
}
