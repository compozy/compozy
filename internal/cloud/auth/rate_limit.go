// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"net/http"
	"net/netip"
	"time"
)

type SignInRate string

const (
	DeviceFlowRate       SignInRate = "device_flow"
	ActivationRate       SignInRate = "activation"
	BrowserAuthorizeRate SignInRate = "browser_authorize"
	BillingSignInRate    SignInRate = "billing_sign_in"
)

func (s *Service) LimitSignIn(r *http.Request, scope SignInRate) error {
	window, limit := 10*time.Minute, int32(30)
	if scope == DeviceFlowRate {
		window, limit = time.Hour, 10
	}
	now := s.now()
	expiresAt := now.Truncate(window).Add(window)
	ip := s.signInClientIP(r).Unmap().WithZone("")
	digest := hmac.New(sha256.New, s.signInRateKey)
	digest.Write(ip.AsSlice())
	allowed, err := s.db.ChargeAuthIPWindow(r.Context(), string(scope), digest.Sum(nil), expiresAt, limit)
	if err != nil || allowed {
		return err
	}
	return rateLimited(int((expiresAt.Sub(now) + time.Second - 1) / time.Second))
}

func (s *Service) signInClientIP(r *http.Request) netip.Addr {
	if s.cfg.TrustEdge {
		ip, err := netip.ParseAddr(s.edgeClientIP(r.Header))
		if err == nil {
			return ip
		}
		// Requests without edge metadata share a single bucket across replicas.
		return netip.Addr{}
	}
	peer, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	return peer.Addr()
}
