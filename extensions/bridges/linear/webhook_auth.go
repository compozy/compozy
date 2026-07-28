package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func validLinearWebhookSignature(secret string, body []byte, signature string) bool {
	expected := linearSignature(secret, body)
	return expected != "" && hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func linearSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	if written, err := mac.Write(body); err != nil || written != len(body) {
		return ""
	}
	return hex.EncodeToString(mac.Sum(nil))
}
