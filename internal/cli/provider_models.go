package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	providerModelsErrorValue    = "Error"
	providerModelsModelValue    = "Model"
	providerModelsAvailableKey  = "available"
	providerModelsProviderIDKey = "provider_id"
)

const providerModelAvailabilityUnknown = "unknown"

const (
	providerModelViewAll     = "all"
	providerModelViewCurated = "curated"
)

func newProviderModelsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Inspect and refresh the provider model catalog",
	}
	cmd.AddCommand(newProviderModelsListCommand(deps))
	cmd.AddCommand(newProviderModelsSetCommand(deps))
	cmd.AddCommand(newProviderModelsRefreshCommand(deps))
	cmd.AddCommand(newProviderModelsStatusCommand(deps))
	return cmd
}

func newProviderModelsListCommand(deps commandDeps) *cobra.Command {
	var sourceID string
	var refresh bool
	var includeStale bool
	var all bool
	cmd := &cobra.Command{
		Use:   "list [provider]",
		Short: "List provider model catalog entries",
		Args:  optionalProviderArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerID := providerArgValue(args)
			view := providerModelViewCurated
			if all {
				view = providerModelViewAll
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ListProviderModels(cmd.Context(), ProviderModelListQuery{
				ProviderID:   providerID,
				SourceID:     sourceID,
				View:         view,
				Refresh:      refresh,
				IncludeStale: includeStale,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerModelListBundle(record))
		},
	}
	cmd.Flags().StringVar(&sourceID, "source", "", "Filter by catalog source id")
	cmd.Flags().BoolVar(&all, "all", false, "Include hidden and deprecated catalog models")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh sources before listing models")
	cmd.Flags().BoolVar(&includeStale, "include-stale", false, "Include stale source rows")
	return cmd
}

func newProviderModelsSetCommand(deps commandDeps) *cobra.Command {
	var hidden bool
	var featured bool
	var deprecated bool
	var defaultEffort string
	cmd := &cobra.Command{
		Use:   "set <provider> <model>",
		Short: "Curate one provider model",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			providerID := strings.TrimSpace(args[0])
			modelID := strings.TrimSpace(args[1])
			if providerID == "" || modelID == "" {
				return fmt.Errorf("provider and model ids cannot be blank")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request := ProviderModelCurationRequest{ModelID: modelID}
			if cmd.Flags().Changed("hidden") {
				request.Hidden = &hidden
			}
			if cmd.Flags().Changed("featured") {
				request.Featured = &featured
			}
			if cmd.Flags().Changed("deprecated") {
				request.Deprecated = &deprecated
			}
			if cmd.Flags().Changed("default-effort") {
				effort := contract.ReasoningEffort(strings.TrimSpace(defaultEffort))
				request.DefaultReasoningEffort = &effort
			}
			record, err := client.CurateProviderModel(cmd.Context(), providerID, request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerModelCurationBundle(&record))
		},
	}
	cmd.Flags().BoolVar(&hidden, "hidden", false, "Set whether the model is hidden")
	cmd.Flags().BoolVar(&featured, "featured", false, "Set whether the model is featured")
	cmd.Flags().BoolVar(&deprecated, "deprecated", false, "Set whether the model is deprecated")
	cmd.Flags().StringVar(&defaultEffort, "default-effort", "", "Set the model's default reasoning effort")
	return cmd
}

func newProviderModelsRefreshCommand(deps commandDeps) *cobra.Command {
	var sourceID string
	var force bool
	var requestID string
	cmd := &cobra.Command{
		Use:   "refresh [provider]",
		Short: "Refresh provider model catalog sources",
		Args:  optionalProviderArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.RefreshProviderModels(
				cmd.Context(),
				providerArgValue(args),
				ProviderModelRefreshRequest{
					SourceID:  sourceID,
					Force:     force,
					RequestID: requestID,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerModelRefreshBundle(record))
		},
	}
	cmd.Flags().StringVar(&sourceID, "source", "", "Refresh only one catalog source id")
	cmd.Flags().BoolVar(&force, "force", false, "Force refresh even when cached status is fresh")
	cmd.Flags().StringVar(&requestID, "request-id", "", "Refresh request id for daemon logs")
	return cmd
}

func newProviderModelsStatusCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [provider]",
		Short: "Show provider model catalog source status",
		Args:  optionalProviderArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.ProviderModelStatus(cmd.Context(), providerArgValue(args))
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, providerModelStatusBundle(record))
		},
	}
	return cmd
}

