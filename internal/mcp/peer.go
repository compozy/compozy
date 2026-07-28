package mcp

import (
	"context"
	"errors"
	"net"
)

var ErrPeerCredentialsUnsupported = errors.New("mcp: peer credential validation is unsupported")

// PeerInfo is the UDS peer identity captured from the accepted socket.
type PeerInfo struct {
	PID            int    `json:"pid"`
	UID            int    `json:"uid"`
	GID            int    `json:"gid"`
	ExecutablePath string `json:"executable_path"`
	Supported      bool   `json:"supported"`
}

type peerInfoContextKey struct{}
type peerInfoErrorContextKey struct{}

// ContextWithPeerInfo annotates an accepted UDS connection context.
func ContextWithPeerInfo(ctx context.Context, peer PeerInfo, err error) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
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
