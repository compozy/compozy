package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	notificationsKey       = "notifications"
	notificationPresetKey  = "preset"
	notificationPresetsKey = "presets"
	notificationTargetsKey = "targets"
)

func newNotificationsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   notificationsKey,
		Short: "Manage notification presets",
	}
	cmd.AddCommand(newNotificationPresetsCommand(deps))
	cmd.AddCommand(newNotificationPresetCommand(deps))
	return cmd
}

func newNotificationPresetsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   notificationPresetsKey,
		Short: "List notification presets",
	}
	cmd.AddCommand(newNotificationPresetListCommand(deps))
	return cmd
}

func newNotificationPresetCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   notificationPresetKey,
		Short: "Manage one notification preset",
	}
	cmd.AddCommand(newNotificationPresetShowCommand(deps))
	cmd.AddCommand(newNotificationPresetCreateCommand(deps))
	cmd.AddCommand(newNotificationPresetUpdateCommand(deps))
	cmd.AddCommand(newNotificationPresetEnableCommand(deps))
	cmd.AddCommand(newNotificationPresetDisableCommand(deps))
	cmd.AddCommand(newNotificationPresetDeleteCommand(deps))
	return cmd
}

func newNotificationPresetListCommand(deps commandDeps) *cobra.Command {
	var (
		name    string
		limit   int
		enabled bool
		builtIn bool
	)
	cmd := &cobra.Command{
		Use:   extensionListKey,
		Short: "List notification presets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query := NotificationPresetQuery{Name: name, Limit: limit}
			if cmd.Flags().Changed(extensionEnabledKey) {
				query.Enabled = &enabled
			}
			if cmd.Flags().Changed("built-in") {
				query.BuiltIn = &builtIn
			}
			result, err := client.ListNotificationPresets(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetListBundle(result))
		},
	}
	cmd.Flags().StringVar(&name, automationNameKey, "", "Filter by preset name")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of presets to return")
	cmd.Flags().
		BoolVar(&enabled, extensionEnabledKey, false, "Only show presets with this enabled value")
	cmd.Flags().BoolVar(&builtIn, "built-in", false, "Only show presets with this built-in value")
	return cmd
}

func newNotificationPresetShowCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show one notification preset",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			preset, err := client.GetNotificationPreset(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetBundle(preset))
		},
	}
}

func newNotificationPresetCreateCommand(deps commandDeps) *cobra.Command {
	var (
		events  []string
		targets []string
		filter  string
		enabled bool
	)
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a notification preset",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			payloadTargets, err := parseNotificationPresetTargets(targets)
			if err != nil {
				return err
			}
			preset, err := client.CreateNotificationPreset(
				cmd.Context(),
				CreateNotificationPresetRequest{
					Name:    strings.TrimSpace(args[0]),
					Events:  normalizeNotificationPresetStrings(events),
					Targets: payloadTargets,
					Filter:  strings.TrimSpace(filter),
					Enabled: enabled,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetBundle(preset))
		},
	}
	cmd.Flags().
		StringArrayVar(&events, "event", nil, "Event pattern to match; repeat for multiple patterns")
	cmd.Flags().
		StringArrayVar(&targets, "target", nil, "Target as bridge_id:canonical_route; repeat for multiple targets")
	cmd.Flags().StringVar(&filter, "filter", "", "Optional filter expression")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Create the preset as enabled")
	mustMarkFlagRequired(cmd, "event")
	return cmd
}

func newNotificationPresetUpdateCommand(deps commandDeps) *cobra.Command {
	var (
		events   []string
		targets  []string
		filter   string
		enabled  bool
		disabled bool
	)
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a notification preset",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildNotificationPresetUpdateRequest(cmd, events, targets, filter, enabled, disabled)
			if err != nil {
				return err
			}
			preset, err := client.UpdateNotificationPreset(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetBundle(preset))
		},
	}
	cmd.Flags().
		StringArrayVar(&events, "event", nil, "Replace event patterns; repeat for multiple patterns")
	cmd.Flags().
		StringArrayVar(&targets, "target", nil, "Replace targets using bridge_id:canonical_route")
	cmd.Flags().StringVar(&filter, "filter", "", "Replace the optional filter expression")
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Mark the preset enabled")
	cmd.Flags().BoolVar(&disabled, "disabled", false, "Mark the preset disabled")
	return cmd
}

