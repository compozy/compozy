package main

import (
	"errors"
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/testutil/acpmock"
)

// configOptionSet is one advertised option set together with the values its fixture
// scripts to fail, so a reannounced set keeps its own rejection metadata.
type configOptionSet struct {
	options []acpsdk.SessionConfigOption
	rejects map[string]struct{}
}

func sessionConfigOptionsFromFixture(
	options []acpmock.SessionConfigOptionFixture,
) []acpsdk.SessionConfigOption {
	if len(options) == 0 {
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

// configRejectedValues indexes the values an option set scripts to fail on
// session/set_config_option.
func configRejectedValues(options []acpmock.SessionConfigOptionFixture) map[string]struct{} {
	var rejects map[string]struct{}
	for _, option := range options {
		for _, value := range option.Values {
			if !value.RejectSet {
				continue
			}
			if rejects == nil {
				rejects = make(map[string]struct{})
			}
			rejects[configReannouncementKey(option.ID, value.Value)] = struct{}{}
		}
	}
	return rejects
}

// sessionConfigReannouncements indexes each value's replacement option set, including
// the nested sets a reannounced selection can itself trigger. Fixture validation rejects
// two different replacements for the same option value, so the index is unambiguous.
func sessionConfigReannouncements(
	options []acpmock.SessionConfigOptionFixture,
) map[string]configOptionSet {
	reannouncements := make(map[string]configOptionSet)
	collectSessionConfigReannouncements(options, reannouncements)
	if len(reannouncements) == 0 {
		return nil
	}
	return reannouncements
}

func collectSessionConfigReannouncements(
	options []acpmock.SessionConfigOptionFixture,
	reannouncements map[string]configOptionSet,
) {
	for _, option := range options {
		for _, value := range option.Values {
			if len(value.WhenSelected) == 0 {
				continue
			}
			key := configReannouncementKey(option.ID, value.Value)
			if _, indexed := reannouncements[key]; !indexed {
				reannouncements[key] = configOptionSet{
					options: sessionConfigOptionsFromFixture(value.WhenSelected),
					rejects: configRejectedValues(value.WhenSelected),
				}
			}
			collectSessionConfigReannouncements(value.WhenSelected, reannouncements)
		}
	}
}

func configReannouncementKey(configID string, value string) string {
	return strings.TrimSpace(configID) + "\x00" + strings.TrimSpace(value)
}

// newSessionState starts a session from the advertised template and its rejection rules.
// Callers hold a.mu.
func (a *mockAgent) newSessionState() *sessionState {
	return &sessionState{
		ConfigOptions:        cloneSessionConfigOptions(a.configTemplate),
		rejectedConfigValues: configRejectedValues(a.agent.ConfigOptions),
	}
}

// rejectsConfigValue reports whether the session's active option set scripts a
// set_config_option failure for the value. A reannounced set carries its own rules.
func (a *mockAgent) rejectsConfigValue(sessionID string, configID string, value string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	rejects := configRejectedValues(a.agent.ConfigOptions)
	if session := a.sessions[strings.TrimSpace(sessionID)]; session != nil {
		rejects = session.rejectedConfigValues
	}
	_, rejected := rejects[configReannouncementKey(configID, value)]
	return rejected
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
				reannounced := a.configReannounce[configReannouncementKey(trimmedConfigID, trimmedValue)]
				if len(reannounced.options) > 0 {
					session.ConfigOptions = cloneSessionConfigOptions(reannounced.options)
					session.rejectedConfigValues = reannounced.rejects
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
	if len(options) == 0 {
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
