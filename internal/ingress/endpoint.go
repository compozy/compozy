package ingress

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

var (
	ErrUnavailable  = errors.New("webhook_ingress_unavailable")
	ErrNotConfirmed = errors.New("webhook_ingress_not_confirmed")
)

type RemoteEndpoint struct {
	Base      string
	Managed   bool
	Reachable bool
	Pending   bool
}

func AdvertisedBase(hostname, publicURL string) (string, error) {
	base := strings.TrimSpace(publicURL)
	if hostname != "" {
		base = "https://" + hostname
	}
	origin, err := url.Parse(base)
	if err != nil || origin.Scheme != "https" || origin.Hostname() == "" ||
		origin.User != nil || origin.RawQuery != "" || origin.ForceQuery || origin.Fragment != "" {
		return "", ErrUnavailable
	}
	origin.Path = strings.TrimRight(origin.Path, "/")
	return origin.String(), nil
}

func (m *Manager) AdmitDelivery(ctx context.Context, base string, ref SubjectRef) error {
	endpoint, err := m.store.GetIngressEndpoint(ctx)
	if errors.Is(err, ErrEndpointUnverified) || (err == nil && (!endpoint.Reachable || endpoint.URL != base)) {
		return ErrUnavailable
	}
	if err != nil {
		return err
	}
	binding, err := m.store.GetIngressBinding(ctx, ref)
	if errors.Is(err, ErrSubjectNotFound) || (err == nil && binding.EndpointGeneration != endpoint.Generation) {
		return ErrNotConfirmed
	}
	return err
}
