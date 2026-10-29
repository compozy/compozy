package procutil

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// accountShellLookupTimeout bounds one directory-services lookup.
const accountShellLookupTimeout = 2 * time.Second

// ErrAccountShellUnavailable reports that the account database names no usable login shell.
var ErrAccountShellUnavailable = errors.New("procutil: account login shell is unavailable")

// AccountLoginShell returns the login shell recorded for the current user's account.
//
// A daemon started by launchd, the desktop app, or a service manager inherits
// whatever SHELL its launcher had, which is not necessarily the shell the
// operator chose with chsh. The account database (directory services on
// macOS, passwd/NSS elsewhere) is the authority for that choice.
func AccountLoginShell(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("procutil: account login shell context is required")
	}
	lookupCtx, cancel := context.WithTimeout(ctx, accountShellLookupTimeout)
	defer cancel()
	shell, err := lookupAccountShell(lookupCtx, os.Getuid())
	if err != nil {
		return "", err
	}
	return validateAccountShell(shell)
}

func validateAccountShell(shell string) (string, error) {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return "", fmt.Errorf("%w: no shell recorded", ErrAccountShellUnavailable)
	}
	if !filepath.IsAbs(shell) {
		return "", fmt.Errorf("%w: %q is not an absolute path", ErrAccountShellUnavailable, shell)
	}
	info, err := os.Stat(shell)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrAccountShellUnavailable, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%w: %q is not an executable file", ErrAccountShellUnavailable, shell)
	}
	return shell, nil
}

// parseDirectoryServicesShell reads `dscacheutil -q user -a uid <uid>` output.
func parseDirectoryServicesShell(output []byte, uid int) (string, error) {
	wantUID := strconv.Itoa(uid)
	var recordUID, shell string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			if recordUID == wantUID && shell != "" {
				return shell, nil
			}
			recordUID, shell = "", ""
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "uid":
			recordUID = strings.TrimSpace(value)
		case "shell":
			shell = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("procutil: read directory services record: %w", err)
	}
	if recordUID == wantUID && shell != "" {
		return shell, nil
	}
	return "", fmt.Errorf("%w: no directory services record for uid %d", ErrAccountShellUnavailable, uid)
}

// parsePasswdShell returns the shell field of the passwd entry for uid.
func parsePasswdShell(passwd []byte, uid int) (string, error) {
	wantUID := strconv.Itoa(uid)
	scanner := bufio.NewScanner(bytes.NewReader(passwd))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != 7 || fields[2] != wantUID {
			continue
		}
		return fields[6], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("procutil: read passwd entries: %w", err)
	}
	return "", fmt.Errorf("%w: no passwd entry for uid %d", ErrAccountShellUnavailable, uid)
}
