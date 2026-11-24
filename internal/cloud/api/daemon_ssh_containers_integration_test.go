//go:build integration && linux

package api_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/update"
)

const (
	sshFixtureVersion = "0.8.0"
	sshFixtureHome    = "/home/pedro"
	sshFixtureUser    = "pedro"
	sshFixturePort    = 22
	sshFixtureLog     = "/var/log/compozy-fixture-sshd.log"
)

type sshFixtureAssets struct {
	image, gateway, binary, archiveName string
	archive                             []byte
}

func prepareSSHFixtureAssets(t *testing.T) sshFixtureAssets {
	t.Helper()
	image := "compozy-ssh-fixture-" + strings.ToLower(rand.Text())
	dockerOutput(t, 4*time.Minute, "build", "--quiet", "-t", image, "testdata/sshsetup")
	t.Cleanup(func() { cleanDocker(t, "image", "rm", image) })
	gateway := dockerOutput(t, 10*time.Second, "network", "inspect", "bridge",
		"--format", "{{(index .IPAM.Config 0).Gateway}}")
	if net.ParseIP(gateway) == nil {
		t.Fatalf("invalid fixture network gateway %q", gateway)
	}
	binary := filepath.Join(t.TempDir(), "compozy")
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags",
		"-X github.com/compozy/compozy/internal/version.Version="+sshFixtureVersion, "-o", binary, "./cmd/compozy")
	cmd.Dir = "../../.."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the fixture release binary: %v\n%s", err, out)
	}
	archiveName, err := update.ArchiveAssetName("linux", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return sshFixtureAssets{image, gateway, binary, archiveName, fixtureReleaseArchive(t, binary)}
}

func fixtureReleaseArchive(t *testing.T, binary string) []byte {
	t.Helper()
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	zipper, err := gzip.NewWriterLevel(&compressed, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(zipper)
	if err := archive.WriteHeader(&tar.Header{Name: "compozy", Mode: 0o755, Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipper.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

type sshFixtureHost struct {
	container, cloud, address string
	port, sshdPort            int
}

func startSSHFixtureHost(
	t *testing.T,
	assets sshFixtureAssets,
	cloudURL, clientPublicKey, bastion string,
) sshFixtureHost {
	t.Helper()
	name := "compozy-ssh-" + strings.ToLower(rand.Text())
	args := []string{"run", "--detach", "--name", name, "--privileged", "--cgroupns=host",
		"--volume", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock"}
	host := sshFixtureHost{container: name, cloud: cloudURL, sshdPort: sshFixturePort, address: "0.0.0.0"}
	if bastion == "" {
		args = append(args, "--publish", "127.0.0.1::22")
	} else {
		args = append(args, "--network", "container:"+bastion)
		host.sshdPort, host.address = 2222, "127.0.0.1"
	}
	dockerOutput(t, 30*time.Second, append(args, assets.image)...)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("sshd log from %s:\n%s", name, host.readLog())
			t.Logf("user service log from %s:\n%s", name, host.readCommandOutput(
				"journalctl", "--no-pager", "-n", "80", "_SYSTEMD_USER_UNIT=compozy.service",
			))
		}
		cleanDocker(t, "rm", "-f", name)
	})
	host.command(t, "mkdir", "-p", "/run/sshd", sshFixtureHome+"/.ssh", sshFixtureHome+"/.compozy")
	host.write(t, "/etc/compozy-fixture-cloud", cloudURL+"\n")
	host.write(t, sshFixtureHome+"/.ssh/authorized_keys", clientPublicKey)
	host.write(t, sshFixtureHome+"/.compozy/config.toml", "[http]\nhost = '127.0.0.1'\n[automation]\nenabled = false\n")
	host.command(t, "chown", "-R", sshFixtureUser+":"+sshFixtureUser, sshFixtureHome)
	testutil.Eventually(t, "systemd logind in SSH container", 30*time.Second, func() bool {
		_, err := host.tryCommand(t, "loginctl", "enable-linger", sshFixtureUser)
		return err == nil
	})
	testutil.Eventually(t, "the SSH user's systemd manager", 15*time.Second, func() bool {
		_, err := host.tryCommand(t, "runuser", "-u", sshFixtureUser, "--", "env", "XDG_RUNTIME_DIR=/run/user/1500",
			"systemctl", "--user", "show-environment")
		return err == nil
	})
	host.command(t, "ssh-keygen", "-A")
	host.startOpenSSH(t)
	host.port = host.sshdPort
	if bastion == "" {
		address := dockerOutput(t, 10*time.Second, "port", name, "22/tcp")
		_, value, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		host.port, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	return host
}

func (h sshFixtureHost) startOpenSSH(t *testing.T) {
	t.Helper()
	dockerOutput(t, 10*time.Second, "exec", "--detach", h.container, "/usr/sbin/sshd", "-D",
		"-f", "/etc/ssh/compozy_fixture_config", "-p", strconv.Itoa(h.sshdPort),
		"-o", "ListenAddress="+h.address, "-E", sshFixtureLog)
	testutil.Eventually(t, "OpenSSH listener", 10*time.Second, func() bool {
		out, err := h.tryCommand(t, "test", "-s", "/run/compozy-fixture-sshd.pid")
		return err == nil && len(out) == 0
	})
}

func (h sshFixtureHost) rotateKey(t *testing.T) {
	t.Helper()
	h.command(t, "sh", "-ec", `kill "$(cat /run/compozy-fixture-sshd.pid)"; rm -f /run/compozy-fixture-sshd.pid
rm /etc/ssh/ssh_host_ed25519_key /etc/ssh/ssh_host_ed25519_key.pub
ssh-keygen -A`)
	h.startOpenSSH(t)
}

func (h sshFixtureHost) hostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	data := h.command(t, "cat", "/etc/ssh/ssh_host_ed25519_key.pub")
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (h sshFixtureHost) write(t *testing.T, destination, body string) {
	t.Helper()
	h.command(t, "sh", "-ec", `printf '%s' "$1" > "$2"`, "fixture", body, destination)
}

func (h sshFixtureHost) copy(t *testing.T, source, path string) {
	t.Helper()
	dockerOutput(t, 15*time.Second, "cp", source, h.container+":"+path)
}

func (h sshFixtureHost) cli(t *testing.T, args ...string) string {
	t.Helper()
	return dockerOutput(t, 25*time.Second, append([]string{"exec", "--user", sshFixtureUser,
		"-e", "HOME=" + sshFixtureHome, "-e", "COMPOZY_CLOUD_URL=" + h.cloud, h.container,
		sshFixtureHome + "/.compozy/bin/compozy"}, args...)...)
}

func (h sshFixtureHost) command(t *testing.T, args ...string) string {
	t.Helper()
	return dockerOutput(t, 10*time.Second, append([]string{"exec", h.container}, args...)...)
}

func (h sshFixtureHost) tryCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", append([]string{"exec", h.container}, args...)...).CombinedOutput()
}

func (h sshFixtureHost) readLog() string {
	return h.readCommandOutput("cat", sshFixtureLog)
}

func (h sshFixtureHost) readCommandOutput(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "docker", append([]string{"exec", h.container}, args...)...).Output()
	return string(out)
}

func dockerOutput(t *testing.T, timeout time.Duration, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", args[0], err, out)
	}
	return strings.TrimSpace(string(out))
}

func cleanDocker(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput(); err != nil {
		t.Errorf("cleanup docker %v: %v\n%s", args, err, out)
	}
}
