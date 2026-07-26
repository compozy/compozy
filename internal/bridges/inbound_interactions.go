package bridges

import (
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// InboundEventFamily identifies the typed inbound bridge event family.
type InboundEventFamily string

const (
	// InboundEventFamilyMessage identifies a text-and-attachment message event.
	InboundEventFamilyMessage InboundEventFamily = InboundEventFamily(bridgecontract.InboundEventFamilyMessage)
	// InboundEventFamilyCommand identifies a typed slash-command style event.
	InboundEventFamilyCommand InboundEventFamily = InboundEventFamily(bridgecontract.InboundEventFamilyCommand)
	// InboundEventFamilyAction identifies a typed button/action event.
	InboundEventFamilyAction InboundEventFamily = InboundEventFamily(bridgecontract.InboundEventFamilyAction)
	// InboundEventFamilyReaction identifies a typed reaction add/remove event.
	InboundEventFamilyReaction InboundEventFamily = InboundEventFamily(bridgecontract.InboundEventFamilyReaction)
	// InboundEventFamilyEdit identifies an update or deletion of an existing message.
	InboundEventFamilyEdit InboundEventFamily = InboundEventFamily(bridgecontract.InboundEventFamilyEdit)
)

// Normalize returns the canonical inbound event-family representation.
func (f InboundEventFamily) Normalize() InboundEventFamily {
	return InboundEventFamily(bridgecontract.InboundEventFamily(f).Normalize())
}

// Validate reports whether the inbound event family belongs to the supported set.
func (f InboundEventFamily) Validate() error {
	return bridgecontract.InboundEventFamily(f).Validate()
}

// InboundCommand captures a typed slash-command style inbound interaction.
type InboundCommand struct {
	Command   string `json:"command"`
	Text      string `json:"text,omitempty"`
	TriggerID string `json:"trigger_id,omitempty"`
}

// Validate reports whether the command payload contains the required identity.
func (c InboundCommand) Validate() error {
	return inboundCommandToContract(c).Validate()
}

// InboundAction captures a typed button/action inbound interaction.
type InboundAction struct {
	ActionID  string `json:"action_id"`
	MessageID string `json:"message_id,omitempty"`
	Value     string `json:"value,omitempty"`
	TriggerID string `json:"trigger_id,omitempty"`
}

// Validate reports whether the action payload contains the required identity.
func (a InboundAction) Validate() error {
	return inboundActionToContract(a).Validate()
}

// InboundReaction captures a typed reaction add/remove inbound interaction.
type InboundReaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
	RawEmoji  string `json:"raw_emoji,omitempty"`
	Added     bool   `json:"added"`
}

// Validate reports whether the reaction payload contains the required identity.
func (r InboundReaction) Validate() error {
	return inboundReactionToContract(r).Validate()
}

// InboundEditOperation distinguishes replacement text from message deletion.
type InboundEditOperation string

const (
	// InboundEditOperationUpdated replaces the existing message text.
	InboundEditOperationUpdated InboundEditOperation = InboundEditOperation(bridgecontract.InboundEditOperationUpdated)
	// InboundEditOperationDeleted removes the existing message.
	InboundEditOperationDeleted InboundEditOperation = InboundEditOperation(bridgecontract.InboundEditOperationDeleted)
)

// Normalize returns the canonical inbound edit operation.
func (o InboundEditOperation) Normalize() InboundEditOperation {
	return InboundEditOperation(bridgecontract.InboundEditOperation(o).Normalize())
}

// Validate reports whether the edit operation belongs to the supported set.
func (o InboundEditOperation) Validate() error {
	return bridgecontract.InboundEditOperation(o).Validate()
}

// InboundEdit captures one typed update or deletion of an existing platform message.
type InboundEdit struct {
	MessageID         string               `json:"message_id"`
	NewText           string               `json:"new_text"`
	OriginalTimestamp time.Time            `json:"original_timestamp"`
	Operation         InboundEditOperation `json:"operation"`
}

// Validate reports whether the edit carries an unambiguous operation payload.
func (e InboundEdit) Validate() error {
	return inboundEditToContract(e).Validate()
}

func inboundCommandToContract(command InboundCommand) bridgecontract.InboundCommand {
	return bridgecontract.InboundCommand{
		Command:   command.Command,
		Text:      command.Text,
		TriggerID: command.TriggerID,
	}
}

func inboundActionToContract(action InboundAction) bridgecontract.InboundAction {
	return bridgecontract.InboundAction{
		ActionID:  action.ActionID,
		MessageID: action.MessageID,
		Value:     action.Value,
		TriggerID: action.TriggerID,
	}
}

func inboundReactionToContract(reaction InboundReaction) bridgecontract.InboundReaction {
	return bridgecontract.InboundReaction{
		MessageID: reaction.MessageID,
		Emoji:     reaction.Emoji,
		RawEmoji:  reaction.RawEmoji,
		Added:     reaction.Added,
	}
}

func inboundEditToContract(edit InboundEdit) bridgecontract.InboundEdit {
	return bridgecontract.InboundEdit{
		MessageID:         edit.MessageID,
		NewText:           edit.NewText,
		OriginalTimestamp: edit.OriginalTimestamp,
		Operation:         bridgecontract.InboundEditOperation(edit.Operation),
	}
}
