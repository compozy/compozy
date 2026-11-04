//go:build windows

package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/tailscale/go-winio"
	"golang.org/x/sys/windows"
)

var appControlPipePattern = regexp.MustCompile(`^\\\\\.\\pipe\\compozyos-[a-f0-9]{64}$`)

func dialAppControl(ctx context.Context, path string) (net.Conn, error) {
	return dialAppControlWithProcessInspector(ctx, path, procutil.InspectWindowsProcess)
}

func dialAppControlWithProcessInspector(
	ctx context.Context,
	path string,
	inspectProcess func(uint32) (string, string, error),
) (net.Conn, error) {
	contents, err := fileutil.ReadPrivateFile(path)
	if err != nil {
		return nil, fmt.Errorf("app control: read pipe discovery: %w", err)
	}
	address := strings.TrimSpace(string(contents))
	if !appControlPipePattern.MatchString(address) {
		return nil, errors.New("app control: invalid local pipe address")
	}
	connection, err := winio.DialPipeContext(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("app control: connect to pipe: %w", err)
	}
	if err := validateAppControlPipeOwner(connection, inspectProcess); err != nil {
		return nil, errors.Join(err, connection.Close())
	}
	return connection, nil
}

func validateAppControlPipeOwner(connection net.Conn, inspectProcess func(uint32) (string, string, error)) error {
	file, ok := connection.(interface{ Fd() uintptr })
	if !ok {
		return errors.New("app control: pipe handle is unavailable")
	}
	var pid uint32
	if err := windows.GetNamedPipeServerProcessId(windows.Handle(file.Fd()), &pid); err != nil {
		return fmt.Errorf("app control: inspect pipe server: %w", err)
	}
	sid, _, err := inspectProcess(pid)
	if err != nil {
		return fmt.Errorf("app control: inspect pipe owner: %w", err)
	}
	operator, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("app control: inspect current user: %w", err)
	}
	if sid != operator.User.Sid.String() {
		return errors.New("app control: pipe server belongs to another user")
	}
	return nil
}
