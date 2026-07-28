package contract

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// UnmarshalJSON rejects obsolete fields and caller-supplied envelope identity.
func (r *NetworkSendRequest) UnmarshalJSON(data []byte) error {
	type networkSendRequestAlias NetworkSendRequest
	raw, err := decodeNetworkSendObject(data)
	if err != nil {
		return err
	}
	if _, ok := raw["interaction_id"]; ok {
		return errors.New("network send request rejects interaction_id; use work_id")
	}
	if err := ValidateNoCallerSuppliedNetworkIdentity(raw, "network send request"); err != nil {
		return err
	}

	var decoded networkSendRequestAlias
	if err := decodeStrictContractJSON(data, &decoded); err != nil {
		return err
	}
	if strings.TrimSpace(decoded.Kind) == contractDirectKey {
		return errors.New("network send request rejects kind direct; use surface direct with kind say")
	}
	*r = NetworkSendRequest(decoded)
	return nil
}

// UnmarshalJSON rejects caller-supplied envelope identity on the agent channel surface.
func (r *AgentChannelSendRequest) UnmarshalJSON(data []byte) error {
	type agentChannelSendRequestAlias AgentChannelSendRequest
	raw, err := decodeNetworkSendObject(data)
	if err != nil {
		return err
	}
	if err := ValidateNoCallerSuppliedNetworkIdentity(raw, "agent channel send request"); err != nil {
		return err
	}

	var decoded agentChannelSendRequestAlias
	if err := decodeStrictContractJSON(data, &decoded); err != nil {
		return err
	}
	*r = AgentChannelSendRequest(decoded)
	return nil
}

func decodeNetworkSendObject(data []byte) (map[string]json.RawMessage, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ValidateNoCallerSuppliedNetworkIdentity rejects caller-authored sender identity and proof fields.
func ValidateNoCallerSuppliedNetworkIdentity(raw map[string]json.RawMessage, requestName string) error {
	for key := range raw {
		normalized := strings.TrimSpace(key)
		if !strings.EqualFold(normalized, "from") && !strings.EqualFold(normalized, "proof") {
			continue
		}
		return fmt.Errorf(
			"%s rejects %s; sender identity and proof are daemon-derived",
			requestName,
			normalized,
		)
	}
	return nil
}
