package remotehost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"

	"golang.org/x/sys/execabs"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/procutil"
)

const portBlockedDiagnosis = "Hosted route can't reach Cloudflare (port 7844 blocked)"

type cloudflaredDiagnostics struct {
	tail    []byte
	blocked atomic.Bool
}

var _ io.Writer = (*cloudflaredDiagnostics)(nil)

func (d *cloudflaredDiagnostics) Write(p []byte) (int, error) {
	d.tail = append(d.tail, p...)
	for line := range bytes.SplitSeq(d.tail, []byte{'\n'}) {
		if bytes.Contains(line, []byte("failed to dial to edge with quic")) && bytes.Contains(line, []byte("7844")) {
			d.blocked.Store(true)
		}
	}
	_, last, found := bytes.CutLast(d.tail, []byte{'\n'})
	if !found {
		last = d.tail
	}
	d.tail = append(d.tail[:0], last[max(0, len(last)-2048):]...)
	return len(p), nil
}

func (d *cloudflaredDiagnostics) diagnosis() string {
	if d.blocked.Load() {
		return portBlockedDiagnosis
	}
	return ""
}

func cloudflaredEnv(home, token string) []string {
	allowed := []string{"HOME", "USERPROFILE", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "TMPDIR",
		"HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "SSL_CERT_FILE", "SSL_CERT_DIR"}
	env := make([]string, 0, len(allowed)+3)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if slices.Contains(allowed, strings.ToUpper(name)) {
			env = append(env, entry)
		}
	}
	return append(env, "COMPOZY_HOME="+home, "COMPOZY_REMOTE_CONNECTOR=1", "TUNNEL_TOKEN="+token)
}

type cloudflaredProcess struct {
	cmd         *exec.Cmd
	done        <-chan error
	readyURL    string
	diagnostics *cloudflaredDiagnostics
}

func startCloudflared(
	ctx context.Context, home, binary, token string,
) (*cloudflaredProcess, error) {
	canonicalHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		return nil, err
	}
	root, executable, err := fileutil.ResolveExistingPathWithinRoot(cloudflaredDirectory(canonicalHome), binary)
	if err != nil {
		return nil, err
	}
	if err := procutil.ReapRemoteConnectors(ctx, canonicalHome, root); err != nil {
		return nil, err
	}
	config := filepath.Join(filepath.Dir(executable), "compozy-cloudflared.yml")
	if err := fileutil.AtomicWriteFile(config, []byte("{}\n"), 0o600); err != nil {
		return nil, err
	}
	address, err := cloudflaredMetricsAddress(ctx)
	if err != nil {
		return nil, err
	}
	diagnostics := &cloudflaredDiagnostics{}
	cmd := execabs.Command(executable, "tunnel", "--config", config, "--metrics", address, "--no-autoupdate", "run")
	cmd.Env = cloudflaredEnv(canonicalHome, token)
	cmd.Stderr = diagnostics
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	done, err := procutil.StartParentOwnedCommand(cmd)
	if err != nil {
		return nil, err
	}
	return &cloudflaredProcess{
		cmd:         cmd,
		done:        done,
		readyURL:    "http://" + address + "/ready",
		diagnostics: diagnostics,
	}, nil
}

func cloudflaredMetricsAddress(ctx context.Context) (string, error) {
	var config net.ListenConfig
	socket, err := config.Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	return socket.Addr().String(), socket.Close()
}

func (p *cloudflaredProcess) ready(ctx context.Context, client *http.Client) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.readyURL, http.NoBody)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	_, drainErr := io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK, errors.Join(drainErr, resp.Body.Close())
}

func (p *cloudflaredProcess) stop(logger *slog.Logger) {
	if err := procutil.StopParentOwnedCommand(p.cmd, p.done); err != nil {
		logger.Warn("Could not confirm the hosted connector stopped", "error", err)
	}
}
