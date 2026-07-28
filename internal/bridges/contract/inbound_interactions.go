package contract

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// InboundEventFamily identifies a typed inbound event family.
type InboundEventFamily string

const (
	InboundEventFamilyMessage  InboundEventFamily = "message"
	InboundEventFamilyCommand  InboundEventFamily = "command"
	InboundEventFamilyAction   InboundEventFamily = "action"
	InboundEventFamilyReaction InboundEventFamily = "reaction"
	InboundEventFamilyEdit     InboundEventFamily = "edit"
)

// InboundEventFamilyValues returns the closed wire values in stable order.
func InboundEventFamilyValues() []string {
	return []string{
		string(InboundEventFamilyMessage), string(InboundEventFamilyCommand),
		string(InboundEventFamilyAction), string(InboundEventFamilyReaction),
		string(InboundEventFamilyEdit),
	}
}

// Normalize returns the canonical inbound event family.
func (f InboundEventFamily) Normalize() InboundEventFamily {
	return InboundEventFamily(strings.ToLower(strings.TrimSpace(string(f))))
}

// Validate reports whether the inbound event family is supported.
func (f InboundEventFamily) Validate() error {
	switch f.Normalize() {
	case InboundEventFamilyMessage, InboundEventFamilyCommand, InboundEventFamilyAction,
		InboundEventFamilyReaction, InboundEventFamilyEdit:
		return nil
	case "":
		return errors.New("bridges: inbound event family is required")
	default:
		return fmt.Errorf("bridges: unsupported inbound event family %q", strings.TrimSpace(string(f)))
	}
}

// InboundCommand captures a slash-command style interaction.
type InboundCommand struct {
	Command   string `json:"command"`
	Text      string `json:"text,omitempty"`
	TriggerID string `json:"trigger_id,omitempty"`
}

// Validate reports whether the command identifies its action.
func (c InboundCommand) Validate() error {
	return requireField(strings.TrimSpace(c.Command), "inbound command")
}

func (c InboundCommand) normalize() InboundCommand {
	return InboundCommand{
		Command: strings.TrimSpace(c.Command), Text: strings.TrimSpace(c.Text),
		TriggerID: strings.TrimSpace(c.TriggerID),
	}
}

// InboundAction captures a button/action interaction.
type InboundAction struct {
	ActionID  string `json:"action_id"`
	MessageID string `json:"message_id,omitempty"`
	Value     string `json:"value,omitempty"`
	TriggerID string `json:"trigger_id,omitempty"`
}

// Validate reports whether the action identifies its action.
func (a InboundAction) Validate() error {
	return requireField(strings.TrimSpace(a.ActionID), "inbound action id")
}

func (a InboundAction) normalize() InboundAction {
	return InboundAction{
		ActionID: strings.TrimSpace(a.ActionID), MessageID: strings.TrimSpace(a.MessageID),
		Value: strings.TrimSpace(a.Value), TriggerID: strings.TrimSpace(a.TriggerID),
	}
}

// InboundReaction captures a reaction add/remove interaction.
type InboundReaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
	RawEmoji  string `json:"raw_emoji,omitempty"`
	Added     bool   `json:"added"`
}

// Validate reports whether the reaction identifies its message and emoji.
func (r InboundReaction) Validate() error {
	if err := requireField(strings.TrimSpace(r.MessageID), "inbound reaction message id"); err != nil {
		return err
	}
	return requireField(strings.TrimSpace(r.Emoji), "inbound reaction emoji")
}

func (r InboundReaction) normalize() InboundReaction {
	return InboundReaction{
		MessageID: strings.TrimSpace(r.MessageID), Emoji: strings.TrimSpace(r.Emoji),
		RawEmoji: strings.TrimSpace(r.RawEmoji), Added: r.Added,
	}
}

// InboundEditOperation distinguishes replacement from deletion.
type InboundEditOperation string

const (
	InboundEditOperationUpdated InboundEditOperation = "updated"
	InboundEditOperationDeleted InboundEditOperation = "deleted"
)

// InboundEditOperationValues returns the closed wire values in stable order.
func InboundEditOperationValues() []string {
	return []string{string(InboundEditOperationUpdated), string(InboundEditOperationDeleted)}
}

// Normalize returns the canonical edit operation.
func (o InboundEditOperation) Normalize() InboundEditOperation {
	return InboundEditOperation(strings.ToLower(strings.TrimSpace(string(o))))
}

// Validate reports whether the edit operation is supported.
func (o InboundEditOperation) Validate() error {
	switch o.Normalize() {
	case InboundEditOperationUpdated, InboundEditOperationDeleted:
		return nil
	case "":
		return errors.New("bridges: inbound edit operation is required")
	default:
		return fmt.Errorf("bridges: unsupported inbound edit operation %q", strings.TrimSpace(string(o)))
	}
}

// InboundEdit captures an update or deletion of an existing message.
type InboundEdit struct {
	MessageID         string               `json:"message_id"`
	NewText           string               `json:"new_text"`
	OriginalTimestamp time.Time            `json:"original_timestamp"`
	Operation         InboundEditOperation `json:"operation"`
}

// Validate reports whether the edit carries an unambiguous payload.
func (e InboundEdit) Validate() error {
	normalized := e.normalize()
	if err := requireField(normalized.MessageID, "inbound edit message id"); err != nil {
		return err
	}
	if normalized.OriginalTimestamp.IsZero() {
		return errors.New("bridges: inbound edit original timestamp is required")
	}
	if err := normalized.Operation.Validate(); err != nil {
		return err
	}
	switch normalized.Operation {
	case InboundEditOperationUpdated:
		return requireField(normalized.NewText, "inbound edit new text")
	case InboundEditOperationDeleted:
		if normalized.NewText != "" {
			return errors.New("bridges: inbound deleted message cannot include new text")
		}
		return nil
	default:
		return errors.New("bridges: inbound edit operation is required")
	}
}

func (e InboundEdit) normalize() InboundEdit {
	e.MessageID = strings.TrimSpace(e.MessageID)
	e.NewText = strings.TrimSpace(e.NewText)
	e.Operation = e.Operation.Normalize()
	if !e.OriginalTimestamp.IsZero() {
		e.OriginalTimestamp = e.OriginalTimestamp.UTC()
	}
	return e
}
