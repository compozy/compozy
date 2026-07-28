package contract

import (
	"errors"
	"strings"
)

func (e DeliveryEvent) validateTypedFields() error {
	switch e.EventType {
	case DeliveryEventTypeError:
		return e.validateErrorTypedFields()
	case DeliveryEventTypeResume:
		return e.validateResumeTypedFields()
	case DeliveryEventTypeDelete:
		return e.validateDeleteTypedFields()
	case DeliveryEventTypeProgress:
		return e.validateProgressTypedFields()
	default:
		return e.validateAbsentTypedFields()
	}
}

func (e DeliveryEvent) validateErrorTypedFields() error {
	if e.Error == nil {
		return errors.New("bridges: delivery error events require an error payload")
	}
	if err := e.Error.Validate(); err != nil {
		return err
	}
	if e.Resume != nil || e.Progress != nil {
		return errors.New("bridges: delivery error events cannot include resume or progress payloads")
	}
	return nil
}

func (e DeliveryEvent) validateResumeTypedFields() error {
	if e.Resume == nil {
		return errors.New("bridges: delivery resume events require a resume payload")
	}
	if err := e.Resume.Validate(); err != nil {
		return err
	}
	if e.Error != nil || e.Progress != nil {
		return errors.New("bridges: delivery resume events cannot include error or progress payloads")
	}
	return nil
}

func (e DeliveryEvent) validateDeleteTypedFields() error {
	if strings.TrimSpace(e.Content.Text) != "" {
		return errors.New("bridges: delivery delete events cannot include message content")
	}
	if e.Error != nil || e.Resume != nil || e.Progress != nil {
		return errors.New("bridges: delivery delete events cannot include typed payloads")
	}
	return nil
}

func (e DeliveryEvent) validateProgressTypedFields() error {
	if e.Progress == nil {
		return errors.New("bridges: delivery progress events require a progress payload")
	}
	if err := e.Progress.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(e.Content.Text) != "" {
		return errors.New("bridges: delivery progress events cannot include message content")
	}
	if e.Error != nil || e.Resume != nil {
		return errors.New("bridges: delivery progress events cannot include error or resume payloads")
	}
	return nil
}

func (e DeliveryEvent) validateAbsentTypedFields() error {
	if e.Error != nil {
		return errors.New("bridges: only delivery error events may include error payload")
	}
	if e.Resume != nil {
		return errors.New("bridges: only delivery resume events may include resume payload")
	}
	if e.Progress != nil {
		return errors.New("bridges: only delivery progress events may include progress payload")
	}
	return nil
}
