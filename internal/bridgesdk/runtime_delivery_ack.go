package bridgesdk

import (
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

// AckCommittedResultUnavailable acknowledges that a remote mutation committed
// but its required result cannot be safely resumed or retried.
func (s *Session) AckCommittedResultUnavailable(
	req bridgepkg.DeliveryRequest,
	reason string,
) (bridgepkg.DeliveryAck, error) {
	ack := bridgepkg.DeliveryAck{
		DeliveryID: req.Event.DeliveryID,
		Seq:        req.Event.Seq,
		Outcome:    bridgepkg.DeliveryAckOutcomeCommittedResultUnavailable,
		Error: &bridgepkg.DeliveryErrorDetail{
			Message: strings.TrimSpace(reason),
		},
	}
	if err := ack.ValidateFor(req.Event); err != nil {
		return bridgepkg.DeliveryAck{}, err
	}
	return ack, nil
}
