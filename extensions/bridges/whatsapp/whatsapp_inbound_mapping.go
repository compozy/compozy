package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"

	"github.com/compozy/agh/internal/subprocess"
)

func allowWhatsAppDirectMessage(cfg resolvedInstanceConfig, sender bridgepkg.MessageSender) bool {
	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return identityAllowed(cfg.allowUserIDs, cfg.allowUsernames, sender)
	case bridgepkg.BridgeDMPolicyPairing:
		if identityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, sender) {
			return true
		}
		return identityAllowed(cfg.allowUserIDs, cfg.allowUsernames, sender)
	default:
		return false
	}
}

func identityAllowed(
	ids map[string]struct{},
	usernames map[string]struct{},
	sender bridgepkg.MessageSender,
) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[strings.TrimSpace(sender.ID)]; ok {
		return true
	}
	if _, ok := usernames[normalizeUsername(firstNonEmpty(sender.Username, sender.DisplayName))]; ok {
		return true
	}
	return false
}

func mapWhatsAppInboundMessage(
	message whatsappInboundMessage,
	contact *whatsappContact,
	managed *subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	phoneNumberID string,
) (bridgepkg.InboundMessageEnvelope, error) {
	if managed == nil {
		return bridgepkg.InboundMessageEnvelope{}, errors.New(
			"whatsapp: managed bridge instance is required",
		)
	}
	if strings.TrimSpace(message.ID) == "" {
		return bridgepkg.InboundMessageEnvelope{}, errors.New(
			"whatsapp: inbound message id is required",
		)
	}
	if strings.TrimSpace(message.From) == "" {
		return bridgepkg.InboundMessageEnvelope{}, errors.New(
			"whatsapp: inbound sender id is required",
		)
	}
	text := extractWhatsAppTextContent(message)
	if text == "" {
		return bridgepkg.InboundMessageEnvelope{}, errors.New(
			"whatsapp: unsupported inbound message type",
		)
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	if parsed := parseUnixTimestamp(message.Timestamp); !parsed.IsZero() {
		receivedAt = parsed
	}

	senderID, senderName, username := resolveWhatsAppSender(message, contact)

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		PlatformMessageID: strings.TrimSpace(message.ID),
		ReceivedAt:        receivedAt,
		PeerID:            senderID,
		Sender: bridgepkg.MessageSender{
			ID:          senderID,
			Username:    username,
			DisplayName: senderName,
		},
		Content: bridgepkg.MessageContent{
			Text: text,
		},
		Attachments: normalizeWhatsAppAttachments(message),
		EventFamily: bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: fmt.Sprintf(
			"whatsapp:%s:%s",
			managed.Instance.ID,
			strings.TrimSpace(message.ID),
		),
	}

	metadata, err := buildWhatsAppInboundMetadata(message, phoneNumberID)
	if err == nil {
		envelope.ProviderMetadata = metadata
	}

	return envelope, envelope.Validate()
}

func resolveWhatsAppSender(
	message whatsappInboundMessage,
	contact *whatsappContact,
) (string, string, string) {
	senderID := strings.TrimSpace(message.From)
	senderName := senderID
	username := ""
	if contact != nil && strings.TrimSpace(contact.Profile.Name) != "" {
		senderName = strings.TrimSpace(contact.Profile.Name)
		username = normalizeUsername(contact.Profile.Name)
	}
	return senderID, senderName, username
}

func buildWhatsAppInboundMetadata(
	message whatsappInboundMessage,
	phoneNumberID string,
) ([]byte, error) {
	contextFrom, contextID := extractWhatsAppContext(message.Context)
	return json.Marshal(map[string]any{
		"context_from":    contextFrom,
		"context_id":      contextID,
		"message_id":      strings.TrimSpace(message.ID),
		"phone_number_id": strings.TrimSpace(phoneNumberID),
		"sender_wa_id":    strings.TrimSpace(message.From),
		"timestamp":       strings.TrimSpace(message.Timestamp),
		"type":            strings.TrimSpace(message.Type),
	})
}

func extractWhatsAppContext(contextRef *struct {
	From string `json:"from,omitempty"`
	ID   string `json:"id,omitempty"`
}) (string, string) {
	if contextRef == nil {
		return "", ""
	}
	return strings.TrimSpace(contextRef.From), strings.TrimSpace(contextRef.ID)
}

func normalizeWhatsAppAttachments(message whatsappInboundMessage) []bridgepkg.MessageAttachment {
	attachments := make([]bridgepkg.MessageAttachment, 0, 4)
	appendAttachment := func(id string, name string, mimeType string, url string) {
		attachment := bridgepkg.MessageAttachment{
			ID:       strings.TrimSpace(id),
			Name:     strings.TrimSpace(name),
			MIMEType: strings.TrimSpace(mimeType),
			URL:      strings.TrimSpace(url),
		}
		if attachment.ID == "" && attachment.Name == "" && attachment.URL == "" {
			return
		}
		attachments = append(attachments, attachment)
	}

	if message.Image != nil {
		appendAttachment(message.Image.ID, providerImageKey, message.Image.MIMEType, "")
	}
	if message.Document != nil {
		appendAttachment(
			message.Document.ID,
			firstNonEmpty(message.Document.Filename, "document"),
			message.Document.MIMEType,
			"",
		)
	}
	if message.Audio != nil {
		appendAttachment(message.Audio.ID, providerAudioKey, message.Audio.MIMEType, "")
	}
	if message.Video != nil {
		appendAttachment(message.Video.ID, "video", message.Video.MIMEType, "")
	}
	if message.Sticker != nil {
		appendAttachment(message.Sticker.ID, providerStickerKey, message.Sticker.MIMEType, "")
	}
	if message.Location != nil {
		name := firstNonEmpty(message.Location.Name, "location")
		url := strings.TrimSpace(message.Location.URL)
		if url == "" {
			url = fmt.Sprintf(
				"https://www.google.com/maps?q=%v,%v",
				message.Location.Latitude,
				message.Location.Longitude,
			)
		}
		appendAttachment("", name, "application/geo+json", url)
	}

	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func extractWhatsAppTextContent(message whatsappInboundMessage) string {
	switch strings.TrimSpace(message.Type) {
	case providerTextKey:
		if message.Text == nil {
			return ""
		}
		return strings.TrimSpace(message.Text.Body)
	case providerImageKey:
		if message.Image == nil {
			return ""
		}
		return firstNonEmpty(strings.TrimSpace(message.Image.Caption), "[Image]")
	case "document":
		if message.Document == nil {
			return ""
		}
		if caption := strings.TrimSpace(message.Document.Caption); caption != "" {
			return caption
		}
		return fmt.Sprintf("[Document: %s]", firstNonEmpty(message.Document.Filename, "file"))
	case providerAudioKey:
		return "[Audio message]"
	case "video":
		if message.Video == nil {
			return "[Video]"
		}
		return firstNonEmpty(strings.TrimSpace(message.Video.Caption), "[Video]")
	case providerStickerKey:
		return "[Sticker]"
	case "location":
		if message.Location == nil {
			return "[Location]"
		}
		parts := []string{
			firstNonEmpty(
				message.Location.Name,
				fmt.Sprintf("%v,%v", message.Location.Latitude, message.Location.Longitude),
			),
		}
		if address := strings.TrimSpace(message.Location.Address); address != "" {
			parts = append(parts, address)
		}
		return "[Location: " + strings.Join(parts, " - ") + "]"
	default:
		return ""
	}
}
