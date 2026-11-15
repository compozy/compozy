package githubauth

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

func Authorize(ctx context.Context, request *http.Request, origin string, resolver *Resolver) error {
	if resolver == nil || !IsAPIOrigin(request.URL, origin) {
		return nil
	}
	token, err := resolver.Resolve(ctx)
	if err != nil {
		return err
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return nil
}

func HTTPClient(base *http.Client, origin string) *http.Client {
	client := *base
	client.Jar = nil
	if client.Timeout <= 0 {
		client.Timeout = 30 * time.Second
	}
	prior := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if prior != nil {
			if err := prior(next, previous); err != nil {
				return err
			}
		}
		if len(previous) >= 10 {
			return http.ErrUseLastResponse
		}
		if !IsAPIOrigin(next.URL, origin) {
			next.Header.Del("Authorization")
		}
		return nil
	}
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = credentialTransport{base: transport, origin: origin}
	return &client
}

type credentialTransport struct {
	base   http.RoundTripper
	origin string
}

var _ http.RoundTripper = credentialTransport{}

func (t credentialTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if !IsAPIOrigin(request.URL, t.origin) {
		request = request.Clone(request.Context())
		request.Header.Del("Authorization")
	}
	return t.base.RoundTrip(request)
}

func IsAPIOrigin(destination *url.URL, origin string) bool {
	configured, err := url.Parse(origin)
	return err == nil && destination.Scheme == "https" && outboundpolicy.SameOrigin(destination, configured)
}

func RedactResponse(response *http.Response, message string) string {
	return redactRequest(response.Request, message)
}

func redactRequest(request *http.Request, message string) string {
	if request != nil {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if token != "" {
			return strings.ReplaceAll(message, token, "[REDACTED]")
		}
	}
	return message
}

func RedactError(request *http.Request, err error) error {
	if err == nil {
		return nil
	}
	message := redactRequest(request, err.Error())
	if message == err.Error() {
		return err
	}
	return &redactedError{message: message, cause: err}
}

type redactedError struct {
	message string
	cause   error
}

func (e *redactedError) Error() string { return e.message }
func (e *redactedError) Unwrap() error { return e.cause }
