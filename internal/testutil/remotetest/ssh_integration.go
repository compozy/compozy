//go:build integration

package remotetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	"github.com/compozy/compozy/internal/update"
)

const (
	SSHWorkspace   = SSHHome + "/code/api"
	SSHAgent       = "ssh-fixture-agent"
	SSHHome        = "/home/pedro"
	SSHUser        = "pedro"
	sshFixturePort = 22
	sshFixtureLog  = "/var/log/compozy-fixture-sshd.log"
)

type SSHAssets struct {
	Image, Gateway, Binary, ArchiveName string
	Archive                             []byte
}

func PrepareSSHAssets(t Fixture, repo string) SSHAssets {
	t.Helper()
	image := "compozy-ssh-fixture-" + strings.ToLower(rand.Text())
	t.Cleanup(func() { cleanDocker(t, "image", "rm", image) })
	dockerOutput(
		t,
		4*time.Minute,
		"build",
		"--quiet",
		"-t",
		image,
		filepath.Join(repo, "internal/cloud/api/testdata/sshsetup"),
	)
	gateway := dockerOutput(t, 10*time.Second, "network", "inspect", "bridge",
		"--format", "{{(index .IPAM.Config 0).Gateway}}")
	if net.ParseIP(gateway) == nil {
		t.Fatalf("invalid fixture network gateway %q", gateway)
	}
	binary := buildSSHBinary(t, repo)
	archiveName, err := update.ArchiveAssetName("linux", runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	return SSHAssets{image, gateway, binary, archiveName, fixtureReleaseArchive(t, binary)}
}

func buildSSHBinary(t Fixture, repo string) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "compozy")
	ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags",
		"-X github.com/compozy/compozy/internal/version.Version="+SSHFixtureVersion, "-o", binary, "./cmd/compozy")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build the fixture release binary: %v\n%s", err, out)
	}
	return binary
}

func fixtureReleaseArchive(t Fixture, binary string) []byte {
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

type SSHHost struct {
	Container, cloud, address string
	Port, sshdPort            int
}

func StartSSHHost(
	t Fixture,
	assets SSHAssets,
	cloudURL, clientPublicKey, bastion string,
) SSHHost {
	t.Helper()
	name := "compozy-ssh-" + strings.ToLower(rand.Text())
	args := []string{"run", "--detach", "--name", name, "--privileged", "--cgroupns=host",
		"--volume", "/sys/fs/cgroup:/sys/fs/cgroup:rw", "--tmpfs", "/run", "--tmpfs", "/run/lock"}
	host := SSHHost{Container: name, cloud: cloudURL, sshdPort: sshFixturePort, address: "0.0.0.0"}
	if bastion == "" {
		args = append(args, "--publish", "127.0.0.1::22")
	} else {
		args = append(args, "--network", "container:"+bastion)
		host.sshdPort, host.address = 2222, "127.0.0.1"
	}
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("sshd log from %s:\n%s", name, host.ReadLog())
			t.Logf("user service log from %s:\n%s", name, host.ReadCommandOutput(
				"journalctl", "--no-pager", "-n", "80", "_SYSTEMD_USER_UNIT=compozy.service",
			))
		}
		cleanDocker(t, "rm", "-f", name)
	})
	dockerOutput(t, 30*time.Second, append(args, assets.Image)...)
	host.Command(t, "mkdir", "-p", "/run/sshd", SSHHome+"/.ssh", SSHHome+"/.compozy")
	host.Write(t, "/etc/compozy-fixture-cloud", cloudURL+"\n")
	host.Write(t, SSHHome+"/.ssh/authorized_keys", clientPublicKey)
	host.Write(t, SSHHome+"/.compozy/config.toml", "[http]\nhost = '127.0.0.1'\n[automation]\nenabled = false\n")
	host.Command(t, "chown", "-R", SSHUser+":"+SSHUser, SSHHome)
	Await(
		t,
		"systemd logind in SSH container",
		30*time.Second*time.Duration(testutil.TimeScale()),
		func(ctx context.Context) bool {
			_, err := host.tryCommand(ctx, "loginctl", "enable-linger", SSHUser)
			return err == nil
		},
	)
	Await(
		t,
		"the SSH user's systemd manager",
		15*time.Second*time.Duration(testutil.TimeScale()),
		func(ctx context.Context) bool {
			_, err := host.tryCommand(ctx, "runuser", "-u", SSHUser, "--", "env", "XDG_RUNTIME_DIR=/run/user/1500",
				"systemctl", "--user", "show-environment")
			return err == nil
		},
	)
	host.Command(t, "ssh-keygen", "-A")
	host.startOpenSSH(t)
	host.Port = host.sshdPort
	if bastion == "" {
		address := dockerOutput(t, 10*time.Second, "port", name, "22/tcp")
		_, value, err := net.SplitHostPort(address)
		if err != nil {
			t.Fatal(err)
		}
		host.Port, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	return host
}

