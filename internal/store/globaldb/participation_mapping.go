package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

type participationSnapshotFields struct {
	JSON    string
	Mode    string
	Channel sql.NullString
	Source  string
}

func encodeParticipationSnapshot(
	ownerWorkspaceID string,
	spec participation.Spec,
) (participationSnapshotFields, error) {
	if spec == (participation.Spec{}) {
		spec = participation.LocalSpec()
	}
	if err := participation.ValidateSpec(spec); err != nil {
		return participationSnapshotFields{}, fmt.Errorf("store: validate network participation snapshot: %w", err)
	}
	if err := validateParticipationWorkspace(ownerWorkspaceID, spec); err != nil {
		return participationSnapshotFields{}, err
	}
	raw, err := json.Marshal(spec)
	if err != nil {
		return participationSnapshotFields{}, fmt.Errorf("store: encode network participation snapshot: %w", err)
	}
	return participationSnapshotFields{
		JSON:    string(raw),
		Mode:    string(spec.Mode),
		Channel: nullableParticipationChannel(spec.ChannelID),
		Source:  string(spec.Source),
	}, nil
}

func nullableParticipationChannel(channel string) sql.NullString {
	return store.SQLNullString(channel)
}

func decodeParticipationSnapshot(
	ownerWorkspaceID string,
	raw string,
	mode string,
	channel sql.NullString,
	source string,
) (participation.Spec, error) {
	var spec participation.Spec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return participation.Spec{}, fmt.Errorf("store: decode network participation snapshot: %w", err)
	}
	if err := participation.ValidateSpec(spec); err != nil {
		return participation.Spec{}, fmt.Errorf("store: validate network participation snapshot: %w", err)
	}
	if err := validateParticipationWorkspace(ownerWorkspaceID, spec); err != nil {
		return participation.Spec{}, err
	}
	projectedChannel := ""
	if channel.Valid {
		projectedChannel = strings.TrimSpace(channel.String)
	}
	if string(spec.Mode) != strings.TrimSpace(mode) ||
		spec.ChannelID != projectedChannel ||
		string(spec.Source) != strings.TrimSpace(source) {
		return participation.Spec{}, fmt.Errorf(
			"store: network participation snapshot projections do not match snapshot",
		)
	}
	return spec, nil
}

func validateParticipationWorkspace(ownerWorkspaceID string, spec participation.Spec) error {
	if spec.Mode != participation.ModeLive {
		return nil
	}
	ownerWorkspaceID = strings.TrimSpace(ownerWorkspaceID)
	if ownerWorkspaceID == "" || spec.WorkspaceID != ownerWorkspaceID {
		return fmt.Errorf(
			"store: network participation snapshot workspace %q does not match owner workspace %q",
			spec.WorkspaceID,
			ownerWorkspaceID,
		)
	}
	return nil
}
