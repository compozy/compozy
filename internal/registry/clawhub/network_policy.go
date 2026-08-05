package clawhub

import (
	"fmt"
	"net/url"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

func newClawHubNetworkPolicy(baseURL string) (outboundpolicy.Policy, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return outboundpolicy.Policy{}, fmt.Errorf("clawhub: parse base URL: %w", err)
	}
	defaultBase, err := url.Parse(defaultBaseURL)
	if err != nil {
		return outboundpolicy.Policy{}, fmt.Errorf("clawhub: parse default base URL: %w", err)
	}
	policy := outboundpolicy.New(false)
	if outboundpolicy.SameOrigin(base, defaultBase) {
		return policy, nil
	}
	policy, err = policy.WithTrustedOrigin(baseURL)
	if err != nil {
		return outboundpolicy.Policy{}, fmt.Errorf("clawhub: invalid base URL: %w", err)
	}
	return policy, nil
}
