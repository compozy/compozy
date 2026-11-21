package main

import (
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func sessionConfigOptionsFromFixture(
	options []acpmock.SessionConfigOptionFixture,
) []acpsdk.SessionConfigOption {
	// An explicit empty set advertises no controls, which ACP encodes as [] rather than null.
	if options == nil {
		return nil
	}
	result := make([]acpsdk.SessionConfigOption, 0, len(options))
	for _, option := range options {
		var category *acpsdk.SessionConfigOptionCategory
		if value := strings.TrimSpace(option.Category); value != "" {
			parsed := acpsdk.SessionConfigOptionCategory(value)
			category = &parsed
		}
		values := make(acpsdk.SessionConfigSelectOptionsUngrouped, 0, len(option.Values))
		for _, value := range option.Values {
			label := strings.TrimSpace(value.Label)
			if label == "" {
				label = strings.TrimSpace(value.Value)
			}
			values = append(values, acpsdk.SessionConfigSelectOption{
				Name:  label,
				Value: acpsdk.SessionConfigValueId(strings.TrimSpace(value.Value)),
			})
		}
		result = append(result, acpsdk.SessionConfigOption{
			Select: &acpsdk.SessionConfigOptionSelect{
				Id:           acpsdk.SessionConfigId(strings.TrimSpace(option.ID)),
				Name:         strings.TrimSpace(option.Name),
				Category:     category,
				CurrentValue: acpsdk.SessionConfigValueId(strings.TrimSpace(option.Current)),
				Options: acpsdk.SessionConfigSelectOptions{
					Ungrouped: &values,
				},
				Type: "select",
			},
		})
	}
	return result
}

// newSessionState starts a session from the advertised template and its fixture option set.
// Callers hold a.mu.
func (a *mockAgent) newSessionState() *sessionState {
	return &sessionState{
		ConfigOptions: cloneSessionConfigOptions(a.configTemplate),
		activeConfig:  a.agent.ConfigOptions,
	}
}

// rejectsConfigValue reports whether the session's active option set scripts a
// set_config_option failure for the value. A reannounced set carries its own rules.
func (a *mockAgent) rejectsConfigValue(sessionID string, configID string, value string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	active := a.agent.ConfigOptions
	if session := a.sessions[strings.TrimSpace(sessionID)]; session != nil {
		active = session.activeConfig
	}
	for _, option := range active {
		if strings.TrimSpace(option.ID) != strings.TrimSpace(configID) {
			continue
		}
		for _, candidate := range option.Values {
			if candidate.RejectSet && strings.TrimSpace(candidate.Value) == strings.TrimSpace(value) {
				return true
			}
		}
	}
	return false
}

// configReplacement returns the option set that selecting the value reannounces. The
// active set's own when_selected wins, so a value shared by several branches keeps each
// branch's replacement; a value advertised without one uses the tree-wide definition.
func (a *mockAgent) configReplacement(
	active []acpmock.SessionConfigOptionFixture,
	configID string,
	value string,
) ([]acpmock.SessionConfigOptionFixture, bool) {
	for _, option := range active {
		if strings.TrimSpace(option.ID) != configID {
			continue
		}
		for _, candidate := range option.Values {
			if strings.TrimSpace(candidate.Value) == value && candidate.WhenSelected != nil {
				return candidate.WhenSelected, true
			}
		}
	}
	replacement, ok := a.configReannounce[acpmock.ConfigReannouncementKey(configID, value)]
	return replacement, ok
}

func (a *mockAgent) setConfigOptionValue(sessionID string, configID string, value string) error {
	trimmedSessionID := strings.TrimSpace(sessionID)
	trimmedConfigID := strings.TrimSpace(configID)
	trimmedValue := strings.TrimSpace(value)
	if trimmedSessionID == "" {
		return errors.New("acpmock-driver: session id is required")
	}
	if trimmedConfigID == "" {
		return errors.New("acpmock-driver: session config option id is required")
	}
	if trimmedValue == "" {
		return errors.New("acpmock-driver: session config option value is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	session := a.sessions[trimmedSessionID]
	if session == nil {
		session = a.newSessionState()
		a.sessions[trimmedSessionID] = session
	}
	for idx := range session.ConfigOptions {
		option := session.ConfigOptions[idx].Select
		if option == nil || string(option.Id) != trimmedConfigID {
			continue
		}
		if option.Options.Ungrouped == nil {
			return fmt.Errorf("acpmock-driver: config option %q has no selectable values", trimmedConfigID)
		}
		for _, candidate := range *option.Options.Ungrouped {
			if string(candidate.Value) == trimmedValue {
				option.CurrentValue = acpsdk.SessionConfigValueId(trimmedValue)
				replacement, reannounced := a.configReplacement(session.activeConfig, trimmedConfigID, trimmedValue)
				if reannounced {
					session.ConfigOptions = sessionConfigOptionsFromFixture(replacement)
					session.activeConfig = replacement
				}
				return nil
			}
		}
		return fmt.Errorf(
			"acpmock-driver: config option %q value %q is not available",
			trimmedConfigID,
			trimmedValue,
		)
	}
	return fmt.Errorf("acpmock-driver: config option %q is not available", trimmedConfigID)
}

func (a *mockAgent) ensureSessionState(sessionID string) *sessionState {
	trimmedSessionID := strings.TrimSpace(sessionID)
	a.mu.Lock()
	defer a.mu.Unlock()
	session := a.sessions[trimmedSessionID]
	if session == nil {
		session = a.newSessionState()
		a.sessions[trimmedSessionID] = session
	}
	return session
}

func (a *mockAgent) sessionConfigOptions(sessionID string) []acpsdk.SessionConfigOption {
	session := a.ensureSessionState(sessionID)
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneSessionConfigOptions(session.ConfigOptions)
}

func cloneSessionConfigOptions(options []acpsdk.SessionConfigOption) []acpsdk.SessionConfigOption {
	if options == nil {
		return nil
	}
	cloned := make([]acpsdk.SessionConfigOption, 0, len(options))
	for _, option := range options {
		if option.Select != nil {
			selectCopy := *option.Select
			if option.Select.Options.Ungrouped != nil {
				values := append(acpsdk.SessionConfigSelectOptionsUngrouped(nil), (*option.Select.Options.Ungrouped)...)
				selectCopy.Options.Ungrouped = &values
			}
			cloned = append(cloned, acpsdk.SessionConfigOption{Select: &selectCopy})
			continue
		}
		if option.Boolean != nil {
			booleanCopy := *option.Boolean
			cloned = append(cloned, acpsdk.SessionConfigOption{Boolean: &booleanCopy})
		}
	}
	return cloned
}
