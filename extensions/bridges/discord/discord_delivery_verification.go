package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"

	"errors"
	"fmt"

	"net/http"

	"strconv"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func resolveDiscordDeliveryChannelID(event bridgepkg.DeliveryEvent) (string, error) {
	channelID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.DeliveryTarget.GroupID),
		strings.TrimSpace(event.RoutingKey.PeerID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
		strings.TrimSpace(event.RoutingKey.GroupID),
	)
	if channelID == "" {
		return "", errors.New("discord: delivery target requires peer_id, thread_id, or group_id")
	}
	return channelID, nil
}

func verifyDiscordSignature(_ context.Context, req *http.Request, body []byte, publicKey string, now time.Time) error {
	decodedPublicKey, err := decodeDiscordPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("discord: invalid public key: %w", err)
	}
	if req == nil {
		return errors.New("discord: webhook request is required")
	}

	timestamp := strings.TrimSpace(req.Header.Get("X-Signature-Timestamp"))
	signature := strings.TrimSpace(req.Header.Get("X-Signature-Ed25519"))
	if timestamp == "" || signature == "" {
		return errors.New("discord: missing signature headers")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if tsValue, err := strconv.ParseInt(timestamp, 10, 64); err == nil {
		if delta := now.Unix() - tsValue; delta > 300 || delta < -300 {
			return errors.New("discord: stale request timestamp")
		}
	}

	decodedSignature, err := hex.DecodeString(signature)
	if err != nil {
		return errors.New("discord: invalid signature encoding")
	}
	if len(decodedSignature) != ed25519.SignatureSize {
		return errors.New("discord: invalid signature length")
	}
	message := make([]byte, 0, len(timestamp)+len(body))
	message = append(message, timestamp...)
	message = append(message, body...)
	if !ed25519.Verify(decodedPublicKey, message, decodedSignature) {
		return errors.New("discord: invalid signature")
	}
	return nil
}

func decodeDiscordPublicKey(value string) (ed25519.PublicKey, error) {
	raw, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d-byte public key, got %d", ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), nil
}
