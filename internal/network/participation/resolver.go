package participation

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
)

type AvailabilityFunc func(ctx context.Context) (bool, error)

type ChannelExistsFunc func(ctx context.Context, workspaceID, channelID string) (bool, error)

type WorkspaceCoordinationFunc func(ctx context.Context, workspaceID string) (bool, error)

type LiveSupportFunc func(ctx context.Context, in ResolveInput) (bool, error)

type AuthorityFunc func(ctx context.Context, in ResolveInput, spec Spec) (bool, error)

type ResolverOptions struct {
	Defaults              Bounds
	Limits                Limits
	Availability          AvailabilityFunc
	ChannelExists         ChannelExistsFunc
	WorkspaceCoordination WorkspaceCoordinationFunc
	LiveSupport           LiveSupportFunc
	Authority             AuthorityFunc
}

type resolver struct {
	options ResolverOptions
}

func NewResolver(options ResolverOptions) (Resolver, error) {
	if err := validateResolverOptions(options); err != nil {
		return nil, err
	}
	return &resolver{options: options}, nil
}

func (r *resolver) Resolve(ctx context.Context, in ResolveInput) (Spec, error) {
	intent, err := r.SelectIntent(ctx, in)
	if err != nil {
		return Spec{}, err
	}
	return r.ResolveIntent(ctx, in, intent)
}

func (r *resolver) SelectIntent(ctx context.Context, in ResolveInput) (Intent, error) {
	if err := validateResolveOwner(in); err != nil {
		return Intent{}, err
	}
	request, source, err := r.selectRequest(ctx, in)
	if err != nil {
		return Intent{}, err
	}
	return Intent{Request: CloneRequest(request), Source: source}, nil
}

func (r *resolver) ResolveIntent(
	ctx context.Context,
	in ResolveInput,
	intent Intent,
) (Spec, error) {
	if err := validateResolveOwner(in); err != nil {
		return Spec{}, err
	}
	request := CloneRequest(intent.Request)
	source, err := normalizeIntentSource(intent.Source)
	if err != nil {
		return Spec{}, err
	}
	if request == nil {
		return localSpec(source), nil
	}
	normalized, err := ValidateWithBounds(*request, r.options.Defaults, r.options.Limits)
	if err != nil {
		return Spec{}, err
	}
	if normalized.Mode == nil || *normalized.Mode == ModeLocal {
		return localSpec(source), nil
	}
	if err := r.checkLiveCapability(ctx, in); err != nil {
		return Spec{}, err
	}
	bounds, err := requestBounds(normalized)
	if err != nil {
		return Spec{}, fmt.Errorf("resolve participation bounds: %w", err)
	}
	spec := Spec{
		Version:         SpecVersion,
		Mode:            ModeLive,
		WorkspaceID:     strings.TrimSpace(in.WorkspaceID),
		ChannelStrategy: *normalized.ChannelStrategy,
		ChannelID:       valueOrZero(normalized.ChannelID),
		Source:          source,
		Bounds:          bounds,
	}
	spec.ChannelID, err = r.resolveChannel(ctx, in, spec)
	if err != nil {
		return Spec{}, err
	}
	if !delegatedAuthorityAllows(in.Authority, spec.ChannelID) {
		return Spec{}, newContractError(
			ErrAuthorityDenied,
			"channel_id",
			"delegated authority does not include %q",
			spec.ChannelID,
		)
	}
	if r.options.Authority != nil {
		allowed, authorityErr := r.options.Authority(ctx, in, spec)
		if authorityErr != nil {
			return Spec{}, fmt.Errorf("check network participation authority: %w", authorityErr)
		}
		if !allowed {
			return Spec{}, newContractError(
				ErrAuthorityDenied,
				"channel_id",
				"authority does not include %q",
				spec.ChannelID,
			)
		}
	}
	return spec, nil
}

func normalizeIntentSource(source Source) (Source, error) {
	normalized := Source(strings.TrimSpace(string(source)))
	switch normalized {
	case SourceExplicitRequest,
		SourceTaskProfile,
		SourceWorkspaceCoordination,
		SourceLoopDefinition,
		SourceAutomationJob,
		SourceBuiltInLocal:
		return normalized, nil
	default:
		return "", fmt.Errorf("network participation source %q is not allowed", normalized)
	}
}

func validateResolveOwner(in ResolveInput) error {
	if err := ValidateOwner(in.Owner); err != nil {
		return fmt.Errorf("resolve participation owner: %w", err)
	}
	workspaceID := strings.TrimSpace(in.WorkspaceID)
	if workspaceID == "" {
		return fmt.Errorf("resolve participation: workspace_id is required")
	}
	if strings.TrimSpace(in.Owner.WorkspaceID) != workspaceID {
		return fmt.Errorf(
			"resolve participation: owner workspace_id %q does not match workspace_id %q",
			in.Owner.WorkspaceID,
			workspaceID,
		)
	}
	return nil
}

func delegatedAuthorityAllows(authority *AuthorityScope, channelID string) bool {
	if authority == nil || !authority.Enforced {
		return true
	}
	allowed := make([]string, 0, len(authority.ChannelIDs))
	for _, candidate := range authority.ChannelIDs {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			allowed = append(allowed, trimmed)
		}
	}
	return slices.Contains(allowed, strings.TrimSpace(channelID))
}

func validateResolverOptions(options ResolverOptions) error {
	if err := validateBounds(options.Defaults); err != nil {
		return fmt.Errorf("validate participation defaults: %w", err)
	}
	if err := options.Limits.Validate(); err != nil {
		return fmt.Errorf("validate participation limits: %w", err)
	}
	if _, err := ResolveBounds(nil, options.Defaults, options.Limits); err != nil {
		return fmt.Errorf("validate participation defaults against limits: %w", err)
	}
	return nil
}

