// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"encoding/base64"
	"errors"

	"github.com/compozy/compozy/internal/cloud/email"
)

func loadResend(getenv func(string) string, environment string) (*email.Resend, []byte, error) {
	apiKey := getenv("COMPOZY_CLOUD_RESEND_API_KEY")
	segment := getenv("COMPOZY_CLOUD_RESEND_NEWS_SEGMENT")
	signingKey := getenv("COMPOZY_CLOUD_UNSUBSCRIBE_KEY")
	if environment == development && apiKey == "" && segment == "" && signingKey == "" {
		return nil, nil, nil
	}
	key, err := base64.RawURLEncoding.Strict().DecodeString(signingKey)
	if err != nil || len(key) != 32 {
		return nil, nil, errors.New("COMPOZY_CLOUD_UNSUBSCRIBE_KEY must be a base64url-encoded 32-byte key")
	}
	var from string
	if environment == development {
		from = getenv("COMPOZY_CLOUD_RESEND_FROM")
	}
	resend, err := email.NewResend(email.ResendConfig{
		APIKey: apiKey, SegmentID: segment, From: from,
	})
	return resend, key, err
}
