package remoteproto

import (
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync/atomic"
)

const descriptorBodyLimit = 64 << 10

const HostedTLSFailureMessage = "Hosted route TLS handshake failed. Check the edge certificate."

var (
	ErrDescriptorUnavailable  = errors.New("computer descriptor is unavailable")
	ErrDescriptorPlanRequired = errors.New("computer descriptor requires Pro")
	ErrDescriptorTLS          = errors.New("computer descriptor TLS handshake failed")
	ErrDescriptorDNS          = errors.New("computer descriptor hostname has no DNS address")
)

// FetchDescriptor reads the public, unauthenticated preflight from the supplied origin.
// The client must refuse redirects to keep the request bound to that origin.
func FetchDescriptor(ctx context.Context, origin *url.URL, client *http.Client) (Descriptor, error) {
	var tlsFailed atomic.Bool
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) { tlsFailed.Store(err != nil) },
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin.JoinPath(DescriptorPath).String(), http.NoBody)
	if err != nil {
		return Descriptor{}, ErrDescriptorUnavailable
	}
	req.Header.Set("Cache-Control", "no-cache")
	// #nosec G704 -- This preflight sends no credentials and is bound to the specified computer's origin.
	resp, err := client.Do(req)
	if err != nil {
		if tlsFailed.Load() {
			return Descriptor{}, ErrDescriptorTLS
		}
		if missingProbeAddress(err, origin.Hostname()) {
			return Descriptor{}, ErrDescriptorDNS
		}
		return Descriptor{}, ErrDescriptorUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPaymentRequired {
		var failure struct {
			Code string `json:"code"`
		}
		if json.UnmarshalRead(io.LimitReader(resp.Body, descriptorBodyLimit), &failure) == nil &&
			failure.Code == "plan_required" {
			return Descriptor{}, ErrDescriptorPlanRequired
		}
	}
	var descriptor Descriptor
	if resp.StatusCode != http.StatusOK ||
		json.UnmarshalRead(io.LimitReader(resp.Body, descriptorBodyLimit), &descriptor) != nil {
		return Descriptor{}, ErrDescriptorUnavailable
	}
	return descriptor, nil
}
