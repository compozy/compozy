package loop

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/loop/dsl"
)

func validateExecutedDefinitionSnapshot(envelope *executedDefinitionSnapshot) error {
	if envelope.FormatVersion != executedDefinitionSnapshotFormatVersion {
		return fmt.Errorf(
			"%w: executed definition format_version %d is unsupported",
			ErrValidation,
			envelope.FormatVersion,
		)
	}
	if envelope.DefinitionVersion != envelope.Definition.Meta.Version {
		return fmt.Errorf("%w: executed definition version changed", ErrValidation)
	}
	if envelope.TemplateSources == nil || envelope.ConditionSources == nil ||
		envelope.ToolSchemas == nil || envelope.WatchEventsContracts == nil {
		return fmt.Errorf("%w: executed definition manifests must be JSON objects", ErrValidation)
	}
	if err := validateResolvedDefaults(envelope.Defaults); err != nil {
		return err
	}
	if err := validateEffectiveConfig(envelope.EffectiveConfig); err != nil {
		return err
	}
	if err := validateExecutedTemplateManifest(envelope.TemplateSources, "template"); err != nil {
		return err
	}
	if err := validateExecutedTemplateManifest(envelope.ConditionSources, "condition"); err != nil {
		return err
	}
	if err := validateExecutedToolSnapshots(envelope.Definition, envelope.ToolSchemas); err != nil {
		return err
	}
	return validateExecutedWatchContracts(envelope.Definition, envelope.WatchEventsContracts)
}

func validateResolvedDefaults(defaults ResolvedDefaults) error {
	if defaults.FanOutBatchSize < 1 {
		return fmt.Errorf("%w: resolved fan-out batch size must be positive", ErrValidation)
	}
	if defaults.RunLoopMode != dsl.RunLoopAwait && defaults.RunLoopMode != dsl.RunLoopDetach {
		return fmt.Errorf("%w: resolved run-loop mode is invalid", ErrValidation)
	}
	switch defaults.Concurrency {
	case dsl.ConcurrencyForbid, dsl.ConcurrencyAllow, dsl.ConcurrencyQueue:
		return nil
	default:
		return fmt.Errorf("%w: resolved concurrency policy is invalid", ErrValidation)
	}
}

func validateExecutedTemplateManifest(manifest map[string]string, label string) error {
	for key, source := range manifest {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(source) == "" {
			return fmt.Errorf("%w: executed definition %s manifest is invalid", ErrValidation, label)
		}
	}
	return nil
}

func validateExecutedToolSnapshots(
	definition dsl.Definition,
	snapshots map[string]ToolSchemaSnapshot,
) error {
	referenced := map[string]struct{}{}
	collectOpenActionKinds(definition.Graph, referenced)
	if len(referenced) != len(snapshots) {
		return fmt.Errorf("%w: executed definition tool snapshot set changed", ErrValidation)
	}
	for kind := range referenced {
		snapshot, ok := snapshots[kind]
		if !ok || strings.TrimSpace(snapshot.ToolID) != kind ||
			strings.TrimSpace(snapshot.InputSchemaDigest) == "" ||
			len(snapshot.InputSchema) == 0 || !json.Valid(snapshot.InputSchema) {
			return fmt.Errorf("%w: executed tool snapshot %q is incomplete", ErrValidation, kind)
		}
		if len(snapshot.OutputSchema) > 0 && (!json.Valid(snapshot.OutputSchema) ||
			strings.TrimSpace(snapshot.OutputSchemaDigest) == "") {
			return fmt.Errorf("%w: executed tool output snapshot %q is incomplete", ErrValidation, kind)
		}
	}
	return nil
}

func collectOpenActionKinds(graph dsl.Graph, kinds map[string]struct{}) {
	for _, node := range graph.Nodes {
		if node.Class == dsl.NodeClassAction && !dsl.IsReservedActionKind(node.Kind) {
			kinds[strings.TrimSpace(node.Kind)] = struct{}{}
		}
		if node.Body != nil {
			collectOpenActionKinds(*node.Body, kinds)
		}
	}
}

func validateExecutedWatchContracts(
	definition dsl.Definition,
	contracts map[hooks.HookEvent]WatchEventsContract,
) error {
	referenced := referencedWatchEventKinds(definition.Graph)
	if len(referenced) != len(contracts) {
		return fmt.Errorf("%w: executed watch-events contract set changed", ErrValidation)
	}
	for kind := range referenced {
		contract, ok := contracts[kind]
		if !ok || contract.Kind != kind || strings.TrimSpace(contract.Stream) == "" ||
			len(contract.LedgerTypes) == 0 {
			return fmt.Errorf("%w: executed watch-events contract %q is incomplete", ErrValidation, kind)
		}
	}
	return nil
}

func referencedWatchEventKinds(graph dsl.Graph) map[hooks.HookEvent]struct{} {
	kinds := map[hooks.HookEvent]struct{}{}
	for _, node := range graph.Nodes {
		for _, subscription := range node.Events {
			kinds[hooks.HookEvent(strings.TrimSpace(subscription.Kind))] = struct{}{}
		}
		if node.Body != nil {
			for kind := range referencedWatchEventKinds(*node.Body) {
				kinds[kind] = struct{}{}
			}
		}
	}
	return kinds
}
