package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

type targetSnapshotDeliveryDefaults struct {
	PeerID   string                 `json:"peer_id,omitempty"`
	ThreadID string                 `json:"thread_id,omitempty"`
	GroupID  string                 `json:"group_id,omitempty"`
	Mode     bridgepkg.DeliveryMode `json:"mode,omitempty"`
}

// TargetSnapshotsFromManagedInstances derives a conservative target directory
// from daemon-provided managed-instance delivery defaults. Providers with
// native directory APIs can override RuntimeConfig.TargetSnapshots.
func TargetSnapshotsFromManagedInstances(
	ctx context.Context,
	session *Session,
	req bridgepkg.BridgeTargetSnapshotRequest,
) ([]bridgepkg.BridgeTargetSnapshot, error) {
	if ctx == nil {
		return nil, errors.New("bridgesdk: target snapshot context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if session == nil {
		return nil, errors.New("bridgesdk: target snapshot session is required")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}
	runtime := session.BridgeRuntime()
	if runtime == nil {
		return nil, errors.New("bridgesdk: bridge runtime snapshot is required")
	}
	bridgeID := strings.TrimSpace(req.BridgeInstanceID)
	for _, managed := range runtime.ManagedInstances {
		instance := managed.Instance
		if strings.TrimSpace(instance.ID) != bridgeID {
			continue
		}
		return targetSnapshotsForManagedInstance(runtime, instance)
	}
	return nil, fmt.Errorf("bridgesdk: bridge instance %q is not in the provider cache", bridgeID)
}

func targetSnapshotsForManagedInstance(
	runtime *subprocess.InitializeBridgeRuntime,
	instance bridgepkg.BridgeInstance,
) ([]bridgepkg.BridgeTargetSnapshot, error) {
	defaults, err := decodeTargetSnapshotDeliveryDefaults(instance.DeliveryDefaults)
	if err != nil {
		return nil, err
	}
	qualifier := bridgepkg.NormalizeBridgeTargetQualifier(
		firstNonBlank(runtime.Platform, runtime.Provider, instance.Platform),
	)
	targets := make([]bridgepkg.BridgeTargetSnapshot, 0, 3)
	if peerID := strings.TrimSpace(defaults.PeerID); peerID != "" {
		targets = append(targets, bridgepkg.BridgeTargetSnapshot{
			CanonicalRoute: bridgeTargetCanonicalRoute(qualifier, "user", peerID),
			DisplayName:    "@" + peerID,
			TargetType:     bridgepkg.BridgeTargetTypeUser,
			Qualifier:      qualifier,
			Capabilities:   []string{string(defaults.modeOrDefault())},
		})
	}
	if groupID := strings.TrimSpace(defaults.GroupID); groupID != "" {
		targets = append(targets, bridgepkg.BridgeTargetSnapshot{
			CanonicalRoute: bridgeTargetCanonicalRoute(qualifier, "channel", groupID),
			DisplayName:    "#" + groupID,
			TargetType:     bridgepkg.BridgeTargetTypeChannel,
			Qualifier:      qualifier,
			Capabilities:   []string{string(defaults.modeOrDefault())},
		})
	}
	if threadID := strings.TrimSpace(defaults.ThreadID); threadID != "" {
		targets = append(targets, bridgepkg.BridgeTargetSnapshot{
			CanonicalRoute: bridgeTargetCanonicalRoute(qualifier, "thread", threadID),
			DisplayName:    "thread/" + threadID,
			TargetType:     bridgepkg.BridgeTargetTypeThread,
			Qualifier:      qualifier,
			Capabilities:   []string{string(bridgepkg.DeliveryModeReply)},
		})
	}
	return targets, nil
}

func decodeTargetSnapshotDeliveryDefaults(raw json.RawMessage) (targetSnapshotDeliveryDefaults, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return targetSnapshotDeliveryDefaults{}, nil
	}
	var defaults targetSnapshotDeliveryDefaults
	if err := json.Unmarshal(raw, &defaults); err != nil {
		return targetSnapshotDeliveryDefaults{}, fmt.Errorf("bridgesdk: decode delivery defaults for targets: %w", err)
	}
	defaults.PeerID = strings.TrimSpace(defaults.PeerID)
	defaults.ThreadID = strings.TrimSpace(defaults.ThreadID)
	defaults.GroupID = strings.TrimSpace(defaults.GroupID)
	defaults.Mode = defaults.Mode.Normalize()
	if defaults.Mode != "" {
		if err := defaults.Mode.Validate(); err != nil {
			return targetSnapshotDeliveryDefaults{}, err
		}
	}
	return defaults, nil
}

func (d targetSnapshotDeliveryDefaults) modeOrDefault() bridgepkg.DeliveryMode {
	if d.Mode == "" {
		return bridgepkg.DeliveryModeDirectSend
	}
	return d.Mode.Normalize()
}

func bridgeTargetCanonicalRoute(platform string, targetType string, id string) string {
	return strings.Join([]string{
		bridgepkg.NormalizeBridgeTargetQualifier(platform),
		strings.TrimSpace(targetType),
		strings.TrimSpace(id),
	}, ":")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
