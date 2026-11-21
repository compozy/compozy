package acpmock

import (
	"fmt"
	"reflect"
	"strings"
)

// A value's when_selected belongs to the option set that advertises it: selecting the
// value replaces the active set with that definition. An explicit empty when_selected
// replaces the advertised controls with none. A value advertised without when_selected
// reuses the tree-wide definition for the same option id and value, which must therefore
// be unambiguous.

// configReannouncementSite records one when_selected definition and where it is scripted.
type configReannouncementSite struct {
	path    string
	options []SessionConfigOptionFixture
}

// configReannouncementFallback records one advertised value without its own when_selected.
type configReannouncementFallback struct {
	key   string
	path  string
	id    string
	value string
}

// ConfigReannouncementKey identifies one option value across a config option tree.
func ConfigReannouncementKey(configID string, value string) string {
	return strings.TrimSpace(configID) + "\x00" + strings.TrimSpace(value)
}

// ConfigReannouncementIndex indexes the tree-wide when_selected definition of each option
// value. A value whose definitions are not equivalent is left out: only the option sets
// that define it can reannounce it.
func ConfigReannouncementIndex(options []SessionConfigOptionFixture) map[string][]SessionConfigOptionFixture {
	definitions := make(map[string][]configReannouncementSite)
	collectConfigReannouncementSites(options, "config_options", definitions, nil)
	index := make(map[string][]SessionConfigOptionFixture, len(definitions))
	for key, sites := range definitions {
		if conflictingConfigReannouncementSite(sites) < 0 {
			index[key] = sites[0].options
		}
	}
	if len(index) == 0 {
		return nil
	}
	return index
}

// EquivalentConfigOptions reports whether two option sets advertise the same controls once
// the driver normalizes them: trimmed ids, names, categories, current values, and values,
// labels defaulting to the value, the same reject_set flags, and equivalent nested
// replacements. An explicit empty replacement differs from an omitted one.
func EquivalentConfigOptions(left []SessionConfigOptionFixture, right []SessionConfigOptionFixture) bool {
	return reflect.DeepEqual(normalizedConfigOptions(left), normalizedConfigOptions(right))
}

// validateConfigReannouncements rejects a value advertised without its own when_selected
// while the tree scripts different replacements for it, because the driver could not tell
// which replacement that selection reannounces.
func validateConfigReannouncements(options []SessionConfigOptionFixture, path string) error {
	definitions := make(map[string][]configReannouncementSite)
	var fallbacks []configReannouncementFallback
	collectConfigReannouncementSites(options, path, definitions, &fallbacks)
	for _, fallback := range fallbacks {
		sites := definitions[fallback.key]
		if conflict := conflictingConfigReannouncementSite(sites); conflict >= 0 {
			return fmt.Errorf(
				"acpmock: %s advertises %s=%q without when_selected, but %s and %s script different replacements",
				fallback.path, fallback.id, fallback.value, sites[0].path, sites[conflict].path,
			)
		}
	}
	return nil
}

func collectConfigReannouncementSites(
	options []SessionConfigOptionFixture,
	path string,
	definitions map[string][]configReannouncementSite,
	fallbacks *[]configReannouncementFallback,
) {
	for optionIdx, option := range options {
		for valueIdx, value := range option.Values {
			valuePath := fmt.Sprintf("%s[%d].values[%d]", path, optionIdx, valueIdx)
			key := ConfigReannouncementKey(option.ID, value.Value)
			if value.WhenSelected == nil {
				if fallbacks != nil {
					*fallbacks = append(*fallbacks, configReannouncementFallback{
						key:   key,
						path:  valuePath,
						id:    strings.TrimSpace(option.ID),
						value: strings.TrimSpace(value.Value),
					})
				}
				continue
			}
			sitePath := valuePath + ".when_selected"
			definitions[key] = append(definitions[key], configReannouncementSite{
				path:    sitePath,
				options: value.WhenSelected,
			})
			collectConfigReannouncementSites(value.WhenSelected, sitePath, definitions, fallbacks)
		}
	}
}

// conflictingConfigReannouncementSite returns the index of the first definition that is not
// equivalent to the first one, or -1 when every definition matches.
func conflictingConfigReannouncementSite(sites []configReannouncementSite) int {
	for idx := 1; idx < len(sites); idx++ {
		if !EquivalentConfigOptions(sites[0].options, sites[idx].options) {
			return idx
		}
	}
	return -1
}

func normalizedConfigOptions(options []SessionConfigOptionFixture) []SessionConfigOptionFixture {
	if options == nil {
		return nil
	}
	normalized := make([]SessionConfigOptionFixture, 0, len(options))
	for _, option := range options {
		values := make([]SessionConfigOptionValueFixture, 0, len(option.Values))
		for _, value := range option.Values {
			trimmed := strings.TrimSpace(value.Value)
			label := strings.TrimSpace(value.Label)
			if label == "" {
				label = trimmed
			}
			values = append(values, SessionConfigOptionValueFixture{
				Value:        trimmed,
				Label:        label,
				RejectSet:    value.RejectSet,
				WhenSelected: normalizedConfigOptions(value.WhenSelected),
			})
		}
		normalized = append(normalized, SessionConfigOptionFixture{
			ID:       strings.TrimSpace(option.ID),
			Name:     strings.TrimSpace(option.Name),
			Category: strings.TrimSpace(option.Category),
			Current:  strings.TrimSpace(option.Current),
			Values:   values,
		})
	}
	return normalized
}
