package main

import (
	"errors"

	"html/template"
	"io"
	"net/http"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func resolveDeliveryTarget(event bridgepkg.DeliveryEvent) (string, error) {
	userID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.RoutingKey.PeerID),
	)
	if userID == "" {
		return "", errors.New("whatsapp: delivery target requires peer_id")
	}
	return userID, nil
}

func writeWhatsAppWebhookOK(w http.ResponseWriter) error {
	if w == nil {
		return nil
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, "ok")
	return err
}

func writeWhatsAppVerifyChallenge(w http.ResponseWriter, challenge whatsappVerifyChallenge) error {
	if w == nil {
		return nil
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, template.HTMLEscapeString(string(challenge)))
	return err
}

func parseWhatsAppVerifyChallenge(value string) (whatsappVerifyChallenge, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("whatsapp: hub.challenge is required")
	}
	if len(trimmed) > 256 {
		return "", errors.New("whatsapp: hub.challenge exceeds 256 characters")
	}
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.', r == '-', r == '_':
		default:
			return "", errors.New("whatsapp: hub.challenge contains invalid characters")
		}
	}
	return whatsappVerifyChallenge(trimmed), nil
}
