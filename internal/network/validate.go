package network

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/rules"
	"github.com/compozy/agh/internal/store"
)

var (
	// ErrInvalidEnvelope reports a structurally invalid envelope.
	ErrInvalidEnvelope = errors.New("network: invalid envelope")
	// ErrMissingField reports a required protocol field is absent.
	ErrMissingField = errors.New("network: missing field")
	// ErrInvalidField reports a present field violates protocol rules.
	ErrInvalidField = errors.New("network: invalid field")
	// ErrInvalidKind reports an unknown or unsupported message kind.
	ErrInvalidKind = errors.New("network: invalid kind")
	// ErrInvalidBody reports a malformed or invalid kind-specific body.
	ErrInvalidBody = errors.New("network: invalid body")
	// ErrEnvelopeTooLarge reports an envelope exceeding the protocol size limit.
	ErrEnvelopeTooLarge = errors.New("network: envelope too large")
	// ErrExpired reports an envelope that is already expired.
	ErrExpired = errors.New("network: expired")
	// ErrReplayTooOld reports an envelope outside the receiver replay window.
	ErrReplayTooOld = errors.New("network: replay window exceeded")
	// ErrVerificationFailed reports a syntactically valid envelope whose integrity checks failed.
	ErrVerificationFailed = errors.New("network: verification failed")
	// ErrLegacyFieldRejected reports an obsolete hard-cut wire field.
	ErrLegacyFieldRejected = errors.New("network: legacy field rejected")
	// ErrDirectRoomCollision reports a direct_id bound to an unexpected peer pair.
	ErrDirectRoomCollision = errors.New("network: direct room collision")
)

const (
	// DefaultMaxReplayAge is the maximum receiver replay age when expires_at is absent.
	DefaultMaxReplayAge      = 5 * time.Minute
	maxProtocolEnvelopeBytes = 1 << 20
)

// ValidateOptions configures envelope validation and normalization.
type ValidateOptions struct {
	Now          time.Time
	MaxReplayAge time.Duration
}

// ParseEnvelope decodes, validates, and normalizes one raw envelope.
func ParseEnvelope(data []byte, opts ValidateOptions) (Envelope, error) {
	if err := rejectLegacyEnvelopeFields(data); err != nil {
		return Envelope{}, err
	}
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return Envelope{}, fmt.Errorf("%w: decode envelope: %w", ErrInvalidEnvelope, err)
	}
	return NormalizeEnvelope(env, opts)
}

// NormalizeEnvelope trims identifier fields, validates the envelope, and returns a safe copy.
func NormalizeEnvelope(env Envelope, opts ValidateOptions) (Envelope, error) {
	opts = opts.withDefaults()
	normalized := normalizeEnvelopeCopy(env)
	if err := validateEnvelopeHeader(normalized); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelopeParticipants(normalized); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelopeReferences(normalized); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelopeBodyAndFreshness(normalized, opts); err != nil {
		return Envelope{}, err
	}
	if err := canonicalizeEnvelopeRawMessages(&normalized); err != nil {
		return Envelope{}, err
	}
	if err := validateEnvelopeEncodedSize(normalized); err != nil {
		return Envelope{}, err
	}
	return normalized, nil
}

func canonicalizeEnvelopeRawMessages(env *Envelope) error {
	if env == nil {
		return fmt.Errorf("%w: envelope is required", ErrInvalidEnvelope)
	}
	body, err := compactEnvelopeRawMessage("body", env.Body)
	if err != nil {
		return err
	}
	env.Body = body
	if env.Proof != nil {
		for key, raw := range *env.Proof {
			compacted, compactErr := compactEnvelopeRawMessage("proof."+key, raw)
			if compactErr != nil {
				return compactErr
			}
			(*env.Proof)[key] = compacted
		}
	}
	for key, raw := range env.Ext {
		compacted, compactErr := compactEnvelopeRawMessage("ext."+key, raw)
		if compactErr != nil {
			return compactErr
		}
		env.Ext[key] = compacted
	}
	return nil
}

func compactEnvelopeRawMessage(field string, raw json.RawMessage) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, raw); err != nil {
		return nil, fmt.Errorf("%w: compact %s: %w", ErrInvalidEnvelope, field, err)
	}
	return append(json.RawMessage(nil), compacted.Bytes()...), nil
}

func validateEnvelopeEncodedSize(env Envelope) error {
	encoded, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("%w: encode envelope: %w", ErrInvalidEnvelope, err)
	}
	if len(encoded) > maxProtocolEnvelopeBytes {
		return fmt.Errorf(
			"%w: encoded_bytes=%d max_bytes=%d",
			ErrEnvelopeTooLarge,
			len(encoded),
			maxProtocolEnvelopeBytes,
		)
	}
	return nil
}

// ValidateEnvelope validates one envelope without returning a normalized copy.
func ValidateEnvelope(env Envelope, opts ValidateOptions) error {
	_, err := NormalizeEnvelope(env, opts)
	return err
}

// ValidateChannel reports whether the channel matches the RFC grammar.
func ValidateChannel(channel string) error {
	if !rules.ValidChannel(channel) {
		return fmt.Errorf("%w: channel=%q", ErrInvalidField, channel)
	}
	return nil
}

