package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	sshBusyCode             = "gateway_ssh_busy"
	sshOwnerNamespacePrefix = "cpz_ssh_"
	sshOwnerTokenBytes      = 24
	sshOwnershipAcquired    = "acquired"
	sshOwnershipBusy        = "busy"
	sshOwnershipAbsent      = "absent"
	sshOwnershipNotOwner    = "not-owner"
	sshOwnershipMarkerState = "present"
)

type sshDaemonOwnership struct {
	token string
}

func newSSHDaemonOwnership(entropy io.Reader) (*sshDaemonOwnership, error) {
	if entropy == nil {
		entropy = rand.Reader
	}
	raw := make([]byte, sshOwnerTokenBytes)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return nil, fmt.Errorf("cli: generate SSH ownership token: %w", err)
	}
	return &sshDaemonOwnership{
		token: sshOwnerNamespacePrefix + base64.RawURLEncoding.EncodeToString(raw),
	}, nil
}

func inspectSSHDaemonOwnership(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
) (bool, error) {
	output, err := executor.Run(ctx, target, sshOwnershipCommand(sshOwnershipInspectScript, remoteHome))
	if err != nil {
		return false, fmt.Errorf("cli: inspect remote SSH ownership: %w", classifySSHFailure(err))
	}
	switch strings.TrimSpace(string(output)) {
	case sshOwnershipMarkerState:
		return true, nil
	case sshOwnershipAbsent:
		return false, nil
	default:
		return false, errors.New("cli: remote SSH ownership response is invalid")
	}
}

func newSSHBusyError() error {
	return &gatewayClientError{
		code: sshBusyCode, statusCode: http.StatusConflict,
		message: "another SSH connection owns this remote CompozyOS home; retry after it disconnects",
	}
}

func sshOwnershipCommand(script string, arguments ...string) []string {
	command := []string{"sh", "-c", script, "compozy-ssh-owner"}
	return append(command, arguments...)
}

const sshOwnershipShellPrelude = `
set -eu
explicit_home=$1
if [ -n "$explicit_home" ]; then
  compozy_home=$explicit_home
elif [ -n "${COMPOZY_HOME:-}" ]; then
  compozy_home=$COMPOZY_HOME
else
  compozy_home=$HOME/.compozy
fi
owner_dir=$compozy_home/gateway/ssh-connect-owner
token_file=$owner_dir/token
`

const sshOwnershipInspectScript = sshOwnershipShellPrelude + `
if [ -d "$owner_dir" ]; then
  printf '%s\n' 'present'
else
  printf '%s\n' 'absent'
fi
`

const sshOwnershipLeaseScript = sshOwnershipShellPrelude + `
token=$2
umask 077
mkdir -p "$compozy_home/gateway"
if mkdir "$owner_dir" 2>/dev/null; then
  printf '%s\n' "$token" > "$token_file"
  printf '%s\n' 'acquired'
  cleanup_owner() {
    if [ -f "$token_file" ] && [ "$(cat "$token_file")" = "$token" ]; then
      rm -f "$token_file"
      rmdir "$owner_dir" 2>/dev/null || true
    fi
  }
  trap cleanup_owner EXIT HUP INT TERM
  while IFS= read -r _; do :; done
else
  printf '%s\n' 'busy'
fi
`

const sshOwnershipStopScript = sshOwnershipShellPrelude + `
token=$2
if [ ! -f "$token_file" ] || [ "$(cat "$token_file")" != "$token" ]; then
  printf '%s\n' 'not-owner'
else
  env COMPOZY_HOME="$compozy_home" compozy daemon stop -o json
fi
`
