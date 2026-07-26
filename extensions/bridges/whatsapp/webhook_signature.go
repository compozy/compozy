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

func verifyWhatsAppSignature(
	_ context.Context,
	req *http.Request,
	body []byte,
	appSecret string,
) error {
	secret := strings.TrimSpace(appSecret)
	if secret == "" {
		return errors.New("whatsapp: app secret is required")
	}
	if req == nil {
		return errors.New("whatsapp: webhook request is required")
	}

	signature := strings.TrimSpace(req.Header.Get(whatsappSignatureHeader))
	if signature == "" {
		return errors.New("whatsapp: missing webhook signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return fmt.Errorf("whatsapp: hash request body: %w", err)
	}
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return errors.New("whatsapp: invalid webhook signature")
	}
	return nil
}
