package globaldb

import (
	"encoding/json"
	"fmt"

	contractspkg "github.com/compozy/compozy/internal/contracts"
	looppkg "github.com/compozy/compozy/internal/loop"
)

func validateContractWaitPayload(expect json.RawMessage, payload json.RawMessage) error {
	if err := contractspkg.ValidateWaitPayload(expect, payload); err != nil {
		return fmt.Errorf("%w: %v", looppkg.ErrValidation, err)
	}
	return nil
}
