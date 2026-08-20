package cmdpalette

import (
	"errors"
	"fmt"
	"strings"
)

func validateViewFrame(kind ViewKind, frame ViewFrame) (ViewFrame, error) {
	if strings.TrimSpace(frame.ViewSession) == "" {
		return ViewFrame{}, errors.New("cmd palette view: frame view_session is required")
	}
	if strings.TrimSpace(frame.Revision) == "" {
		return ViewFrame{}, errors.New("cmd palette view: frame revision is required")
	}
	if (frame.Payload == nil) == (frame.Patch == nil) {
		return ViewFrame{}, errors.New("cmd palette view: frame requires exactly one of payload or patch")
	}
	validated := cloneViewFrame(frame)
	if frame.Payload != nil {
		payload, err := ValidateViewPayload(kind, *frame.Payload, nil, nil)
		if err != nil {
			return ViewFrame{}, err
		}
		validated.Payload = &payload
	}
	if frame.Patch != nil {
		if err := ValidateViewPatch(*frame.Patch); err != nil {
			return ViewFrame{}, err
		}
	}
	seenHandlers := make(map[string]struct{}, len(frame.Handlers))
	for index, handler := range frame.Handlers {
		handler = strings.TrimSpace(handler)
		if handler == "" {
			return ViewFrame{}, fmt.Errorf("cmd palette view: handlers[%d] is required", index)
		}
		if _, exists := seenHandlers[handler]; exists {
			return ViewFrame{}, fmt.Errorf("cmd palette view: handlers[%d] duplicates %q", index, handler)
		}
		seenHandlers[handler] = struct{}{}
		validated.Handlers[index] = handler
	}
	seenEffects := make(map[string]struct{}, len(frame.Effects))
	for index, effect := range frame.Effects {
		effect.ID = strings.TrimSpace(effect.ID)
		if err := validateViewEffect(effect); err != nil {
			return ViewFrame{}, fmt.Errorf("cmd palette view: effects[%d]: %w", index, err)
		}
		if _, exists := seenEffects[effect.ID]; exists {
			return ViewFrame{}, fmt.Errorf("cmd palette view: effects[%d] duplicates %q", index, effect.ID)
		}
		seenEffects[effect.ID] = struct{}{}
		validated.Effects[index] = effect
	}
	return validated, nil
}

func validateViewEffect(effect Effect) error {
	if effect.ID == "" {
		return errors.New("id is required")
	}
	members := 0
	for _, set := range []bool{
		effect.Toast != nil,
		effect.Copy != nil,
		effect.OpenURL != nil,
		effect.OpenApp != nil,
		effect.PickFiles != nil,
	} {
		if set {
			members++
		}
	}
	if members != 1 {
		return errors.New("exactly one effect member is required")
	}
	if effect.OpenURL != nil {
		if err := validateHTTPURL(effect.OpenURL.URL); err != nil {
			return err
		}
	}
	return nil
}

func cloneViewFrame(frame ViewFrame) ViewFrame {
	cloned := frame
	cloned.Handlers = append([]string(nil), frame.Handlers...)
	cloned.Effects = cloneViewEffects(frame.Effects)
	if frame.Payload != nil {
		payload := cloneViewPayload(*frame.Payload)
		cloned.Payload = &payload
	}
	if frame.Patch != nil {
		patch := *frame.Patch
		patch.Ops = clonePatchOps(frame.Patch.Ops)
		cloned.Patch = &patch
	}
	return cloned
}
