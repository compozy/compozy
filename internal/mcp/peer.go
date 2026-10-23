package mcp

import (
	"context"
	"errors"
	"net"
)

var ErrPeerCredentialsUnsupported = errors.New("mcp: peer credential validation is unsupported")

var ErrWindowsPeerCredentialsUnsupported = errors.New(
	"mcp: Windows AF_UNIX peer identity requires SIO_AF_UNIX_GETPEERPID; update Windows to a build supporting this ioctl",
)

// PeerInfo is the UDS peer identity captured from the accepted socket.
type PeerInfo struct {
	PID            int    `json:"pid"`
	UID            int    `json:"uid"`
	GID            int    `json:"gid"`
	UserSID        string `json:"user_sid,omitempty"`
	ExecutablePath string `json:"executable_path"`
	Supported      bool   `json:"supported"`
}

type peerInfoContextKey struct{}
type peerInfoErrorContextKey struct{}

// ContextWithPeerInfo annotates an accepted UDS connection context.
func ContextWithPeerInfo(ctx context.Context, peer PeerInfo, err error) context.Context {
	ctx = context.WithValue(ctx, peerInfoContextKey{}, peer)
	if err != nil {
		ctx = context.WithValue(ctx, peerInfoErrorContextKey{}, err)
	}
	return ctx
}

// PeerInfoFromContext extracts UDS peer identity from a request context.
func PeerInfoFromContext(ctx context.Context) (PeerInfo, error) {
	if ctx == nil {
		return PeerInfo{}, errors.New("mcp: context is required")
	}
	if err, ok := ctx.Value(peerInfoErrorContextKey{}).(error); ok && err != nil {
		return PeerInfo{}, err
	}
	peer, ok := ctx.Value(peerInfoContextKey{}).(PeerInfo)
	if !ok {
		return PeerInfo{}, ErrPeerCredentialsUnsupported
	}
	return peer, nil
}

// PeerInfoFromConn returns OS peer credentials and executable identity for an accepted UDS connection.
func PeerInfoFromConn(conn net.Conn) (PeerInfo, error) {
	if conn == nil {
		return PeerInfo{}, errors.New("mcp: connection is required")
	}
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return PeerInfo{}, ErrPeerCredentialsUnsupported
	}
	return peerInfoFromUnixConn(unixConn)
}
