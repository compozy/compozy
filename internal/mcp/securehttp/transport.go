package securehttp

import (
	"context"
	"net"
	"net/http"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

type secureDialer struct {
	policy   policy
	resolver ipResolver
	dialer   networkDialer
}

func (d secureDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return outboundpolicy.NewDialer(d.policy.shared(), d.resolver, d.dialer).DialContext(ctx, network, address)
}

type secureTransport struct {
	base            http.RoundTripper
	policy          policy
	maxResponseSize int64
}

var _ http.RoundTripper = secureTransport{}

func (t secureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request == nil {
		return nil, ErrInvalidURL
	}
	if err := t.policy.validateURL(request.URL); err != nil {
		return nil, err
	}
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	if response.Body != nil {
		response.Body = &limitedReadCloser{ReadCloser: response.Body, remaining: t.maxResponseSize}
	}
	return response, nil
}
