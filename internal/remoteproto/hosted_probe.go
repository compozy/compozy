package remoteproto

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
)

const hostedProbeDNSResolver = "one.one.one.one:53"

type hostedProbeTransport struct {
	system http.RoundTripper
	public http.RoundTripper
}

// NewHostedProbeTransport supplies a public DNS fallback for the host's HTTPS readiness probe.
func NewHostedProbeTransport() http.RoundTripper {
	system := &http.Transport{
		Proxy: http.ProxyFromEnvironment, DisableKeepAlives: true, ForceAttemptHTTP2: true,
	}
	public := system.Clone()
	resolverDialer := &net.Dialer{}
	publicDialer := &net.Dialer{Resolver: &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			// TCP lets the dialer try another address family if the first connection fails.
			return resolverDialer.DialContext(ctx, "tcp", hostedProbeDNSResolver)
		},
	}}
	public.DialContext = publicDialer.DialContext
	return hostedProbeTransport{system: system, public: public}
}

func (t hostedProbeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.system.RoundTrip(req)
	if missingProbeAddress(err, req.URL.Hostname()) {
		return t.public.RoundTrip(req)
	}
	return resp, err
}

func missingProbeAddress(err error, hostname string) bool {
	dnsErr, ok := errors.AsType[*net.DNSError](err)
	return ok && dnsErr.IsNotFound && strings.EqualFold(strings.TrimSuffix(dnsErr.Name, "."), hostname)
}
