package config

import (
	"net"
	"net/netip"
	"net/url"
	"slices"
	"strconv"

	"github.com/compozy/compozy/internal/remoteproto"
)

const (
	DefaultRemoteListener = "127.0.0.1:47823"
	DefaultHostedListener = "127.0.0.1:47824"
)

type RemoteConfig struct {
	Enabled               bool                   `toml:"enabled"`
	Listener              string                 `toml:"listener"`
	HostedListener        string                 `toml:"hosted_listener"`
	RequireDeviceApproval bool                   `toml:"require_device_approval"`
	MaxScopes             []remoteproto.Scope    `toml:"max_scopes"`
	Direct                RemoteDirectConfig     `toml:"direct"`
	CloudRoute            RemoteCloudRouteConfig `toml:"cloud_route"`
	Ingress               RemoteIngressConfig    `toml:"ingress"`
}

type RemoteDirectConfig struct {
	Enabled   bool     `toml:"enabled"`
	Addresses []string `toml:"addresses"`
}

type RemoteCloudRouteConfig struct {
	Enabled bool `toml:"enabled"`
}

type RemoteIngressConfig struct {
	PublicURL string `toml:"public_url"`
}

func DefaultRemoteConfig() RemoteConfig {
	return RemoteConfig{Listener: DefaultRemoteListener, HostedListener: DefaultHostedListener}
}

func (r RemoteConfig) Clone() RemoteConfig {
	r.MaxScopes = slices.Clone(r.MaxScopes)
	r.Direct.Addresses = slices.Clone(r.Direct.Addresses)
	return r
}

func (r RemoteConfig) Validate() error {
	if r.Listener == "" && r.HostedListener == "" && !r.Enabled {
		return nil
	}
	tunnel, err := netip.ParseAddrPort(r.Listener)
	if err != nil || !tunnel.Addr().Unmap().IsLoopback() {
		return ValidationError{Path: "remote.listener", Message: "must be a loopback IP address and port"}
	}
	hosted, err := netip.ParseAddrPort(r.HostedListener)
	if err != nil || !hosted.Addr().Unmap().IsLoopback() || netip.AddrPortFrom(hosted.Addr().Unmap(), hosted.Port()) ==
		netip.AddrPortFrom(tunnel.Addr().Unmap(), tunnel.Port()) {
		return ValidationError{
			Path:    "remote.hosted_listener",
			Message: "must be a distinct loopback IP address and port",
		}
	}
	for _, addr := range r.Direct.Addresses {
		if _, ok := r.DirectAddress(addr); !ok {
			return ValidationError{
				Path:    "remote.direct.addresses",
				Message: "must contain private or tailnet IP addresses",
			}
		}
	}
	if slices.ContainsFunc(r.MaxScopes, func(scope remoteproto.Scope) bool { return !scope.Valid() }) {
		return ValidationError{Path: "remote.max_scopes", Message: "contains an unknown scope"}
	}
	if r.Ingress.PublicURL != "" {
		u, err := url.Parse(r.Ingress.PublicURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" ||
			u.Fragment != "" {
			return ValidationError{Path: "remote.ingress.public_url", Message: "must be an HTTPS URL"}
		}
	}
	return nil
}

func (r RemoteConfig) DirectAddress(input string) (string, bool) {
	host, port, err := net.SplitHostPort(input)
	if err != nil {
		host = input
		_, port, err = net.SplitHostPort(r.Listener)
		if err != nil {
			return "", false
		}
	}
	parsed, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}
	addr := parsed.Unmap()
	number, err := strconv.ParseUint(port, 10, 16)
	if err != nil || (!addr.IsPrivate() && !tailnetIPv4.Contains(addr) && !tailnetIPv6.Contains(addr)) {
		return "", false
	}
	return netip.AddrPortFrom(addr, uint16(number)).String(), true
}

var (
	tailnetIPv4 = netip.MustParsePrefix("100.64.0.0/10")
	tailnetIPv6 = netip.MustParsePrefix("fd7a:115c:a1e0::/48")
)

type remoteOverlay struct {
	Enabled               *bool                `toml:"enabled"`
	Listener              *string              `toml:"listener"`
	HostedListener        *string              `toml:"hosted_listener"`
	RequireDeviceApproval *bool                `toml:"require_device_approval"`
	MaxScopes             *[]remoteproto.Scope `toml:"max_scopes"`
	Direct                struct {
		Enabled   *bool     `toml:"enabled"`
		Addresses *[]string `toml:"addresses"`
	} `toml:"direct"`
	CloudRoute struct {
		Enabled *bool `toml:"enabled"`
	} `toml:"cloud_route"`
	Ingress struct {
		PublicURL *string `toml:"public_url"`
	} `toml:"ingress"`
}

func (o remoteOverlay) Apply(dst *RemoteConfig) {
	applyOptional(o.Enabled, &dst.Enabled)
	applyOptional(o.Listener, &dst.Listener)
	applyOptional(o.HostedListener, &dst.HostedListener)
	applyOptional(o.RequireDeviceApproval, &dst.RequireDeviceApproval)
	applyOptional(o.MaxScopes, &dst.MaxScopes)
	applyOptional(o.Direct.Enabled, &dst.Direct.Enabled)
	applyOptional(o.Direct.Addresses, &dst.Direct.Addresses)
	applyOptional(o.CloudRoute.Enabled, &dst.CloudRoute.Enabled)
	applyOptional(o.Ingress.PublicURL, &dst.Ingress.PublicURL)
}
