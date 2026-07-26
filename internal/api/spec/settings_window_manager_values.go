package spec

import (
	"reflect"

	"github.com/compozy/agh/internal/api/contract"
)

func withSettingsWindowManagerSchemaEnumValues(
	values map[reflect.Type][]string,
) map[reflect.Type][]string {
	values[reflect.TypeFor[contract.SettingsWindowNewPolicy]()] =
		settingsWindowNewPolicyValues()
	values[reflect.TypeFor[contract.SettingsWindowSmallViewportPolicy]()] =
		settingsWindowSmallViewportPolicyValues()
	values[reflect.TypeFor[contract.SettingsWindowFocusPolicy]()] =
		settingsWindowFocusPolicyValues()
	values[reflect.TypeFor[contract.SettingsWindowDragAwayPolicy]()] =
		settingsWindowDragAwayPolicyValues()
	values[reflect.TypeFor[contract.SettingsWindowDragModifier]()] =
		settingsWindowDragModifierValues()
	values[reflect.TypeFor[contract.SettingsWindowDesktopTransition]()] =
		settingsWindowDesktopTransitionValues()
	values[reflect.TypeFor[contract.SettingsWindowBindingAction]()] =
		settingsWindowBindingActionValues()
	return values
}

func settingsWindowNewPolicyValues() []string {
	return []string{
		string(contract.SettingsWindowNewPolicyFloating),
		string(contract.SettingsWindowNewPolicyBesideFocus),
	}
}

func settingsWindowSmallViewportPolicyValues() []string {
	return []string{
		string(contract.SettingsWindowSmallViewportPolicyStack),
		string(contract.SettingsWindowSmallViewportPolicyReject),
	}
}

func settingsWindowFocusPolicyValues() []string {
	return []string{
		string(contract.SettingsWindowFocusPolicyClickDirectional),
		string(contract.SettingsWindowFocusPolicyDirectional),
	}
}

func settingsWindowDragAwayPolicyValues() []string {
	return []string{
		string(contract.SettingsWindowDragAwayPolicyWindow),
		string(contract.SettingsWindowDragAwayPolicyGroup),
	}
}

func settingsWindowDragModifierValues() []string {
	return []string{
		string(contract.SettingsWindowDragModifierAlt),
		string(contract.SettingsWindowDragModifierControl),
		string(contract.SettingsWindowDragModifierMeta),
		string(contract.SettingsWindowDragModifierShift),
		string(contract.SettingsWindowDragModifierNone),
	}
}

func settingsWindowDesktopTransitionValues() []string {
	return []string{
		string(contract.SettingsWindowDesktopTransitionSlide),
		string(contract.SettingsWindowDesktopTransitionCrossfade),
		string(contract.SettingsWindowDesktopTransitionInstant),
	}
}

func settingsWindowBindingActionValues() []string {
	return []string{
		string(contract.SettingsWindowBindingActionNone),
		string(contract.SettingsWindowBindingActionReserved),
		string(contract.SettingsWindowBindingActionZoom),
	}
}
