package network

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

type bodyDecoder func(json.RawMessage) (Body, error)

// DecodeBody parses and validates one kind-specific envelope body.
func DecodeBody(kind Kind, raw json.RawMessage) (Body, error) {
	if _, err := validateJSONObject("body", raw); err != nil {
		return nil, err
	}
	decoder, err := bodyDecoderForKind(kind)
	if err != nil {
		return nil, err
	}
	return decoder(raw)
}

func bodyDecoderForKind(kind Kind) (bodyDecoder, error) {
	switch kind {
	case KindGreet:
		return func(raw json.RawMessage) (Body, error) {
			return decodeNormalizedBody(raw, "greet", normalizeAndValidateGreetBody)
		}, nil
	case KindWhois:
		return func(raw json.RawMessage) (Body, error) {
			return decodeNormalizedBody(raw, "whois", normalizeAndValidateWhoisBody)
		}, nil
	case KindSay:
		return func(raw json.RawMessage) (Body, error) {
			return decodeNormalizedBody(raw, "say", normalizeAndValidateSayBody)
		}, nil
	case KindCapability:
		return decodeCapabilityEnvelopeBody, nil
	case KindReceipt:
		return func(raw json.RawMessage) (Body, error) {
			return decodeNormalizedBody(raw, "receipt", normalizeAndValidateReceiptBody)
		}, nil
	case KindTrace:
		return func(raw json.RawMessage) (Body, error) {
			return decodeNormalizedBody(raw, "trace", normalizeAndValidateTraceBody)
		}, nil
	default:
		return nil, fmt.Errorf("%w: kind=%q", ErrInvalidKind, string(kind))
	}
}

func decodeCapabilityEnvelopeBody(raw json.RawMessage) (Body, error) {
	object, err := validateJSONObject("body", raw)
	if err != nil {
		return nil, err
	}
	if _, ok := object["capability"]; !ok {
		return nil, fmt.Errorf(
			"%w: capability body must wrap artifact fields inside \"capability\", e.g. {\"capability\":{...}}",
			ErrInvalidBody,
		)
	}
	return decodeNormalizedBody(raw, "capability", normalizeAndValidateCapabilityBody)
}

func decodeNormalizedBody[T Body](raw json.RawMessage, label string, normalize func(*T) error) (Body, error) {
	var body T
	if err := decodeJSON(raw, &body); err != nil {
		return nil, fmt.Errorf("%w: %s body: %w", ErrInvalidBody, label, err)
	}
	if err := normalize(&body); err != nil {
		return nil, err
	}
	return body, nil
}

func validateKindEnvelopeRules(env Envelope) error {
	body, err := env.DecodeBody()
	if err != nil {
		return err
	}
	switch typed := body.(type) {
	case GreetBody:
		if typed.PeerCard.PeerID != env.From {
			return fmt.Errorf("%w: greet peer_card.peer_id must match from", ErrInvalidBody)
		}
	case WhoisBody:
		if typed.Type == WhoisTypeResponse {
			if env.ReplyTo == nil {
				return fmt.Errorf("%w: whois response reply_to is required", ErrMissingField)
			}
			if typed.PeerCard == nil {
				return fmt.Errorf("%w: whois response peer_card is required", ErrInvalidBody)
			}
			if typed.PeerCard.PeerID != env.From {
				return fmt.Errorf("%w: whois response peer_card.peer_id must match from", ErrInvalidBody)
			}
		}
	case ReceiptBody:
		if env.WorkID == nil {
			return fmt.Errorf("%w: receipt work_id is required", ErrMissingField)
		}
	case TraceBody:
		if env.WorkID == nil {
			return fmt.Errorf("%w: trace work_id is required", ErrMissingField)
		}
	}
	return nil
}

func normalizeAndValidateGreetBody(body *GreetBody) error {
	return normalizeAndValidatePeerCard(&body.PeerCard)
}

func normalizeAndValidatePeerCard(card *PeerCard) error {
	card.PeerID = strings.TrimSpace(card.PeerID)
	if card.PeerID == "" {
		return fmt.Errorf("%w: peer_card.peer_id is required", ErrInvalidBody)
	}
	if err := ValidatePeerID(card.PeerID); err != nil {
		return fmt.Errorf("%w: peer_card.peer_id", err)
	}
	card.DisplayName = normalizeOptionalText(card.DisplayName)
	card.ProfilesSupported = normalizeStringList(card.ProfilesSupported)
	card.Capabilities = normalizeStringList(card.Capabilities)
	card.ArtifactsSupported = normalizeStringList(card.ArtifactsSupported)
	card.TrustModesSupported = normalizeStringList(card.TrustModesSupported)
	card.Ext = cloneExtensionMap(card.Ext)
	if card.ProfilesSupported == nil {
		return fmt.Errorf("%w: peer_card.profiles_supported is required", ErrInvalidBody)
	}
	if card.Capabilities == nil {
		return fmt.Errorf("%w: peer_card.capabilities is required", ErrInvalidBody)
	}
	if card.ArtifactsSupported == nil {
		return fmt.Errorf("%w: peer_card.artifacts_supported is required", ErrInvalidBody)
	}
	if card.TrustModesSupported == nil {
		return fmt.Errorf("%w: peer_card.trust_modes_supported is required", ErrInvalidBody)
	}
	return nil
}

func normalizeAndValidateWhoisBody(body *WhoisBody) error {
	body.Type = WhoisType(strings.TrimSpace(string(body.Type)))
	if err := body.Type.Validate(); err != nil {
		return err
	}
	if body.Type == WhoisTypeRequest {
		if body.PeerCard != nil {
			return fmt.Errorf("%w: whois request must not include peer_card", ErrInvalidBody)
		}
		return nil
	}
	if body.PeerCard == nil {
		return fmt.Errorf("%w: whois response peer_card is required", ErrInvalidBody)
	}
	return normalizeAndValidatePeerCard(body.PeerCard)
}