func buildNotificationPresetUpdateRequest(
	cmd *cobra.Command,
	events []string,
	targets []string,
	filter string,
	enabled bool,
	disabled bool,
) (UpdateNotificationPresetRequest, error) {
	if cmd == nil {
		return UpdateNotificationPresetRequest{}, errors.New("cli: notification preset update command is required")
	}
	eventsChanged := cmd.Flags().Changed("event")
	targetsChanged := cmd.Flags().Changed("target")
	filterChanged := cmd.Flags().Changed("filter")
	enabledChanged := cmd.Flags().Changed("enabled")
	disabledChanged := cmd.Flags().Changed("disabled")
	if enabledChanged && disabledChanged {
		return UpdateNotificationPresetRequest{}, errors.New("cli: use either --enabled or --disabled, not both")
	}
	if !eventsChanged && !targetsChanged && !filterChanged && !enabledChanged && !disabledChanged {
		return UpdateNotificationPresetRequest{}, errors.New("cli: at least one update flag is required")
	}

	request := UpdateNotificationPresetRequest{}
	if eventsChanged {
		values := normalizeNotificationPresetStrings(events)
		request.Events = &values
	}
	if targetsChanged {
		payloadTargets, err := parseNotificationPresetTargets(targets)
		if err != nil {
			return UpdateNotificationPresetRequest{}, err
		}
		request.Targets = &payloadTargets
	}
	if filterChanged {
		value := strings.TrimSpace(filter)
		request.Filter = &value
	}
	if enabledChanged || disabledChanged {
		value := enabledChanged && enabled
		if disabledChanged {
			value = !disabled
		}
		request.Enabled = &value
	}
	return request, nil
}

func newNotificationPresetEnableCommand(deps commandDeps) *cobra.Command {
	return newNotificationPresetToggleCommand(deps, true)
}

func newNotificationPresetDisableCommand(deps commandDeps) *cobra.Command {
	return newNotificationPresetToggleCommand(deps, false)
}

func newNotificationPresetToggleCommand(deps commandDeps, enabled bool) *cobra.Command {
	var targets []string
	use := cliUseDisableName
	short := "Disable a notification preset"
	if enabled {
		use = cliUseEnableName
		short = "Enable a notification preset"
	}
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			req := UpdateNotificationPresetRequest{Enabled: &enabled}
			if cmd.Flags().Changed("target") {
				payloadTargets, parseErr := parseNotificationPresetTargets(targets)
				if parseErr != nil {
					return parseErr
				}
				req.Targets = &payloadTargets
			}
			preset, err := client.UpdateNotificationPreset(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetBundle(preset))
		},
	}
	cmd.Flags().
		StringArrayVar(&targets, "target", nil, "Replace targets using bridge_id:canonical_route")
	return cmd
}

func newNotificationPresetDeleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a custom notification preset",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			name := strings.TrimSpace(args[0])
			if err := client.DeleteNotificationPreset(cmd.Context(), name); err != nil {
				return err
			}
			return writeCommandOutput(cmd, notificationPresetDeleteBundle(name))
		},
	}
}

func notificationPresetListBundle(result NotificationPresetListRecord) outputBundle {
	return listBundle(
		result,
		result.Presets,
		"Notification Presets",
		[]string{"NAME", "ENABLED", "BUILT IN", "EVENTS", "TARGETS", "MODIFIED"},
		"notification_presets",
		[]string{
			automationNameKey,
			extensionEnabledKey,
			"built_in",
			"events",
			notificationTargetsKey,
			"user_modified",
		},
		func(preset NotificationPresetRecord) []string {
			return []string{
				stringOrDash(preset.Name),
				fmt.Sprintf("%t", preset.Enabled),
				fmt.Sprintf("%t", preset.BuiltIn),
				stringOrDash(strings.Join(preset.Events, ",")),
				stringOrDash(notificationPresetTargetSummary(preset.Targets)),
				fmt.Sprintf("%t", preset.UserModified),
			}
		},
		func(preset NotificationPresetRecord) []string {
			return []string{
				preset.Name,
				fmt.Sprintf("%t", preset.Enabled),
				fmt.Sprintf("%t", preset.BuiltIn),
				strings.Join(preset.Events, ","),
				notificationPresetTargetSummary(preset.Targets),
				fmt.Sprintf("%t", preset.UserModified),
			}
		},
	)
}

