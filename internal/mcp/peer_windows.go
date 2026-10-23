//go:build windows

package mcp

import (
	"errors"
	"fmt"
	"math"
	"net"
	"unsafe"

	"golang.org/x/sys/windows"
)

// afunix.h defines _WSAIOR(IOC_VENDOR, 256), returning an ULONG process ID.
const sioAFUnixGetPeerPID = windows.IOC_OUT | windows.IOC_VENDOR | 256

func peerInfoFromUnixConn(conn *net.UnixConn) (PeerInfo, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: get unix raw connection: %w", err)
	}
	var pid uint32
	var sysErr error
	if err := rawConn.Control(func(fd uintptr) {
		pid, sysErr = windowsSocketPeerPID(windows.Handle(fd))
	}); err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: inspect Windows unix peer: %w", err)
	}
	if sysErr != nil {
		return PeerInfo{}, sysErr
	}
	return windowsProcessPeerInfo(pid)
}

func windowsSocketPeerPID(socket windows.Handle) (uint32, error) {
	var pid, returned uint32
	err := windows.WSAIoctl(socket, sioAFUnixGetPeerPID, nil, 0,
		(*byte)(unsafe.Pointer(&pid)), uint32(unsafe.Sizeof(pid)), &returned, nil, 0)
	if err != nil {
		if errors.Is(err, windows.WSAEOPNOTSUPP) || errors.Is(err, windows.WSAEINVAL) ||
			errors.Is(err, windows.WSAENOPROTOOPT) {
			return 0, fmt.Errorf("%w: %w", ErrWindowsPeerCredentialsUnsupported, err)
		}
		return 0, fmt.Errorf("mcp: read Windows AF_UNIX peer pid: %w", err)
	}
	if returned != uint32(unsafe.Sizeof(pid)) || pid == 0 || uint64(pid) > uint64(math.MaxInt) {
		return 0, errors.New("mcp: invalid Windows AF_UNIX peer pid response")
	}
	return pid, nil
}

func windowsProcessPeerInfo(pid uint32) (peer PeerInfo, err error) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: open Windows peer process: %w", err)
	}
	defer func() { err = errors.Join(err, windows.CloseHandle(process)) }()
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_QUERY, &token); err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: open Windows peer token: %w", err)
	}
	defer func() { err = errors.Join(err, token.Close()) }()
	user, err := token.GetTokenUser()
	if err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: read Windows peer user: %w", err)
	}
	path := make([]uint16, 32768)
	size := uint32(len(path))
	if err := windows.QueryFullProcessImageName(process, 0, &path[0], &size); err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: resolve Windows peer executable: %w", err)
	}
	return PeerInfo{
		PID: int(pid), UID: -1, GID: -1, UserSID: user.User.Sid.String(),
		ExecutablePath: windows.UTF16ToString(path[:size]), Supported: true,
	}, nil
}

func validatePeerUser(peer PeerInfo) error {
	peerSID, err := windows.StringToSid(peer.UserSID)
	if err != nil {
		return fmt.Errorf("%w: missing or invalid Windows peer SID", ErrHostedPeerInvalid)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("%w: read daemon process user: %w", ErrHostedPeerInvalid, err)
	}
	if !peerSID.Equals(user.User.Sid) {
		return fmt.Errorf("%w: Windows user SID mismatch", ErrHostedPeerInvalid)
	}
	return nil
}
