//go:build linux

package mcp

import (
	"fmt"
	"math"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

func peerInfoFromUnixConn(conn *net.UnixConn) (PeerInfo, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: get unix raw connection: %w", err)
	}
	var peer PeerInfo
	var sysErr error
	controlErr := rawConn.Control(func(fd uintptr) {
		fdInt, err := unixFDToInt(fd)
		if err != nil {
			sysErr = err
			return
		}
		ucred, err := unix.GetsockoptUcred(fdInt, unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			sysErr = fmt.Errorf("mcp: read uds peer credentials: %w", err)
			return
		}
		exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", ucred.Pid))
		if err != nil {
			sysErr = fmt.Errorf("mcp: resolve executable for peer pid %d: %w", ucred.Pid, err)
			return
		}
		peer = PeerInfo{
			PID:            int(ucred.Pid),
			UID:            int(ucred.Uid),
			GID:            int(ucred.Gid),
			ExecutablePath: exe,
			Supported:      true,
		}
	})
	if controlErr != nil {
		return PeerInfo{}, fmt.Errorf("mcp: inspect unix peer: %w", controlErr)
	}
	if sysErr != nil {
		return PeerInfo{}, sysErr
	}
	return peer, nil
}

func unixFDToInt(fd uintptr) (int, error) {
	if fd > uintptr(math.MaxInt) {
		return 0, fmt.Errorf("mcp: unix file descriptor %d exceeds int range", fd)
	}
	return int(fd), nil
}
