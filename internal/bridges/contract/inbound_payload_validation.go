package contract

import (
	"errors"
	"fmt"
	"strings"
)

func (e InboundMessageEnvelope) validatePayload() error {
	if err := e.validateReplyContext(); err != nil {
		return err
	}
	switch e.EventFamily {
	case InboundEventFamilyMessage:
		return e.validateMessagePayload()
	case InboundEventFamilyCommand:
		return e.validateCommandPayload()
	case InboundEventFamilyAction:
		return e.validateActionPayload()
	case InboundEventFamilyReaction:
		return e.validateReactionPayload()
	case InboundEventFamilyEdit:
		return e.validateEditPayload()
	default:
		return errors.New("bridges: inbound event family is required")
	}
}

func (e InboundMessageEnvelope) validateMessagePayload() error {
	if e.Command != nil || e.Action != nil || e.Reaction != nil || e.Edit != nil {
		return errors.New("bridges: inbound message family cannot include typed interaction payloads")
	}
	return requireField(e.PlatformMessageID, "inbound message platform message id")
}

func (e InboundMessageEnvelope) validateCommandPayload() error {
	if e.Command == nil {
		return errors.New("bridges: inbound command family requires command payload")
	}
	if e.Action != nil || e.Reaction != nil || e.Edit != nil {
		return errors.New("bridges: inbound command family cannot include action, reaction, or edit payloads")
	}
	if err := e.validateMessageFieldsAbsent("command"); err != nil {
		return err
	}
	return e.Command.Validate()
}

func (e InboundMessageEnvelope) validateActionPayload() error {
	if e.Action == nil {
		return errors.New("bridges: inbound action family requires action payload")
	}
	if e.Command != nil || e.Reaction != nil || e.Edit != nil {
		return errors.New("bridges: inbound action family cannot include command, reaction, or edit payloads")
	}
	if err := e.validateMessageFieldsAbsent("action"); err != nil {
		return err
	}
	return e.Action.Validate()
}

func (e InboundMessageEnvelope) validateReactionPayload() error {
	if e.Reaction == nil {
		return errors.New("bridges: inbound reaction family requires reaction payload")
	}
	if e.Command != nil || e.Action != nil || e.Edit != nil {
		return errors.New("bridges: inbound reaction family cannot include command, action, or edit payloads")
	}
	if err := e.validateMessageFieldsAbsent("reaction"); err != nil {
		return err
	}
	return e.Reaction.Validate()
}

func (e InboundMessageEnvelope) validateEditPayload() error {
	if e.Edit == nil {
		return errors.New("bridges: inbound edit family requires edit payload")
	}
	if e.Command != nil || e.Action != nil || e.Reaction != nil {
		return errors.New("bridges: inbound edit family cannot include command, action, or reaction payloads")
	}
	if err := e.validateMessageFieldsAbsent("edit"); err != nil {
		return err
	}
	return e.Edit.Validate()
}

func (e InboundMessageEnvelope) validateReplyContext() error {
	if e.ReplyToText == "" && e.ReplyToAuthorID == "" && e.ReplyToAuthorName == "" {
		return nil
	}
	if e.EventFamily != InboundEventFamilyMessage && e.EventFamily != InboundEventFamilyEdit {
		return fmt.Errorf("bridges: inbound %s family cannot include reply context", e.EventFamily)
	}
	return nil
}

func (e InboundMessageEnvelope) validateMessageFieldsAbsent(family string) error {
	if e.PlatformMessageID != "" || strings.TrimSpace(e.Content.Text) != "" || len(e.Attachments) > 0 {
		return fmt.Errorf("bridges: inbound %s family cannot include message payload fields", family)
	}
	return nil
}
