//go:build darwin && !cgo

package mcp

import "net"

func peerInfoFromUnixConn(*net.UnixConn) (PeerInfo, error) {
	return PeerInfo{}, ErrPeerCredentialsUnsupported
}
