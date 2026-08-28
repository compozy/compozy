package calls

// PublicMessageDelivery projects a durable or public delivery word into the stable public vocabulary.
func PublicMessageDelivery(value string) MessageDelivery {
	switch value {
	case string(DeliveryStatePending), string(MessageDeliveryQueued):
		return MessageDeliveryQueued
	case string(DeliveryStateAttention):
		return MessageDeliveryAttention
	case string(DeliveryStateInjected), string(MessageDeliveryDeliveredIntoTurn):
		return MessageDeliveryDeliveredIntoTurn
	case string(DeliveryStateWoken), string(MessageDeliveryWoke):
		return MessageDeliveryWoke
	case string(DeliveryStateFailed):
		return MessageDeliveryFailed
	default:
		return MessageDeliveryFailed
	}
}
