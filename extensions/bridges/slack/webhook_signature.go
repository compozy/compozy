package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func verifySlackSignature(_ context.Context, req *http.Request, body []byte, secret string, now time.Time) error {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return errors.New("slack: signing secret is required")
	}
	if req == nil {
		return errors.New("slack: webhook request is required")
	}

	timestamp := strings.TrimSpace(req.Header.Get("X-Slack-Request-Timestamp"))
	signature := strings.TrimSpace(req.Header.Get("X-Slack-Signature"))
	if timestamp == "" || signature == "" {
		return errors.New("slack: missing request signature headers")
	}
	tsValue, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return errors.New("slack: invalid request timestamp")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if delta := now.Unix() - tsValue; delta > 300 || delta < -300 {
		return errors.New("slack: stale request timestamp")
	}

	mac := hmac.New(sha256.New, []byte(trimmedSecret))
	if _, err := mac.Write([]byte(slackSignatureVersion + ":" + timestamp + ":")); err != nil {
		return fmt.Errorf("slack: hash signature prefix: %w", err)
	}
	if _, err := mac.Write(body); err != nil {
		return fmt.Errorf("slack: hash request body: %w", err)
	}
	expected := slackSignatureVersion + "=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("slack: invalid request signature")
	}
	return nil
}
