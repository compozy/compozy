package network

import "fmt"

// ValidateConversationRef reports whether a reference identifies exactly one container.
func ValidateConversationRef(ref ConversationRef) error {
	return ref.Validate()
}

// ValidateEnvelopeConversation enforces kind-specific container and work fields.
func ValidateEnvelopeConversation(env Envelope) error {
	if env.WorkID != nil {
		if err := ValidateWorkID(*env.WorkID); err != nil {
			return err
		}
	}
	if isDiscoveryKind(env.Kind) {
		return validateDiscoveryEnvelopeOmitsConversation(env)
	}
	if !isConversationKind(env.Kind) {
		return nil
	}
	if _, err := ConversationRefFromEnvelope(env); err != nil {
		return err
	}
	switch env.Kind {
	case KindCapability:
		if env.WorkID == nil {
			return fmt.Errorf("%w: capability work_id is required", ErrMissingField)
		}
	case KindReceipt:
		if env.WorkID == nil {
			return fmt.Errorf("%w: receipt work_id is required", ErrMissingField)
		}
	case KindTrace:
		if env.WorkID == nil {
			return fmt.Errorf("%w: trace work_id is required", ErrMissingField)
		}
	}
	return nil
}

// ConversationRefFromEnvelope returns the validated conversation container.
func ConversationRefFromEnvelope(env Envelope) (ConversationRef, error) {
	if env.Surface == nil {
		if env.ThreadID != nil {
			return ConversationRef{}, fmt.Errorf("%w: thread_id requires surface", ErrInvalidField)
		}
		if env.DirectID != nil {
			return ConversationRef{}, fmt.Errorf("%w: direct_id requires surface", ErrInvalidField)
		}
		return ConversationRef{}, fmt.Errorf("%w: surface is required", ErrMissingField)
	}
	ref := ConversationRef{WorkspaceID: env.WorkspaceID, Channel: env.Channel, Surface: *env.Surface}
	if env.ThreadID != nil {
		ref.ThreadID = *env.ThreadID
	}
	if env.DirectID != nil {
		ref.DirectID = *env.DirectID
	}
	if err := ref.Validate(); err != nil {
		return ConversationRef{}, err
	}
	return ref, nil
}

func validateDiscoveryEnvelopeOmitsConversation(env Envelope) error {
	if env.Surface != nil {
		return fmt.Errorf("%w: %s must not include surface", ErrInvalidField, env.Kind)
	}
	if env.ThreadID != nil {
		return fmt.Errorf("%w: %s must not include thread_id", ErrInvalidField, env.Kind)
	}
	if env.DirectID != nil {
		return fmt.Errorf("%w: %s must not include direct_id", ErrInvalidField, env.Kind)
	}
	if env.WorkID != nil {
		return fmt.Errorf("%w: %s must not include work_id", ErrInvalidField, env.Kind)
	}
	return nil
}

func isDiscoveryKind(kind Kind) bool {
	return kind == KindGreet || kind == KindWhois
}

func isConversationKind(kind Kind) bool {
	switch kind {
	case KindSay, KindCapability, KindReceipt, KindTrace:
		return true
	default:
		return false
	}
}
