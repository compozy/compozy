package securehttp

import (
	"net/url"

	"github.com/compozy/compozy/internal/outboundpolicy"
)

type policy struct {
	allowLoopback bool
}

func (p policy) validateURL(destination *url.URL) error {
	return p.shared().ValidateURL(destination)
}

func sameOrigin(left, right *url.URL) bool {
	return outboundpolicy.SameOrigin(left, right)
}

func (p policy) shared() outboundpolicy.Policy {
	return outboundpolicy.New(p.allowLoopback)
}
