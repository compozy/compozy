package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func verifyGitHubWebhookSignature(
	_ context.Context,
	req *http.Request,
	body []byte,
	candidates []resolvedInstanceConfig,
) error {
	if req == nil {
		return errors.New("github: webhook request is required")
	}
	signature := strings.TrimSpace(req.Header.Get("X-Hub-Signature-256"))
	if signature == "" {
		return errors.New("github: missing webhook signature")
	}
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "sha256") {
		return errors.New("github: invalid webhook signature format")
	}
	expected := strings.ToLower(strings.TrimSpace(parts[1]))
	if expected == "" {
		return errors.New("github: invalid webhook signature")
	}

	seen := make(map[string]struct{}, len(candidates))
	for idx := range candidates {
		secret := strings.TrimSpace(candidates[idx].webhookSecret)
		if secret == "" {
			continue
		}
		if _, ok := seen[secret]; ok {
			continue
		}
		seen[secret] = struct{}{}
		mac := hmac.New(sha256.New, []byte(secret))
		if _, err := mac.Write(body); err != nil {
			return fmt.Errorf("github: hash request body: %w", err)
		}
		if hmac.Equal([]byte(expected), []byte(hex.EncodeToString(mac.Sum(nil)))) {
			return nil
		}
	}
	return errors.New("github: invalid webhook signature")
}
