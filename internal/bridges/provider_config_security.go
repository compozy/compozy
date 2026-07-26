package bridges

import (
	"encoding/json"

	bridgecontract "github.com/compozy/compozy/internal/bridges/contract"
)

func normalizeProviderConfigJSON(value json.RawMessage) (json.RawMessage, error) {
	return bridgecontract.NormalizeProviderConfigJSON(value)
}