func (h SSHHost) startOpenSSH(t Fixture) {
	t.Helper()
	dockerOutput(t, 10*time.Second, "exec", "--detach", h.Container, "/usr/sbin/sshd", "-D",
		"-f", "/etc/ssh/compozy_fixture_config", "-p", strconv.Itoa(h.sshdPort),
		"-o", "ListenAddress="+h.address, "-E", sshFixtureLog)
	Await(t, "OpenSSH listener", 10*time.Second*time.Duration(testutil.TimeScale()), func(ctx context.Context) bool {
		out, err := h.tryCommand(ctx, "test", "-s", "/run/compozy-fixture-sshd.pid")
		return err == nil && len(out) == 0
	})
}

func (h SSHHost) RotateKey(t Fixture) {
	t.Helper()
	h.Command(t, "sh", "-ec", `kill "$(cat /run/compozy-fixture-sshd.pid)"; rm -f /run/compozy-fixture-sshd.pid
rm /etc/ssh/ssh_host_ed25519_key /etc/ssh/ssh_host_ed25519_key.pub
ssh-keygen -A`)
	h.startOpenSSH(t)
}

func (h SSHHost) HostKey(t Fixture) ssh.PublicKey {
	t.Helper()
	data := h.Command(t, "cat", "/etc/ssh/ssh_host_ed25519_key.pub")
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (h SSHHost) Write(t Fixture, destination, body string) {
	t.Helper()
	h.Command(t, "sh", "-ec", `printf '%s' "$1" > "$2"`, "fixture", body, destination)
}

func (h SSHHost) Copy(t Fixture, source, path string) {
	t.Helper()
	dockerOutput(t, 15*time.Second, "cp", source, h.Container+":"+path)
}

func (h SSHHost) CLI(t Fixture, args ...string) string {
	t.Helper()
	return dockerOutput(t, 25*time.Second, append([]string{"exec", "--user", SSHUser,
		"-e", "HOME=" + SSHHome, "-e", "COMPOZY_CLOUD_URL=" + h.cloud, h.Container,
		SSHHome + "/.compozy/bin/compozy"}, args...)...)
}

func (h SSHHost) Command(t Fixture, args ...string) string {
	t.Helper()
	return dockerOutput(t, 10*time.Second, append([]string{"exec", h.Container}, args...)...)
}

func (h SSHHost) tryCommand(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", append([]string{"exec", h.Container}, args...)...).CombinedOutput()
}

func (h SSHHost) ReadLog() string {
	return h.ReadCommandOutput("cat", sshFixtureLog)
}

func (h SSHHost) ReadCommandOutput(args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, "docker", append([]string{"exec", h.Container}, args...)...).Output()
	return string(out)
}

func dockerOutput(t Fixture, timeout time.Duration, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", args[0], err, out)
	}
	return strings.TrimSpace(string(out))
}

func cleanDocker(t Fixture, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No such container:") &&
		!strings.Contains(string(out), "No such image:") {
		t.Errorf("cleanup docker %v: %v\n%s", args, err, out)
	}
}

func ConfigureSSHACP(t Fixture, host SSHHost, fixture string) {
	t.Helper()
	paths, err := config.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registration, err := acpmock.Register(paths, acpmock.RegisterOptions{
		FixturePath: fixture, FixtureAgent: SSHAgent, AgentName: SSHAgent, ProviderName: acpmock.ProviderName,
	})
	if err != nil {
		t.Fatal(err)
	}
	definition, err := os.ReadFile(registration.AgentDefPath)
	if err != nil {
		t.Fatal(err)
	}
	const driver = SSHHome + "/acpmock-driver"
	const script = SSHHome + "/acpmock-fixture.json"
	host.Copy(t, registration.DriverPath, driver)
	host.Copy(t, fixture, script)
	host.Command(t, "mkdir", "-p", SSHWorkspace, SSHHome+"/.compozy/agents/"+SSHAgent)
	declared := strings.NewReplacer(registration.DriverPath, driver, registration.FixturePath, script,
		registration.DiagnosticsPath, SSHHome+"/acpmock-diagnostics.jsonl").Replace(string(definition))
	host.Write(t, SSHHome+"/.compozy/agents/"+SSHAgent+"/AGENT.md", declared)
	provider := acpmock.ProviderConfig(driver)
	cfg := host.Command(t, "cat", SSHHome+"/.compozy/config.toml") + fmt.Sprintf(`
[providers.acpmock]
command = %q
display_name = %q
harness = %q
auth_mode = %q
none_security = %q
`, provider.Command, provider.DisplayName, provider.Harness, provider.AuthMode, provider.NoneSecurity)
	host.Write(t, SSHHome+"/.compozy/config.toml", cfg)
	host.Command(t, "chown", "-R", SSHUser+":"+SSHUser, SSHHome)
}
