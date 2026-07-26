package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
)

func extensionMapFromNetworkMessage(entry store.NetworkMessageEntry) (network.ExtensionMap, error) {
	trimmed := strings.TrimSpace(string(entry.ExtJSON))
	if trimmed == "" || trimmed == "{}" {
		return nil, nil
	}
	var ext network.ExtensionMap
	if err := json.Unmarshal([]byte(trimmed), &ext); err != nil {
		return nil, fmt.Errorf("decode network message ext_json: %w", err)
	}
	return ext, nil
}
