//go:build !darwin && !linux

package mcp

import "net"

func peerInfoFromUnixConn(*net.UnixConn) (PeerInfo, error) {
	return PeerInfo{}, ErrPeerCredentialsUnsupported
}
