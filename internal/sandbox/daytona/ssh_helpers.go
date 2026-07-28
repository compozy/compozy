package daytona

import (
	"bytes"

	"errors"
	"fmt"

	"net"

	"os"
	"path/filepath"

	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultHostKeyCallback() ssh.HostKeyCallback {
	home, err := os.UserHomeDir()
	if err != nil {
		return func(hostname string, _ net.Addr, _ ssh.PublicKey) error {
			return fmt.Errorf("sandbox/daytona: resolve home for SSH known_hosts %q: %w", hostname, err)
		}
	}
	callback, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return func(hostname string, _ net.Addr, _ ssh.PublicKey) error {
			return fmt.Errorf("sandbox/daytona: load SSH known_hosts for %q: %w", hostname, err)
		}
	}
	return callback
}

func normalizeSSHWaitErr(err error, stderr *bytes.Buffer) error {
	if err == nil {
		return nil
	}
	if missing, ok := errors.AsType[*ssh.ExitMissingError](err); ok && missing != nil {
		if stderr == nil || strings.TrimSpace(stderr.String()) == "" {
			return nil
		}
	}
	return err
}
