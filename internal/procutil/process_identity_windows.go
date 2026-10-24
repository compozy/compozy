//go:build windows

package procutil

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

// InspectWindowsProcess reads the token user SID and executable from one owned process handle.
func InspectWindowsProcess(pid uint32) (userSID string, executablePath string, err error) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", "", fmt.Errorf("procutil: open Windows process: %w", err)
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(process)) }()
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return "", "", fmt.Errorf("procutil: open Windows process token: %w", err)
	}
	defer func() { err = errors.Join(err, token.Close()) }()
	user, err := token.GetTokenUser()
	if err != nil {
		return "", "", fmt.Errorf("procutil: read Windows process user: %w", err)
	}
	path := make([]uint16, 32768)
	size := uint32(len(path))
	if err := windows.QueryFullProcessImageName(process, 0, &path[0], &size); err != nil {
		return "", "", fmt.Errorf("procutil: resolve Windows process executable: %w", err)
	}
	return user.User.Sid.String(), windows.UTF16ToString(path[:size]), nil
}
