package terminal

import (
	"encoding/json"
	"fmt"
)

// InputResolutionOutcome is the closed outcome vocabulary for resolved input requests.
type InputResolutionOutcome string

const (
	InputResolutionOutcomeAnswered   InputResolutionOutcome = "answered"
	InputResolutionOutcomeRejected   InputResolutionOutcome = "rejected"
	InputResolutionOutcomeSuperseded InputResolutionOutcome = "superseded"
	InputResolutionOutcomeExpired    InputResolutionOutcome = "expired"
)

func InputResolutionOutcomeValues() []string {
	return []string{
		string(InputResolutionOutcomeAnswered),
		string(InputResolutionOutcomeRejected),
		string(InputResolutionOutcomeSuperseded),
		string(InputResolutionOutcomeExpired),
	}
}

func ParseInputResolutionOutcome(value string) (InputResolutionOutcome, error) {
	outcome := InputResolutionOutcome(value)
	switch outcome {
	case InputResolutionOutcomeAnswered,
		InputResolutionOutcomeRejected,
		InputResolutionOutcomeSuperseded,
		InputResolutionOutcomeExpired:
		return outcome, nil
	default:
		return "", fmt.Errorf("terminal: unknown input resolution outcome %q", value)
	}
}

func (o *InputResolutionOutcome) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("terminal: decode input resolution outcome: %w", err)
	}
	parsed, err := ParseInputResolutionOutcome(value)
	if err != nil {
		return err
	}
	*o = parsed
	return nil
}
