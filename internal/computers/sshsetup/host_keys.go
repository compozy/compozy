package sshsetup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/compozy/compozy/internal/fileutil"
)

func (m *Manager) hostKey(ctx context.Context, job *setupRun, t target, fingerprint string) error {
	current, err := readHostKey(m.pins, t.knownHost())
	if err != nil {
		return err
	}
	if current != nil {
		return checkFingerprint(current, fingerprint)
	}
	key, err := m.userHostKey(ctx, t)
	if err != nil {
		return err
	}
	if key == nil {
		key, err = m.probeHostKey(ctx, t)
		if err != nil {
			return err
		}
	}
	if fingerprint == "" {
		if err := m.trustPrompt(ctx, job, t, ssh.FingerprintSHA256(key)); err != nil {
			return err
		}
	} else if err := checkFingerprint(key, fingerprint); err != nil {
		return err
	}
	return m.pinHostKey(t, key)
}

func checkFingerprint(key ssh.PublicKey, expected string) error {
	if expected != "" && ssh.FingerprintSHA256(key) != expected {
		return failure(http.StatusUnprocessableEntity, "ssh_host_key_changed",
			"The host key does not match the supplied fingerprint.")
	}
	return nil
}

func (m *Manager) userHostKey(ctx context.Context, t target) (ssh.PublicKey, error) {
	path := filepath.Join(m.sshDir, "known_hosts")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	// #nosec G204 -- searches a local file with a validated hostname, without contacting the host.
	cmd := exec.CommandContext(ctx, "ssh-keygen", "-F", t.knownHost(), "-f", path)
	out, err := cmd.Output()
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseHostKey(out, "")
}

func readHostKey(path, host string) (ssh.PublicKey, error) {
	// #nosec G304 -- only Compozy's known-hosts and private quarantine paths reach this helper.
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseHostKey(data, host)
}

func parseHostKey(data []byte, host string) (ssh.PublicKey, error) {
	for len(data) > 0 {
		marker, hosts, key, _, rest, err := ssh.ParseKnownHosts(data)
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if marker == "" && (host == "" || (len(hosts) == 1 && hosts[0] == host)) {
			return key, nil
		}
		data = rest
	}
	return nil, nil
}

func (m *Manager) probeHostKey(ctx context.Context, t target) (key ssh.PublicKey, resultErr error) {
	dir, err := os.MkdirTemp(filepath.Dir(m.pins), "probe-")
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, os.RemoveAll(dir)) }()
	quarantine := filepath.Join(dir, "known_hosts")
	args := append(sshOptions(t, quarantine),
		"-o", "StrictHostKeyChecking=accept-new", "-o", "PreferredAuthentications=none",
		"-o", "PubkeyAuthentication=no", "-o", "PasswordAuthentication=no", "-o", "KbdInteractiveAuthentication=no",
		"-o", "GSSAPIAuthentication=no", "-o", "HostbasedAuthentication=no",
		"-o", "BatchMode=yes", "-o", "ClearAllForwardings=yes", "-o", "HashKnownHosts=no", "-N", "--", t.Alias)
	probe, cancel := context.WithTimeout(ctx, connectBudget)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, runErr := m.ssh.run(probe, args, nil, nil)
		if runErr != nil {
			return // OpenSSH normally exits after the server refuses authentication.
		}
	}()
	defer func() { cancel(); <-done }()
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return quarantineKey(quarantine, t)
		case <-probe.Done():
			return nil, errInterrupted
		case <-ticker.C:
			key, err := readHostKey(quarantine, t.knownHost())
			if key != nil && err == nil {
				return key, nil
			}
		}
	}
}

func quarantineKey(path string, t target) (ssh.PublicKey, error) {
	key, err := readHostKey(path, t.knownHost())
	if key == nil && err == nil {
		err = errInterrupted
	}
	return key, err
}

func (m *Manager) pinHostKey(t target, key ssh.PublicKey) error {
	m.pinMu.Lock()
	defer m.pinMu.Unlock()
	current, err := readHostKey(m.pins, t.knownHost())
	if err != nil {
		return err
	}
	if current != nil {
		return checkFingerprint(current, ssh.FingerprintSHA256(key))
	}
	// #nosec G304 -- this file is private to the controller's home.
	contents, err := os.ReadFile(m.pins)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	line := fmt.Sprintf("%s %s", t.knownHost(), ssh.MarshalAuthorizedKey(key))
	return fileutil.AtomicWritePrivateFile(m.pins, []byte(strings.TrimSpace(string(contents))+"\n"+line))
}
