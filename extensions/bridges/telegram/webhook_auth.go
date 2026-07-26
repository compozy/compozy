package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
)

func verifyWebhookSecret(_ context.Context, req *http.Request, _ []byte, secret string) error {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return errors.New("telegram: webhook secret is required")
	}
	if req == nil {
		return errors.New("telegram: webhook request is required")
	}
	header := strings.TrimSpace(req.Header.Get("X-Telegram-Bot-Api-Secret-Token"))
	headerDigest := sha256.Sum256([]byte(header))
	secretDigest := sha256.Sum256([]byte(trimmedSecret))
	if header == "" || subtle.ConstantTimeCompare(headerDigest[:], secretDigest[:]) != 1 {
		return errors.New("telegram: invalid webhook secret")
	}
	return nil
}
