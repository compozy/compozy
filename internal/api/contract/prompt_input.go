package contract

import (
	"errors"
	"slices"
	"strings"
)

// PromptInput is the canonical authored prompt extracted from one transport request.
type PromptInput struct {
	Message        string
	MessageID      string
	IdempotencyKey string
}

// ExtractPromptInput validates explicit command identity and AI SDK message correlation.
func ExtractPromptInput(req SendPromptRequest) (PromptInput, error) {
	if err := req.ValidateBusyInputFence(); err != nil {
		return PromptInput{}, err
	}
	messageID := strings.TrimSpace(req.MessageID)
	if messageID == "" {
		return PromptInput{}, errors.New("message_id is required")
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		return PromptInput{}, errors.New("idempotency_key is required")
	}
	topLevelMessage := strings.TrimSpace(req.Message)
	if len(req.Messages) == 0 {
		if topLevelMessage == "" {
			return PromptInput{}, errors.New("message is required")
		}
		return PromptInput{Message: topLevelMessage, MessageID: messageID, IdempotencyKey: idempotencyKey}, nil
	}

	for _, message := range slices.Backward(req.Messages) {
		if strings.TrimSpace(message.Role) != "user" {
			continue
		}
		if strings.TrimSpace(message.ID) != messageID {
			return PromptInput{}, errors.New("latest user message id must equal message_id")
		}
		authored := promptUIMessageText(message)
		if authored == "" {
			return PromptInput{}, errors.New("message is required")
		}
		if topLevelMessage != "" && topLevelMessage != authored {
			return PromptInput{}, errors.New("message must equal the latest user message text")
		}
		return PromptInput{Message: authored, MessageID: messageID, IdempotencyKey: idempotencyKey}, nil
	}
	return PromptInput{}, errors.New("latest user message is required")
}

// ValidateBusyInputFence requires stale-action protection for commands that replace an active turn.
func (r SendPromptRequest) ValidateBusyInputFence() error {
	switch r.Mode {
	case PromptModeSteer, PromptModeInterrupt:
		if strings.TrimSpace(r.ExpectedTurnID) == "" {
			return errors.New("expected_turn_id is required")
		}
	}
	return nil
}

func promptUIMessageText(message PromptUIMessage) string {
	if content := strings.TrimSpace(message.Content); content != "" {
		return content
	}
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		partType := strings.TrimSpace(part.Type)
		if partType != "" && !strings.EqualFold(partType, "text") {
			continue
		}
		if text := strings.TrimSpace(part.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

// Validate checks the required identity and authored text for steering input.
func (r SteerPromptRequest) Validate() error {
	if strings.TrimSpace(r.Text) == "" {
		return errors.New("text is required")
	}
	if strings.TrimSpace(r.MessageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(r.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	if strings.TrimSpace(r.ExpectedTurnID) == "" {
		return errors.New("expected_turn_id is required")
	}
	return nil
}

// Validate checks one atomic queued-input replacement.
func (r ReplaceSessionInputRequest) Validate() error {
	return validateSessionInputMutation(r.Text, r.MessageID, r.IdempotencyKey)
}

// Validate checks one atomic queue-to-steer replacement and its active-turn fence.
func (r PromoteSessionInputRequest) Validate() error {
	if err := validateSessionInputMutation(r.Text, r.MessageID, r.IdempotencyKey); err != nil {
		return err
	}
	if strings.TrimSpace(r.ExpectedTurnID) == "" {
		return errors.New("expected_turn_id is required")
	}
	return nil
}

func validateSessionInputMutation(text string, messageID string, idempotencyKey string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("text is required")
	}
	if strings.TrimSpace(messageID) == "" {
		return errors.New("message_id is required")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}