func normalizeAndValidateSayBody(body *SayBody) error {
	if strings.TrimSpace(body.Text) == "" {
		return fmt.Errorf("%w: say text is required", ErrInvalidBody)
	}
	body.Intent = strings.TrimSpace(body.Intent)
	return nil
}

func normalizeAndValidateCapabilityBody(body *CapabilityBody) error {
	body.Capability.ID = strings.TrimSpace(body.Capability.ID)
	body.Capability.Summary = strings.TrimSpace(body.Capability.Summary)
	body.Capability.Outcome = strings.TrimSpace(body.Capability.Outcome)
	body.Capability.Version = strings.TrimSpace(body.Capability.Version)
	body.Capability.Digest = strings.TrimSpace(body.Capability.Digest)
	body.Capability.ContextNeeded = normalizeStringList(body.Capability.ContextNeeded)
	body.Capability.ArtifactsExpected = normalizeStringList(body.Capability.ArtifactsExpected)
	body.Capability.ExecutionOutline = normalizeStringList(body.Capability.ExecutionOutline)
	body.Capability.Constraints = normalizeStringList(body.Capability.Constraints)
	body.Capability.Examples = normalizeStringList(body.Capability.Examples)
	body.Capability.Requirements = normalizeCapabilityRequirementList(body.Capability.Requirements)
	switch {
	case body.Capability.ID == "":
		return fmt.Errorf("%w: capability.id is required", ErrInvalidBody)
	case body.Capability.Summary == "":
		return fmt.Errorf("%w: capability.summary is required", ErrInvalidBody)
	case body.Capability.Outcome == "":
		return fmt.Errorf("%w: capability.outcome is required", ErrInvalidBody)
	case body.Capability.Digest == "":
		return fmt.Errorf("%w: capability.digest is required", ErrInvalidBody)
	}
	if err := validateCapabilityRequirements(body.Capability.Requirements, "capability.requirements"); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidBody, err)
	}
	expectedDigest, err := aghconfig.CanonicalCapabilityDigest(aghconfig.CapabilityDef{
		ID: body.Capability.ID, Summary: body.Capability.Summary, Outcome: body.Capability.Outcome,
		Version: body.Capability.Version, ContextNeeded: slices.Clone(body.Capability.ContextNeeded),
		ArtifactsExpected: slices.Clone(body.Capability.ArtifactsExpected),
		ExecutionOutline:  slices.Clone(body.Capability.ExecutionOutline),
		Constraints:       slices.Clone(body.Capability.Constraints), Examples: slices.Clone(body.Capability.Examples),
		Requirements: slices.Clone(body.Capability.Requirements),
	})
	if err != nil {
		return fmt.Errorf("%w: compute capability digest: %w", ErrInvalidBody, err)
	}
	if body.Capability.Digest != expectedDigest {
		return fmt.Errorf(
			"%w: capability.digest=%q does not match canonical digest %q",
			ErrVerificationFailed,
			body.Capability.Digest,
			expectedDigest,
		)
	}
	return nil
}

func normalizeAndValidateReceiptBody(body *ReceiptBody) error {
	body.ForID = strings.TrimSpace(body.ForID)
	if body.ForID == "" {
		return fmt.Errorf("%w: receipt for_id is required", ErrInvalidBody)
	}
	body.Status = ReceiptStatus(strings.TrimSpace(string(body.Status)))
	if err := body.Status.Validate(); err != nil {
		return err
	}
	if body.ReasonCode != nil {
		normalized := ReasonCode(strings.TrimSpace(string(*body.ReasonCode)))
		body.ReasonCode = &normalized
		if err := body.ReasonCode.Validate(); err != nil {
			return err
		}
	}
	body.Detail = normalizeOptionalText(body.Detail)
	switch body.Status {
	case ReceiptStatusAccepted:
		if body.ReasonCode != nil {
			return fmt.Errorf("%w: accepted receipt must not include reason_code", ErrInvalidBody)
		}
	case ReceiptStatusRejected, ReceiptStatusDuplicate, ReceiptStatusExpired, ReceiptStatusUnsupported:
		if body.ReasonCode == nil {
			return fmt.Errorf("%w: receipt status %q requires reason_code", ErrInvalidBody, body.Status)
		}
	}
	return nil
}

func normalizeAndValidateTraceBody(body *TraceBody) error {
	body.State = WorkState(strings.TrimSpace(string(body.State)))
	return body.State.Validate()
}

func normalizeCapabilityRequirementList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, len(values))
	for index, value := range values {
		normalized[index] = strings.TrimSpace(value)
	}
	return normalized
}

func validateCapabilityRequirements(requirements []string, fieldPrefix string) error {
	seen := make(map[string]int, len(requirements))
	for index, requirement := range requirements {
		if requirement == "" {
			return fmt.Errorf("%s[%d] is required", fieldPrefix, index)
		}
		if priorIndex, ok := seen[requirement]; ok {
			return fmt.Errorf(
				"%s duplicate value %q after normalization at indexes %d and %d",
				fieldPrefix,
				requirement,
				priorIndex,
				index,
			)
		}
		seen[requirement] = index
	}
	return nil
}

func validateJSONObject(field string, raw json.RawMessage) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: %s is required", ErrMissingField, field)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, fmt.Errorf("%w: %s must be a JSON object: %w", ErrInvalidField, field, err)
	}
	return object, nil
}

func decodeJSON(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	return decoder.Decode(target)
}