func providerModelListBundle(record ProviderModelListRecord) outputBundle {
	return listBundle(
		record,
		record.Models,
		"Provider Models",
		[]string{
			agentKernelProviderValue,
			providerModelsModelValue,
			"Curated",
			"Hidden",
			"Deprecated",
			"Featured",
			"Default Effort",
			"Available",
			authoredContextStateValue,
			outputStaleValue,
			"Sources",
			"Refreshed",
		},
		"provider_models",
		[]string{
			providerModelsProviderIDKey,
			"model_id",
			providerModelViewCurated,
			"hidden",
			"deprecated",
			"featured",
			"default_reasoning_effort",
			providerModelsAvailableKey,
			"availability_state",
			outputStaleKey,
			"sources",
			"refreshed_at",
		},
		func(model ProviderModelRecord) []string {
			return []string{
				model.ProviderID,
				model.ModelID,
				providerModelBoolString(model.Curated),
				providerModelBoolString(model.Hidden),
				providerModelBoolString(model.Deprecated),
				providerModelBoolString(model.Featured),
				providerModelEffortString(model.DefaultReasoningEffort),
				providerModelNullableBoolString(model.Available),
				model.AvailabilityState,
				providerModelBoolString(model.Stale),
				providerModelSourceSummary(model.Sources),
				model.RefreshedAt,
			}
		},
		func(model ProviderModelRecord) []string {
			return []string{
				model.ProviderID,
				model.ModelID,
				providerModelBoolString(model.Curated),
				providerModelBoolString(model.Hidden),
				providerModelBoolString(model.Deprecated),
				providerModelBoolString(model.Featured),
				providerModelEffortString(model.DefaultReasoningEffort),
				providerModelNullableBoolString(model.Available),
				model.AvailabilityState,
				providerModelBoolString(model.Stale),
				providerModelSourceSummary(model.Sources),
				model.RefreshedAt,
			}
		},
	)
}

func providerModelCurationBundle(record *ProviderModelCurationRecord) outputBundle {
	model := record.Model
	rows := []keyValue{
		{Label: agentKernelProviderValue, Value: model.ProviderID},
		{Label: providerModelsModelValue, Value: model.ModelID},
		{Label: "Curated", Value: providerModelBoolString(model.Curated)},
		{Label: "Hidden", Value: providerModelBoolString(model.Hidden)},
		{Label: "Deprecated", Value: providerModelBoolString(model.Deprecated)},
		{Label: "Featured", Value: providerModelBoolString(model.Featured)},
		{Label: "Default Effort", Value: providerModelEffortString(model.DefaultReasoningEffort)},
		{Label: cliAppliedValue, Value: providerModelBoolString(record.Apply.Applied)},
		{Label: cliLifecycleValue, Value: string(record.Apply.Lifecycle)},
		{Label: cliActiveGenerationValue, Value: fmt.Sprintf("%d", record.Apply.ActiveGeneration)},
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Provider Model Curation", rows), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"provider_model_curation",
				[]string{
					providerModelsProviderIDKey,
					"model_id",
					providerModelViewCurated,
					"hidden",
					"deprecated",
					"featured",
					"default_reasoning_effort",
					cliAppliedKey,
					cliLifecycleKey,
					cliActiveGenerationKey,
				},
				[]string{
					model.ProviderID,
					model.ModelID,
					providerModelBoolString(model.Curated),
					providerModelBoolString(model.Hidden),
					providerModelBoolString(model.Deprecated),
					providerModelBoolString(model.Featured),
					providerModelEffortString(model.DefaultReasoningEffort),
					providerModelBoolString(record.Apply.Applied),
					string(record.Apply.Lifecycle),
					fmt.Sprintf("%d", record.Apply.ActiveGeneration),
				},
			), nil
		},
	}
}

func providerModelRefreshBundle(record ProviderModelRefreshRecord) outputBundle {
	return providerModelSourceStatusBundle("Provider Model Refresh", "provider_model_refresh", record, record.Sources)
}

func providerModelStatusBundle(record ProviderModelStatusRecord) outputBundle {
	return providerModelSourceStatusBundle("Provider Model Status", "provider_model_status", record, record.Sources)
}

func providerModelSourceStatusBundle(
	humanTitle string,
	toonName string,
	jsonValue any,
	sources []ProviderModelSourceStatusRecord,
) outputBundle {
	return listBundle(
		jsonValue,
		sources,
		humanTitle,
		[]string{
			agentKernelProviderValue,
			authoredContextSourceValue,
			bridgeKindValue,
			authoredContextStateValue,
			"Rows",
			"Stale",
			providerModelsErrorValue,
		},
		toonName,
		[]string{
			providerModelsProviderIDKey,
			"source_id",
			"source_kind",
			"refresh_state",
			"row_count",
			"stale",
			"last_error",
		},
		providerModelSourceStatusRow,
		providerModelSourceStatusRow,
	)
}

func providerModelSourceStatusRow(source ProviderModelSourceStatusRecord) []string {
	return []string{
		source.ProviderID,
		source.SourceID,
		source.SourceKind,
		source.RefreshState,
		fmt.Sprintf("%d", source.RowCount),
		providerModelBoolString(source.Stale),
		source.LastError,
	}
}

func optionalProviderArg() cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > 1 {
			return fmt.Errorf("accepts at most 1 arg(s), received %d", len(args))
		}
		if len(args) == 1 && strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("provider id cannot be blank")
		}
		return nil
	}
}

func providerArgValue(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return strings.TrimSpace(args[0])
}

func providerModelSourceSummary(sources []contract.ModelCatalogSourceRefPayload) string {
	if len(sources) == 0 {
		return ""
	}
	values := make([]string, 0, len(sources))
	for _, source := range sources {
		values = append(values, source.SourceID)
	}
	return strings.Join(values, ",")
}

func providerModelNullableBoolString(value *bool) string {
	if value == nil {
		return providerModelAvailabilityUnknown
	}
	return providerModelBoolString(*value)
}

func providerModelEffortString(value *contract.ReasoningEffort) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(string(*value))
}

func providerModelBoolString(value bool) string {
	if value {
		return toolBoolTrue
	}
	return toolBoolFalse
}
