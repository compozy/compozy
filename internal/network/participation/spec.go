package participation

import "strings"

// LocalSpec returns the canonical zero-Network participation snapshot.
func LocalSpec() Spec {
	return Spec{
		Version: SpecVersion,
		Mode:    ModeLocal,
		Source:  SourceBuiltInLocal,
	}
}

// ValidateSpec verifies that an immutable participation snapshot is complete and internally consistent.
func ValidateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Version) != SpecVersion {
		return newContractError(
			ErrStrategyInvalid,
			"version",
			"must equal %q",
			SpecVersion,
		)
	}
	if err := validateMode(spec.Mode); err != nil {
		return err
	}
	if err := validateSource(spec.Source); err != nil {
		return err
	}
	if spec.Mode == ModeLocal {
		if strings.TrimSpace(spec.WorkspaceID) != "" ||
			spec.ChannelStrategy != "" ||
			strings.TrimSpace(spec.ChannelID) != "" ||
			!spec.Bounds.IsZero() {
			return newContractError(
				ErrStrategyChannelConflict,
				"mode",
				"local snapshot cannot carry workspace, channel, or bounds",
			)
		}
		return nil
	}
	if strings.TrimSpace(spec.WorkspaceID) == "" {
		return newContractError(ErrStrategyInvalid, "workspace_id", "live snapshot requires a workspace")
	}
	if err := validateStrategy(spec.ChannelStrategy); err != nil {
		return err
	}
	if strings.TrimSpace(spec.ChannelID) == "" || !channelPattern.MatchString(spec.ChannelID) {
		return newContractError(
			ErrStrategyInvalid,
			"channel_id",
			"live snapshot requires a channel matching %q",
			channelPattern.String(),
		)
	}
	return validateBounds(spec.Bounds)
}

func validateSource(source Source) error {
	switch source {
	case SourceExplicitRequest,
		SourceTaskProfile,
		SourceWorkspaceCoordination,
		SourceLoopDefinition,
		SourceAutomationJob,
		SourceBuiltInLocal:
		return nil
	default:
		return newContractError(ErrStrategyInvalid, "source", "unsupported value %q", source)
	}
}