// ValidateWorkspaceID reports whether the workspace identity is safe for protocol use.
func ValidateWorkspaceID(workspaceID string) error {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrMissingField)
	}
	if strings.ContainsAny(trimmed, ". *>") || containsControlCharacter(trimmed) {
		return fmt.Errorf("%w: workspace_id=%q", ErrInvalidField, workspaceID)
	}
	return nil
}

// ValidateSurface reports whether the surface matches the RFC conversation values.
func ValidateSurface(surface Surface) error {
	return Surface(strings.TrimSpace(string(surface))).Validate()
}

// ValidateWorkState reports whether the state is a known work lifecycle state.
func ValidateWorkState(state WorkState) error {
	return WorkState(strings.TrimSpace(string(state))).Validate()
}

// ValidateWorkTransition reports whether a trace may advance work between states.
func ValidateWorkTransition(from WorkState, to WorkState) error {
	current := WorkState(strings.TrimSpace(string(from)))
	next := WorkState(strings.TrimSpace(string(to)))
	if err := ValidateWorkState(current); err != nil {
		return err
	}
	if err := ValidateWorkState(next); err != nil {
		return err
	}
	if !canApplyTrace(current, next) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidStateTransition, current, next)
	}
	return nil
}

func validateEnvelopeHeader(env Envelope) error {
	if env.Protocol == "" {
		return fmt.Errorf("%w: protocol is required", ErrMissingField)
	}
	if env.Protocol != ProtocolV0 {
		return fmt.Errorf("%w: protocol=%q", ErrInvalidField, env.Protocol)
	}
	if env.ID == "" {
		return fmt.Errorf("%w: id is required", ErrMissingField)
	}
	if err := ValidateWorkspaceID(env.WorkspaceID); err != nil {
		return err
	}
	if err := env.Kind.Validate(); err != nil {
		return err
	}
	if env.Channel == "" {
		return fmt.Errorf("%w: channel is required", ErrMissingField)
	}
	return ValidateChannel(env.Channel)
}

func validateEnvelopeParticipants(env Envelope) error {
	if env.From == "" {
		return fmt.Errorf("%w: from is required", ErrMissingField)
	}
	if err := ValidatePeerID(env.From); err != nil {
		return fmt.Errorf("%w: from", err)
	}
	if env.To != nil {
		if err := ValidatePeerID(*env.To); err != nil {
			return fmt.Errorf("%w: to", err)
		}
	}
	if _, err := store.NormalizeNetworkPeerIDs(env.Mentions, "mentions"); err != nil {
		return err
	}
	return nil
}

func validateEnvelopeReferences(env Envelope) error {
	if err := validateOptionalIdentifierField(env.ReplyTo, "reply_to"); err != nil {
		return err
	}
	if err := validateOptionalIdentifierField(env.TraceID, "trace_id"); err != nil {
		return err
	}
	if err := validateOptionalIdentifierField(env.CausationID, "causation_id"); err != nil {
		return err
	}
	if env.TS <= 0 {
		return fmt.Errorf("%w: ts is required", ErrMissingField)
	}
	return ValidateEnvelopeConversation(env)
}

func validateOptionalIdentifierField(value *string, field string) error {
	if value != nil && *value == "" {
		return fmt.Errorf("%w: %s", ErrInvalidField, field)
	}
	return nil
}

func validateEnvelopeBodyAndFreshness(env Envelope, opts ValidateOptions) error {
	if _, err := validateJSONObject("body", env.Body); err != nil {
		return err
	}
	if _, err := env.DecodeBody(); err != nil {
		return err
	}
	if err := validateKindEnvelopeRules(env); err != nil {
		return err
	}
	if err := validateEnvelopeContainsNoRawSecrets(env); err != nil {
		return err
	}
	return validateEnvelopeFreshness(env, opts)
}

func validateEnvelopeFreshness(env Envelope, opts ValidateOptions) error {
	nowUnix := opts.Now.Unix()
	maxAge := int64(opts.MaxReplayAge / time.Second)
	if maxAge > 0 && env.TS-nowUnix > maxAge {
		return fmt.Errorf("%w: ts=%d max_replay_age=%s", ErrReplayTooOld, env.TS, opts.MaxReplayAge)
	}
	if env.ExpiresAt != nil {
		if *env.ExpiresAt <= nowUnix {
			return fmt.Errorf("%w: expires_at=%d", ErrExpired, *env.ExpiresAt)
		}
		return nil
	}
	if maxAge > 0 && nowUnix-env.TS > maxAge {
		return fmt.Errorf("%w: ts=%d max_replay_age=%s", ErrReplayTooOld, env.TS, opts.MaxReplayAge)
	}
	return nil
}

func containsControlCharacter(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return r < 0x20 || r == 0x7f
	})
}

func (opts ValidateOptions) withDefaults() ValidateOptions {
	if opts.Now.IsZero() {
		opts.Now = time.Now().UTC()
	}
	if opts.MaxReplayAge <= 0 {
		opts.MaxReplayAge = DefaultMaxReplayAge
	}
	return opts
}
