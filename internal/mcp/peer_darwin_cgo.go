//go:build darwin && cgo

package mcp

/*
#include <libproc.h>
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"net"

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
		xucred, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			sysErr = fmt.Errorf("mcp: read uds peer credentials: %w", err)
			return
		}
		pid, err := unix.GetsockoptInt(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERPID)
		if err != nil {
			sysErr = fmt.Errorf("mcp: read uds peer pid: %w", err)
			return
		}
		exe, err := darwinProcPath(pid)
		if err != nil {
			sysErr = err
			return
		}
		peer = PeerInfo{
			PID:            pid,
			UID:            int(xucred.Uid),
			GID:            int(xucred.Groups[0]),
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

func darwinProcPath(pid int) (string, error) {
	if pid <= 0 {
		return "", errors.New("mcp: peer pid is required")
	}
	buf := C.malloc(C.PROC_PIDPATHINFO_MAXSIZE)
	if buf == nil {
		return "", errors.New("mcp: allocate peer path buffer")
	}
	defer C.free(buf)
	size := C.proc_pidpath(C.int(pid), buf, C.uint32_t(C.PROC_PIDPATHINFO_MAXSIZE))
	if size <= 0 {
		return "", fmt.Errorf("mcp: resolve executable for peer pid %d", pid)
	}
	return C.GoString((*C.char)(buf)), nil
}
