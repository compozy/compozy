//go:build darwin

package mcp

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net"

	"golang.org/x/sys/unix"
)

const darwinProcArgsHeaderSize = 4

func peerInfoFromUnixConn(conn *net.UnixConn) (PeerInfo, error) {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return PeerInfo{}, fmt.Errorf("mcp: get unix raw connection: %w", err)
	}
	var peer PeerInfo
	var sysErr error
	controlErr := rawConn.Control(func(fd uintptr) {
		fdInt, err := darwinUnixFDToInt(fd)
		if err != nil {
			sysErr = err
			return
		}
		xucred, err := unix.GetsockoptXucred(fdInt, unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			sysErr = fmt.Errorf("mcp: read uds peer credentials: %w", err)
			return
		}
		if xucred.Ngroups <= 0 {
			sysErr = errors.New("mcp: uds peer credentials contain no groups")
			return
		}
		pid, err := unix.GetsockoptInt(fdInt, unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		if err != nil {
			sysErr = fmt.Errorf("mcp: read uds peer pid: %w", err)
			return
		}
		executablePath, err := darwinProcPath(pid)
		if err != nil {
			sysErr = err
			return
		}
		peer = PeerInfo{
			PID:            pid,
			UID:            int(xucred.Uid),
			GID:            int(xucred.Groups[0]),
			ExecutablePath: executablePath,
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

func darwinProcPath(pid int) (string, error) {
	if pid <= 0 {
		return "", errors.New("mcp: peer pid is required")
	}
	processArgs, err := unix.SysctlRaw("kern.procargs2", pid)
	if err != nil {
		return "", fmt.Errorf("mcp: resolve executable for peer pid %d: %w", pid, err)
	}
	if len(processArgs) <= darwinProcArgsHeaderSize {
		return "", fmt.Errorf("mcp: resolve executable for peer pid %d: missing process arguments", pid)
	}
	pathBytes := processArgs[darwinProcArgsHeaderSize:]
	if nulIndex := bytes.IndexByte(pathBytes, 0); nulIndex >= 0 {
		pathBytes = pathBytes[:nulIndex]
	}
	if len(pathBytes) == 0 {
		return "", fmt.Errorf("mcp: resolve executable for peer pid %d: empty path", pid)
	}
	return string(pathBytes), nil
}

func darwinUnixFDToInt(fd uintptr) (int, error) {
	if fd > uintptr(math.MaxInt) {
		return 0, fmt.Errorf("mcp: unix file descriptor %d exceeds int range", fd)
	}
	return int(fd), nil
}
