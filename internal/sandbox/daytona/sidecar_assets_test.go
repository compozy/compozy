package daytona

import (
	"bytes"
	"debug/buildinfo"
	"log/slog"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestSidecarTransportBinaryAssetsContract(t *testing.T) {
	// not parallel: this test intentionally hides PATH to prove runtime sidecar lookup does not use host tools.
	t.Run("Should resolve embedded Linux sidecar binary without host Go toolchain", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())

		bootstrap := &fakeTransport{readArchives: [][]byte{[]byte("aarch64\n")}}
		transport := &sidecarTransport{
			logger:    slog.New(slog.DiscardHandler),
			bootstrap: bootstrap,
			binaries:  make(map[string][]byte),
		}

		binary, err := transport.sidecarBinary(testutil.Context(t), sandboxInfo{ID: "sandbox-sidecar"})
		if err != nil {
			t.Fatalf("sidecarBinary() error = %v", err)
		}
		if !bytes.HasPrefix(binary, []byte{0x7f, 'E', 'L', 'F'}) {
			t.Fatalf("sidecarBinary() prefix = %x, want ELF binary", binary[:min(len(binary), 4)])
		}
		if got, want := len(bootstrap.dials), 1; got != want {
			t.Fatalf("bootstrap dials = %d, want %d", got, want)
		}
	})

	tests := []struct {
		name            string
		arch            string
		architectureKey string
		architectureVal string
	}{
		{
			name:            "Should omit VCS metadata from embedded amd64 sidecar binary",
			arch:            launcherSidecarArchAMD64,
			architectureKey: "GOAMD64",
			architectureVal: "v1",
		},
		{
			name:            "Should omit VCS metadata from embedded arm64 sidecar binary",
			arch:            launcherSidecarArchARM64,
			architectureKey: "GOARM64",
			architectureVal: "v8.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// not parallel: this suite includes a PATH-hiding subtest with t.Setenv.

			binary, err := embeddedLauncherSidecarBinary(tt.arch)
			if err != nil {
				t.Fatalf("embeddedLauncherSidecarBinary(%q) error = %v", tt.arch, err)
			}
			info, err := buildinfo.Read(bytes.NewReader(binary))
			if err != nil {
				t.Fatalf("buildinfo.Read(%q) error = %v", tt.arch, err)
			}
			settings := make(map[string]string, len(info.Settings))
			for _, setting := range info.Settings {
				settings[setting.Key] = setting.Value
				if setting.Key == "vcs" || strings.HasPrefix(setting.Key, "vcs.") {
					t.Fatalf(
						"embedded sidecar %s contains VCS build setting %s=%q",
						tt.arch,
						setting.Key,
						setting.Value,
					)
				}
			}
			requireBuildSetting(t, settings, "-trimpath", "true")
			requireBuildSetting(t, settings, "CGO_ENABLED", "0")
			requireBuildSetting(t, settings, "GOOS", "linux")
			requireBuildSetting(t, settings, "GOARCH", tt.arch)
			requireBuildSetting(t, settings, tt.architectureKey, tt.architectureVal)
		})
	}
}

func requireBuildSetting(t *testing.T, settings map[string]string, key string, want string) {
	t.Helper()

	got, ok := settings[key]
	if !ok || got != want {
		t.Fatalf("build setting %s = %q, found %v, want %q", key, got, ok, want)
	}
}