func (r *resolver) selectRequest(ctx context.Context, in ResolveInput) (*Request, Source, error) {
	if in.Request != nil && !in.Request.isZero() {
		source, err := normalizeRequestSource(in.RequestSource)
		if err != nil {
			return nil, "", err
		}
		return in.Request, source, nil
	}
	if in.Definition != nil && !in.Definition.isZero() {
		return in.Definition, sourceForDefinition(in.Owner.Kind), nil
	}
	if !in.Coordinated || r.options.WorkspaceCoordination == nil {
		return nil, SourceBuiltInLocal, nil
	}
	enabled, err := r.options.WorkspaceCoordination(ctx, strings.TrimSpace(in.WorkspaceID))
	if err != nil {
		return nil, "", fmt.Errorf("read workspace network coordination: %w", err)
	}
	if !enabled {
		return nil, SourceBuiltInLocal, nil
	}
	live := ModeLive
	strategy := StrategyRun
	return &Request{Mode: &live, ChannelStrategy: &strategy}, SourceWorkspaceCoordination, nil
}

func normalizeRequestSource(source Source) (Source, error) {
	switch normalized := Source(strings.TrimSpace(string(source))); normalized {
	case "", SourceExplicitRequest:
		return SourceExplicitRequest, nil
	case SourceAutomationJob:
		return SourceAutomationJob, nil
	default:
		return "", fmt.Errorf("network participation request source %q is not allowed", normalized)
	}
}

func (r *resolver) checkLiveCapability(ctx context.Context, in ResolveInput) error {
	if r.options.Availability != nil {
		available, err := r.options.Availability(ctx)
		if err != nil {
			return fmt.Errorf("read network availability: %w", err)
		}
		if !available {
			return newContractError(ErrUnavailable, "mode", "live participation is administratively disabled")
		}
	}
	if r.options.LiveSupport != nil {
		supported, err := r.options.LiveSupport(ctx, in)
		if err != nil {
			return fmt.Errorf("check live participation support: %w", err)
		}
		if !supported {
			return newContractError(ErrLiveUnsupported, "mode", "live is unsupported; supported mode: local")
		}
	}
	return nil
}

func (r *resolver) resolveChannel(ctx context.Context, in ResolveInput, spec Spec) (string, error) {
	switch spec.ChannelStrategy {
	case StrategyNamed:
		if r.options.ChannelExists == nil {
			return "", newContractError(
				ErrChannelUnknown,
				"channel_id",
				"named channel %q cannot be verified",
				spec.ChannelID,
			)
		}
		exists, err := r.options.ChannelExists(ctx, spec.WorkspaceID, spec.ChannelID)
		if err != nil {
			return "", fmt.Errorf("look up network channel %q: %w", spec.ChannelID, err)
		}
		if !exists {
			return "", newContractError(
				ErrChannelUnknown,
				"channel_id",
				"channel %q does not exist in workspace %q",
				spec.ChannelID,
				spec.WorkspaceID,
			)
		}
		return spec.ChannelID, nil
	case StrategyRun:
		if strings.TrimSpace(in.RunID) == "" {
			return "", newContractError(
				ErrStrategyInvalid,
				"run_id",
				"run strategy requires a run id for owner kind %q",
				in.Owner.Kind,
			)
		}
		return deriveChannel("coord", in.RunID), nil
	case StrategyLoopRun:
		if strings.TrimSpace(in.LoopRunID) == "" {
			return "", newContractError(
				ErrStrategyInvalid,
				"loop_run_id",
				"loop_run strategy requires a loop run id for owner kind %q",
				in.Owner.Kind,
			)
		}
		return deriveChannel("loop", in.LoopRunID), nil
	default:
		return "", newContractError(
			ErrStrategyInvalid,
			"channel_strategy",
			"unsupported value %q",
			spec.ChannelStrategy,
		)
	}
}

func localSpec(source Source) Spec {
	return Spec{Version: SpecVersion, Mode: ModeLocal, Source: source}
}

func sourceForDefinition(kind OwnerKind) Source {
	switch kind {
	case OwnerKindTaskRun:
		return SourceTaskProfile
	case OwnerKindLoopRun:
		return SourceLoopDefinition
	case OwnerKindAutomationRun:
		return SourceAutomationJob
	default:
		return SourceBuiltInLocal
	}
}

func deriveChannel(prefix, rawID string) string {
	seed := strings.TrimSpace(rawID)
	normalizedSeed := strings.ToLower(seed)
	const digestLength = 12
	maxStemLength := 64 - len(prefix) - digestLength - 2
	cleaned := make([]rune, 0, min(len(normalizedSeed), maxStemLength))
	lastDash := false
	for _, r := range normalizedSeed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			cleaned = append(cleaned, r)
			lastDash = false
		case r == '-':
			if len(cleaned) > 0 && !lastDash {
				cleaned = append(cleaned, r)
				lastDash = true
			}
		default:
			if len(cleaned) > 0 && !lastDash {
				cleaned = append(cleaned, '-')
				lastDash = true
			}
		}
		if len(cleaned) >= maxStemLength {
			break
		}
	}
	stem := strings.Trim(string(cleaned), "-_")
	if stem == "" {
		stem = "id"
	}
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("%s-%s-%x", prefix, stem, sum[:digestLength/2])
}

func valueOrZero[T any](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
