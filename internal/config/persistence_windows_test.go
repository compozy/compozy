//go:build windows

package config

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"
)

// Invariant: unchanged configuration loads with read rights; retirement requires write rights.
// Owner: persisted config loading. Windows ACLs require this platform-specific suite.
func TestPersistedOverlayWindowsReadPermissions(t *testing.T) {
	t.Parallel()
	operator, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		migrate bool
	}{
		{name: "Should read unchanged config"},
		{name: "Should refuse retirement without write rights", migrate: true},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				path := filepath.Join(root, "config.toml")
				content := "[skills]\nenabled = false\n"
				if tc.migrate {
					content += "[skills.marketplace]\nregistry = 'clawhub'\n"
				}
				writeFile(t, path, content)
				if output, err := exec.Command("icacls", root, "/deny", operator.Username+":(WD,AD,WEA,WA)").
					CombinedOutput(); err != nil {
					t.Fatalf("deny directory writes: %v: %s", err, output)
				}
				t.Cleanup(func() {
					if output, err := exec.Command("icacls", root, "/remove:d", operator.Username).
						CombinedOutput(); err != nil {
						t.Errorf("restore directory ACL: %v: %s", err, output)
					}
				})
				overlay, err := loadPersistedConfigOverlay(path, loadConfigOverlayBytes)
				if tc.migrate != (err != nil) {
					t.Fatalf("migration=%v, overlay=%+v, error=%v", tc.migrate, overlay, err)
				}
				got, err := os.ReadFile(path)
				if err != nil || string(got) != content {
					t.Fatalf("read-only config changed: %q, %v", got, err)
				}
			},
		)
	}
}
