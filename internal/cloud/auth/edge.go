// SPDX-License-Identifier: FSL-1.1-MIT

package auth

import (
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	ActivateAction = "activate"
	BrowserAction  = "browser_authorize"
)

func (s *Service) RequestLocation(headers http.Header) string {
	const maxLocation = 160
	if !s.cfg.TrustEdge {
		return ""
	}
	raw := headers.Get("X-Compozy-Location")
	location, err := url.PathUnescape(raw)
	if err != nil || len(location) > maxLocation || !utf8.ValidString(location) {
		return ""
	}
	parts := slices.Collect(strings.SplitSeq(location, ","))
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
		if _, err := netip.ParseAddr(parts[i]); err == nil {
			return ""
		}
		if strings.ContainsFunc(part, unicode.IsControl) {
			return ""
		}
	}
	if len(parts) > 2 {
		return parts[0] + ", " + parts[len(parts)-1]
	}
	return strings.Join(parts, ", ")
}

func (s *Service) VerifyBrowser(r *http.Request, token, action string) error {
	var clientIP string
	if s.cfg.TrustEdge {
		clientIP = r.Header.Get("X-Compozy-Client-IP")
	}
	if err := s.cfg.Turnstile.Verify(r.Context(), token, clientIP, action); err != nil {
		return ErrTurnstile
	}
	return nil
}
