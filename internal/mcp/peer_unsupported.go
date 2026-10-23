//go:build !darwin && !linux && !windows

package mcp

import "net"

func peerInfoFromUnixConn(*net.UnixConn) (PeerInfo, error) {
	return PeerInfo{}, ErrPeerCredentialsUnsupported
}
