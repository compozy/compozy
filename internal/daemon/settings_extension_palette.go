package daemon

import (
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/windowmanager"
)

type extensionCmdPaletteSettingsRuntime interface {
	CmdPaletteSettings(string, extensionpkg.ProfileLens) (extensionpkg.CmdPaletteProjection, error)
}

func (s *settingsRuntimeSurface) attachExtensionPaletteSettings(
	installed []settingspkg.InstalledExtension,
) error {
	if s == nil || s.extensionRuntime == nil {
		return nil
	}
	runtime, ok := s.extensionRuntime().(extensionCmdPaletteSettingsRuntime)
	if !ok || runtime == nil {
		return nil
	}
	projection, err := runtime.CmdPaletteSettings("", extensionpkg.ProfileLens{
		ID:   string(cmdpalette.DefaultProfileLensID),
		Name: "default",
	})
	if err != nil {
		return fmt.Errorf("daemon: project extension palette settings: %w", err)
	}
	bindable := windowmanager.DefaultBindableIDs()
	for _, command := range projection.Commands {
		bindable[command.ID] = struct{}{}
	}
	claims := make([]windowmanager.ExtensionDefaultShortcut, 0, len(projection.Defaults))
	for _, item := range projection.Defaults {
		claims = append(claims, windowmanager.ExtensionDefaultShortcut{
			CommandID: item.CommandID, Chord: item.Chord,
			Source: (cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: item.Extension}).ID(),
			Active: item.Active,
		})
	}
	effective, statuses, _, err := windowmanager.TolerantEffectiveKeymapWithExtensionDefaults(
		s.config.WindowManager.Shortcuts, bindable, claims,
	)
	if err != nil {
		return fmt.Errorf("daemon: resolve extension palette settings shortcuts: %w", err)
	}
	statusByCommand := make(map[string]windowmanager.ExtensionDefaultStatus, len(statuses))
	for _, status := range statuses {
		statusByCommand[status.CommandID] = status
	}
	byExtension := extensionPaletteSettingsByName(projection, effective, statusByCommand)
	for index := range installed {
		installed[index].Palette = byExtension[installed[index].Name]
	}
	return nil
}

func extensionPaletteSettingsByName(
	projection extensionpkg.CmdPaletteProjection,
	effective map[string]windowmanager.ShortcutBinding,
	statuses map[string]windowmanager.ExtensionDefaultStatus,
) map[string]*settingspkg.InstalledExtensionPalette {
	result := make(map[string]*settingspkg.InstalledExtensionPalette)
	for _, command := range projection.Commands {
		palette := extensionPaletteSettings(result, command.Extension)
		status, hasDefault := statuses[command.ID]
		item := settingspkg.InstalledExtensionPaletteCommand{
			ID: command.ID, Title: command.Title,
			Bindings:  append([]string(nil), effective[command.ID]...),
			Available: command.UnavailableReason == "", Reason: command.UnavailableReason,
		}
		if hasDefault && len(status.Binding) > 0 {
			item.DefaultBinding = status.Binding[0]
			item.DefaultDormant = status.Dormant
			item.ConflictWith = status.ConflictWith
		}
		palette.Commands = append(palette.Commands, item)
	}
	for _, view := range projection.Views {
		palette := extensionPaletteSettings(result, view.Extension)
		palette.Views = append(palette.Views, settingspkg.InstalledExtensionPaletteView{
			ID: view.ID, Title: view.Title,
			Available: view.UnavailableReason == "", Reason: view.UnavailableReason,
		})
	}
	for _, palette := range result {
		sort.Slice(palette.Commands, func(i, j int) bool {
			return palette.Commands[i].ID < palette.Commands[j].ID
		})
		sort.Slice(palette.Views, func(i, j int) bool {
			return palette.Views[i].ID < palette.Views[j].ID
		})
	}
	return result
}

func extensionPaletteSettings(
	byExtension map[string]*settingspkg.InstalledExtensionPalette,
	name string,
) *settingspkg.InstalledExtensionPalette {
	if palette := byExtension[name]; palette != nil {
		return palette
	}
	palette := &settingspkg.InstalledExtensionPalette{
		Commands: []settingspkg.InstalledExtensionPaletteCommand{},
		Views:    []settingspkg.InstalledExtensionPaletteView{},
	}
	byExtension[name] = palette
	return palette
}