func notificationPresetBundle(preset NotificationPresetRecord) outputBundle {
	return outputBundle{
		jsonValue: preset,
		human: func() (string, error) {
			return renderHumanSection("Notification Preset", []keyValue{
				{Label: automationNameValue, Value: stringOrDash(preset.Name)},
				{Label: extensionEnabledValue, Value: fmt.Sprintf("%t", preset.Enabled)},
				{Label: "Built In", Value: fmt.Sprintf("%t", preset.BuiltIn)},
				{Label: "Events", Value: stringOrDash(strings.Join(preset.Events, ", "))},
				{
					Label: "Targets",
					Value: stringOrDash(notificationPresetTargetSummary(preset.Targets)),
				},
				{Label: "Filter", Value: stringOrDash(preset.Filter)},
				{Label: "Default Version", Value: stringOrDash(preset.DefaultVersion)},
				{Label: cliHashValue, Value: stringOrDash(preset.DefaultHash)},
				{Label: "User Modified", Value: fmt.Sprintf("%t", preset.UserModified)},
				{Label: "Default Update", Value: fmt.Sprintf("%t", preset.DefaultUpdateAvailable)},
				{Label: bridgeCreatedValue, Value: stringOrDash(formatTime(preset.CreatedAt))},
				{
					Label: authoredContextUpdatedValue,
					Value: stringOrDash(formatTime(preset.UpdatedAt)),
				},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("notification_preset", []string{
				automationNameKey,
				extensionEnabledKey,
				"built_in",
				"events",
				notificationTargetsKey,
				"filter",
				"user_modified",
			}, []string{
				preset.Name,
				fmt.Sprintf("%t", preset.Enabled),
				fmt.Sprintf("%t", preset.BuiltIn),
				strings.Join(preset.Events, ","),
				notificationPresetTargetSummary(preset.Targets),
				preset.Filter,
				fmt.Sprintf("%t", preset.UserModified),
			}), nil
		},
	}
}

func notificationPresetDeleteBundle(name string) outputBundle {
	payload := map[string]string{resourceDeletedKey: strings.TrimSpace(name)}
	return outputBundle{
		jsonValue: payload,
		human: func() (string, error) {
			return fmt.Sprintf("Deleted notification preset %s", stringOrDash(name)), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"notification_preset_delete",
				[]string{resourceDeletedKey},
				[]string{name},
			), nil
		},
	}
}

func parseNotificationPresetTargets(values []string) ([]contract.NotificationTargetPayload, error) {
	targets := make([]contract.NotificationTargetPayload, 0, len(values))
	for _, raw := range values {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		bridgeID, route, ok := strings.Cut(trimmed, ":")
		if !ok || strings.TrimSpace(bridgeID) == "" || strings.TrimSpace(route) == "" {
			return nil, errors.New("cli: --target must use bridge_id:canonical_route")
		}
		targets = append(targets, contract.NotificationTargetPayload{
			BridgeID:       strings.TrimSpace(bridgeID),
			CanonicalRoute: strings.TrimSpace(route),
		})
	}
	return targets, nil
}

func normalizeNotificationPresetStrings(values []string) []string {
	items := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func notificationPresetTargetSummary(targets []contract.NotificationTargetPayload) string {
	if len(targets) == 0 {
		return ""
	}
	parts := make([]string, 0, len(targets))
	for _, target := range targets {
		route := strings.TrimSpace(target.CanonicalRoute)
		if route == "" {
			route = strings.TrimSpace(target.DisplayName)
		}
		parts = append(parts, strings.TrimSpace(target.BridgeID)+":"+route)
	}
	return strings.Join(parts, ",")
}
