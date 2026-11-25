// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/cloud/email"
)

func loadResend(getenv func(string) string, environment string) (*email.Resend, []byte, error) {
	apiKey := getenv("COMPOZY_CLOUD_RESEND_API_KEY")
	segment := getenv("COMPOZY_CLOUD_RESEND_NEWS_SEGMENT")
	contactsKey := getenv("COMPOZY_CLOUD_RESEND_CONTACTS_API_KEY")
	postal := strings.TrimSpace(getenv("COMPOZY_CLOUD_MARKETING_POSTAL_ADDRESS"))
	signingKey := getenv("COMPOZY_CLOUD_UNSUBSCRIBE_KEY")
	if environment == development && apiKey == "" && segment == "" && signingKey == "" && contactsKey == "" &&
		postal == "" {
		return nil, nil, nil
	}
	key, err := base64.RawURLEncoding.Strict().DecodeString(signingKey)
	if err != nil || len(key) != 32 {
		return nil, nil, errors.New("COMPOZY_CLOUD_UNSUBSCRIBE_KEY must be a base64url-encoded 32-byte key")
	}
	if segment != "" && postal == "" {
		return nil, nil, errors.New("COMPOZY_CLOUD_MARKETING_POSTAL_ADDRESS is required for product news")
	}
	var from string
	if environment == development {
		from = getenv("COMPOZY_CLOUD_RESEND_FROM")
	}
	resend, err := email.NewResend(email.ResendConfig{
		APIKey: apiKey, ContactAPIKey: contactsKey, SegmentID: segment,
		From: from, PostalAddress: postal,
	})
	return resend, key, err
}
