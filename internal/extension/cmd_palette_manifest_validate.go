package extensionpkg

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	cmdPaletteActionNavigate       = "navigate"
	cmdPaletteActionTool           = "tool"
	cmdPaletteActionURL            = "url"
	cmdPaletteActionView           = "view"
	cmdPaletteArgumentCheckbox     = "checkbox"
	cmdPaletteArgumentDropdown     = "dropdown"
	cmdPaletteArgumentPassword     = "password"
	cmdPaletteArgumentText         = "text"
	cmdPaletteViewDetail           = "detail"
	cmdPaletteViewForm             = "form"
	cmdPaletteViewGrid             = "grid"
	cmdPaletteViewList             = "list"
	cmdPaletteTitleMaxRunes        = 256
	cmdPaletteSectionMaxRunes      = 128
	cmdPalettePlaceholderMaxRunes  = 256
	cmdPaletteKeywordMaxRunes      = 64
	cmdPaletteLocalIDMaxRunes      = 64
	cmdPaletteConfirmationMaxRunes = 1024
	cmdPaletteKeywordsMax          = 64
	cmdPaletteArgumentsMax         = 32
)

var (
	cmdPaletteLocalIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
	cmdPaletteIconPattern    = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

func validateManifestCmdPalette(manifest *Manifest) (map[string]ManifestToolDescriptor, error) {
	config := manifest.Resources.CmdPalette
	if len(config.Commands) == 0 && len(config.Views) == 0 {
		return nil, nil
	}
	tools, err := cmdPaletteToolsByName(manifest)
	if err != nil {
		return nil, err
	}
	viewIDs, err := validateCmdPaletteViews(manifest, tools)
	if err != nil {
		return nil, err
	}
	return tools, validateCmdPaletteCommands(config.Commands, tools, viewIDs)
}

func validateCmdPaletteViews(
	manifest *Manifest,
	tools map[string]ManifestToolDescriptor,
) (map[string]struct{}, error) {
	config := manifest.Resources.CmdPalette
	viewIDs := make(map[string]struct{}, len(config.Views))
	for index, view := range config.Views {
		path := fmt.Sprintf("cmd_palette.views[%d]", index)
		if err := validateCmdPaletteLocalID(path+".id", view.ID, viewIDs); err != nil {
			return nil, err
		}
		if err := validateCmdPaletteText(path+".title", view.Title, cmdPaletteTitleMaxRunes); err != nil {
			return nil, err
		}
		switch view.Kind {
		case cmdPaletteViewList, cmdPaletteViewDetail, cmdPaletteViewForm, cmdPaletteViewGrid:
		default:
			return nil, cmdPaletteManifestError(path+".kind", view.Kind, "must be list, detail, form, or grid")
		}
		if view.Program == (view.Source != nil) {
			return nil, cmdPaletteManifestError(path, "", "exactly one of source or program is required")
		}
		if view.Program && !slices.Contains(
			manifest.Capabilities.Provides,
			string(extensionprotocol.CapabilityProvideViewProvider),
		) {
			return nil, cmdPaletteManifestError(
				path+".program",
				"true",
				"requires the view.provider capability",
			)
		}
		if view.Source != nil {
			tool, exists := tools[view.Source.Tool]
			if !exists {
				return nil, cmdPaletteUnknownRef(path+".source.tool", "tool", view.Source.Tool)
			}
			if tool.Tool.Risk != toolspkg.RiskRead || !tool.Tool.ReadOnly || tool.Tool.Destructive ||
				tool.Tool.OpenWorld || tool.Tool.RequiresInteraction {
				return nil, cmdPaletteManifestError(
					path+".source.tool", view.Source.Tool, "view source tool must be read-only risk class",
				)
			}
		}
	}
	return viewIDs, nil
}

func validateCmdPaletteCommands(
	commands []CmdPaletteCommand,
	tools map[string]ManifestToolDescriptor,
	viewIDs map[string]struct{},
) error {
	commandIDs := make(map[string]struct{}, len(commands))
	for index, command := range commands {
		path := fmt.Sprintf("cmd_palette.commands[%d]", index)
		if err := validateCmdPaletteLocalID(path+".id", command.ID, commandIDs); err != nil {
			return err
		}
		if err := validateCmdPaletteCommand(path, command, tools, viewIDs); err != nil {
			return err
		}
	}
	return nil
}

func validateCmdPaletteCommand(
	path string,
	command CmdPaletteCommand,
	tools map[string]ManifestToolDescriptor,
	views map[string]struct{},
) error {
	if err := validateCmdPaletteText(path+".title", command.Title, cmdPaletteTitleMaxRunes); err != nil {
		return err
	}
	if command.Section != "" {
		if err := validateCmdPaletteText(path+".section", command.Section, cmdPaletteSectionMaxRunes); err != nil {
			return err
		}
	}
	if err := validateCmdPaletteIcon(path+".icon", command.Icon); err != nil {
		return err
	}
	if len(command.Keywords) > cmdPaletteKeywordsMax {
		return cmdPaletteManifestError(path+".keywords", "", "must contain at most 64 values")
	}
	for index, keyword := range command.Keywords {
		if err := validateCmdPaletteText(
			fmt.Sprintf("%s.keywords[%d]", path, index), keyword, cmdPaletteKeywordMaxRunes,
		); err != nil {
			return err
		}
	}
	if len(command.Arguments) > cmdPaletteArgumentsMax {
		return cmdPaletteManifestError(path+".arguments", "", "must contain at most 32 values")
	}
	if err := validateCmdPaletteArguments(path, command.Arguments); err != nil {
		return err
	}
	if err := validateCmdPaletteAction(path+".action", command.Action, tools, views); err != nil {
		return err
	}
	if command.Destructive {
		if command.Confirmation == nil {
			return cmdPaletteManifestError(path+".confirmation", "", "is required for destructive commands")
		}
		if err := validateCmdPaletteText(
			path+".confirmation.title", command.Confirmation.Title, cmdPaletteTitleMaxRunes,
		); err != nil {
			return err
		}
		if err := validateCmdPaletteText(
			path+".confirmation.confirm", command.Confirmation.Confirm, cmdPaletteKeywordMaxRunes,
		); err != nil {
			return err
		}
		if command.Confirmation.Body != "" {
			if err := validateCmdPaletteText(
				path+".confirmation.body", command.Confirmation.Body, cmdPaletteConfirmationMaxRunes,
			); err != nil {
				return err
			}
		}
	}
	if command.DefaultShortcut != "" {
		if _, err := windowmanager.CanonicalShortcutChord(command.DefaultShortcut); err != nil {
			return cmdPaletteManifestError(path+".default_shortcut", command.DefaultShortcut, err.Error())
		}
	}
	return nil
}

func validateCmdPaletteArguments(path string, arguments []CmdPaletteArgument) error {
	seen := make(map[string]struct{}, len(arguments))
	for index, argument := range arguments {
		argumentPath := fmt.Sprintf("%s.arguments[%d]", path, index)
		if err := validateCmdPaletteLocalID(argumentPath+".name", argument.Name, seen); err != nil {
			return err
		}
		switch argument.Type {
		case cmdPaletteArgumentText, cmdPaletteArgumentPassword, cmdPaletteArgumentCheckbox:
			if len(argument.Options) > 0 {
				return cmdPaletteManifestError(argumentPath+".options", "", "is allowed only for dropdown arguments")
			}
		case cmdPaletteArgumentDropdown:
			if len(argument.Options) == 0 {
				return cmdPaletteManifestError(argumentPath+".options", "", "must contain at least one value")
			}
			for optionIndex, option := range argument.Options {
				if err := validateCmdPaletteText(
					fmt.Sprintf("%s.options[%d]", argumentPath, optionIndex),
					option,
					cmdPalettePlaceholderMaxRunes,
				); err != nil {
					return err
				}
			}
		default:
			return cmdPaletteManifestError(
				argumentPath+".type", argument.Type, "must be text, password, dropdown, or checkbox",
			)
		}
		if argument.Placeholder != "" {
			if err := validateCmdPaletteText(
				argumentPath+".placeholder", argument.Placeholder, cmdPalettePlaceholderMaxRunes,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCmdPaletteAction(
	path string,
	action CmdPaletteAction,
	tools map[string]ManifestToolDescriptor,
	views map[string]struct{},
) error {
	targets := []struct {
		kind  string
		value string
	}{
		{cmdPaletteActionTool, action.Tool},
		{cmdPaletteActionView, action.View},
		{cmdPaletteActionNavigate, action.App},
		{cmdPaletteActionURL, action.URL},
	}
	target := ""
	knownKind := false
	for _, candidate := range targets {
		if candidate.kind != action.Kind {
			continue
		}
		knownKind = true
		target = candidate.value
		break
	}
	if !knownKind {
		return cmdPaletteManifestError(path+".kind", action.Kind, "must be tool, view, navigate, or url")
	}
	if target == "" {
		return cmdPaletteManifestError(path+"."+cmdPaletteActionTargetField(action.Kind), "", "value is required")
	}
	for _, candidate := range targets {
		if candidate.kind != action.Kind && candidate.value != "" {
			return cmdPaletteManifestError(
				path+"."+cmdPaletteActionTargetField(candidate.kind), candidate.value,
				fmt.Sprintf("is not allowed for %s actions", action.Kind),
			)
		}
	}
	switch action.Kind {
	case cmdPaletteActionTool:
		if _, exists := tools[action.Tool]; !exists {
			return cmdPaletteUnknownRef(path+".tool", "tool", action.Tool)
		}
	case cmdPaletteActionView:
		if _, exists := views[action.View]; !exists {
			return cmdPaletteUnknownRef(path+".view", "view", action.View)
		}
	case cmdPaletteActionURL:
		parsed, err := url.ParseRequestURI(action.URL)
		if err != nil || (parsed.Scheme != forgeURLSchemeHTTPS && parsed.Scheme != forgeURLSchemeHTTP) {
			return cmdPaletteManifestError(path+".url", action.URL, "must be an absolute http or https URL")
		}
	}
	return nil
}

func cmdPaletteActionTargetField(kind string) string {
	if kind == cmdPaletteActionNavigate {
		return "app"
	}
	return kind
}

func cmdPaletteToolsByName(manifest *Manifest) (map[string]ManifestToolDescriptor, error) {
	descriptors, err := ResolveManifestToolDescriptors(manifest)
	if err != nil {
		return nil, err
	}
	result := make(map[string]ManifestToolDescriptor, len(descriptors))
	for index := range descriptors {
		descriptor := &descriptors[index]
		result[descriptor.Name] = *descriptor
	}
	return result, nil
}

func validateCmdPaletteLocalID(field, value string, seen map[string]struct{}) error {
	if !cmdPaletteLocalIDPattern.MatchString(value) {
		return cmdPaletteManifestError(field, value, "must be a lowercase identifier")
	}
	if utf8.RuneCountInString(value) > cmdPaletteLocalIDMaxRunes {
		return cmdPaletteManifestError(field, value, "must be at most 64 characters")
	}
	if _, exists := seen[value]; exists {
		return cmdPaletteManifestError(field, "", fmt.Sprintf("duplicate %q", value))
	}
	seen[value] = struct{}{}
	return nil
}

func validateCmdPaletteText(field, value string, maxRunes int) error {
	if value == "" {
		return cmdPaletteManifestError(field, "", "value is required")
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return cmdPaletteManifestError(field, "", fmt.Sprintf("must be at most %d characters", maxRunes))
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return cmdPaletteManifestError(field, "", "must not contain control characters")
		}
	}
	return nil
}

func validateCmdPaletteIcon(field, value string) error {
	if value == "" {
		return cmdPaletteManifestError(field, "", "value is required")
	}
	if cmdPaletteIconPattern.MatchString(value) {
		return nil
	}
	if uniseg.GraphemeClusterCount(value) != 1 || isASCII(value) || !isCmdPaletteEmojiGrapheme(value) {
		return cmdPaletteManifestError(field, value, "must be a lowercase icon token or one emoji grapheme")
	}
	return validateCmdPaletteText(field, value, 16)
}

func isCmdPaletteEmojiGrapheme(value string) bool {
	hasEmoji := false
	for _, character := range value {
		if unicode.IsLetter(character) && !isEmojiRune(character) {
			return false
		}
		if isEmojiRune(character) {
			hasEmoji = true
		}
	}
	return hasEmoji
}

func isEmojiRune(character rune) bool {
	switch {
	case character == 0x200D || character == 0xFE0F || character == 0x20E3:
		return true
	case character >= 0x1F1E6 && character <= 0x1F1FF:
		return true
	case character >= 0x1F3FB && character <= 0x1F3FF:
		return true
	case character >= 0x2600 && character <= 0x27BF:
		return true
	case character >= 0x1F300 && character <= 0x1FAFF:
		return true
	case unicode.Is(unicode.So, character):
		return true
	default:
		return false
	}
}

func isASCII(value string) bool {
	for _, character := range value {
		if character > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func cmdPaletteUnknownRef(field, kind, value string) error {
	return cmdPaletteManifestError(field, "", fmt.Sprintf("unknown %s %q", kind, value))
}

func cmdPaletteManifestError(field, value, message string) error {
	return &ManifestValidationError{Field: field, Value: strings.TrimSpace(value), Message: message}
}
