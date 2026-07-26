package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/compozy/agh/internal/resources"
	"github.com/spf13/cobra"
)

const (
	taskOwnerKey = "owner"
)

const (
	resourceCreatedValue = "Created"
	resourceKindValue    = "Kind"
	resourceOwnerValue   = "Owner"
	resourceScopeValue   = "Scope"
	resourceSourceValue  = "Source"
	resourceStatusValue  = "Status"
	resourceUpdatedValue = "Updated"
	resourceVersionValue = "Version"
	resourceCreatedAtKey = "created_at"
	resourceDeletedKey   = "deleted"
	resourceKindKey      = "kind"
	resourceListKey      = "list"
	resourceResourceKey  = "resource"
	resourceStatusKey    = "status"
)

type resourcePutInput struct {
	scopeKind       string
	scopeID         string
	spec            string
	specFile        string
	expectedVersion int64
}

func newResourceCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   resourceResourceKey,
		Short: "Manage desired-state resources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newResourceListCommand(deps))
	cmd.AddCommand(newResourceGetCommand(deps))
	cmd.AddCommand(newResourcePutCommand(deps))
	cmd.AddCommand(newResourceDeleteCommand(deps))
	return cmd
}

func newResourceListCommand(deps commandDeps) *cobra.Command {
	var query ResourceListQuery
	var (
		kindRaw       string
		scopeKindRaw  string
		ownerKindRaw  string
		sourceKindRaw string
	)
	cmd := &cobra.Command{
		Use:   resourceListKey,
		Short: "List desired-state resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query.Kind = resources.ResourceKind(strings.TrimSpace(kindRaw))
			query.ScopeKind = resources.ResourceScopeKind(strings.TrimSpace(scopeKindRaw))
			query.OwnerKind = resources.ResourceOwnerKind(strings.TrimSpace(ownerKindRaw))
			query.SourceKind = resources.ResourceSourceKind(strings.TrimSpace(sourceKindRaw))
			records, err := client.ListResources(cmd.Context(), query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, resourceListBundle(records))
		},
	}
	cmd.Flags().StringVar(&kindRaw, resourceKindKey, "", "Filter by resource kind")
	cmd.Flags().StringVar(&scopeKindRaw, "scope-kind", "", "Filter by scope kind")
	cmd.Flags().StringVar(&query.ScopeID, "scope-id", "", "Filter by scope id")
	cmd.Flags().StringVar(&ownerKindRaw, "owner-kind", "", "Filter by owner kind")
	cmd.Flags().StringVar(&query.OwnerID, "owner-id", "", "Filter by owner id")
	cmd.Flags().StringVar(&sourceKindRaw, "source-kind", "", "Filter by source kind")
	cmd.Flags().StringVar(&query.SourceID, "source-id", "", "Filter by source id")
	cmd.Flags().IntVar(&query.Limit, "last", 0, "Maximum number of resources to return")
	return cmd
}

func newResourceGetCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <kind> <id>",
		Short: "Show one desired-state resource",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			record, err := client.GetResource(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, resourceBundle(record))
		},
	}
	return cmd
}

func newResourcePutCommand(deps commandDeps) *cobra.Command {
	var input resourcePutInput
	cmd := &cobra.Command{
		Use:   "put <kind> <id>",
		Short: "Create or update one desired-state resource",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			request, err := buildResourcePutRequest(cmd, input)
			if err != nil {
				return err
			}
			record, err := client.PutResource(cmd.Context(), args[0], args[1], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, resourceBundle(record))
		},
	}
	cmd.Flags().
		StringVar(
			&input.scopeKind,
			automationScopeKey,
			string(resources.ResourceScopeKindGlobal),
			"Scope kind: global or workspace",
		)
	cmd.Flags().StringVar(&input.scopeID, "scope-id", "", "Workspace id for workspace-scoped resources")
	cmd.Flags().StringVar(&input.spec, "spec", "", "Inline JSON resource spec")
	cmd.Flags().StringVar(&input.specFile, "spec-file", "", "Path to JSON resource spec file, or '-' for stdin")
	cmd.Flags().Int64Var(&input.expectedVersion, "expected-version", 0, "Optimistic version for updates")
	return cmd
}

func newResourceDeleteCommand(deps commandDeps) *cobra.Command {
	var expectedVersion int64
	cmd := &cobra.Command{
		Use:   "delete <kind> <id>",
		Short: "Delete one desired-state resource",
		Args:  exactTwoNonBlankArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if expectedVersion <= 0 {
				return fmt.Errorf("cli: --expected-version must be positive")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.DeleteResource(cmd.Context(), args[0], args[1], ResourceDeleteRequest{
				ExpectedVersion: expectedVersion,
			}); err != nil {
				return err
			}
			return writeCommandOutput(cmd, resourceDeleteBundle(args[0], args[1], expectedVersion))
		},
	}
	cmd.Flags().Int64Var(&expectedVersion, "expected-version", 0, "Version required for optimistic delete")
	mustMarkFlagRequired(cmd, "expected-version")
	return cmd
}

func buildResourcePutRequest(cmd *cobra.Command, input resourcePutInput) (ResourcePutRequest, error) {
	spec, err := resolveResourceSpec(cmd, input.spec, input.specFile)
	if err != nil {
		return ResourcePutRequest{}, err
	}
	scope := resources.ResourceScope{
		Kind: resources.ResourceScopeKind(strings.TrimSpace(input.scopeKind)),
		ID:   strings.TrimSpace(input.scopeID),
	}
	if err := scope.Validate(automationScopeKey); err != nil {
		return ResourcePutRequest{}, fmt.Errorf("cli: %w", err)
	}
	return ResourcePutRequest{
		Scope:           scope.Normalize(),
		ExpectedVersion: input.expectedVersion,
		Spec:            spec,
	}, nil
}

func resolveResourceSpec(cmd *cobra.Command, inline string, filePath string) (json.RawMessage, error) {
	inlineChanged := cmd.Flags().Lookup("spec") != nil && cmd.Flags().Lookup("spec").Changed
	filePath = strings.TrimSpace(filePath)
	if inlineChanged && filePath != "" {
		return nil, fmt.Errorf("cli: provide --spec or --spec-file, not both")
	}
	if inlineChanged {
		return compactResourceJSON("spec", inline)
	}
	if filePath == "" {
		return nil, fmt.Errorf("cli: --spec or --spec-file is required")
	}
	var payload []byte
	var err error
	if filePath == "-" {
		payload, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("cli: read resource spec stdin: %w", err)
		}
	} else {
		payload, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("cli: read resource spec file: %w", err)
		}
	}
	return compactResourceJSON("spec", string(payload))
}

func compactResourceJSON(name string, raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("cli: --%s requires valid JSON", name)
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(trimmed)); err != nil {
		return nil, fmt.Errorf("cli: invalid --%s JSON: %w", name, err)
	}
	return json.RawMessage(compacted.String()), nil
}

func resourceListBundle(items []ResourceRecord) outputBundle {
	return listBundle(
		struct {
			Records []ResourceRecord `json:"records"`
		}{Records: items},
		items,
		"Resources",
		[]string{"KIND", "ID", "VERSION", "SCOPE", "OWNER", "SOURCE"},
		"resources",
		[]string{resourceKindKey, "id", versionKey, automationScopeKey, taskOwnerKey, automationSourceKey},
		resourceRow,
		resourceRow,
	)
}

func resourceBundle(item ResourceRecord) outputBundle {
	return outputBundle{
		jsonValue: struct {
			Record ResourceRecord `json:"record"`
		}{Record: item},
		human: func() (string, error) {
			return renderHumanSection("Resource", []keyValue{
				{Label: resourceKindValue, Value: stringOrDash(string(item.Kind))},
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: resourceVersionValue, Value: fmt.Sprintf("%d", item.Version)},
				{Label: resourceScopeValue, Value: stringOrDash(formatResourceScope(item.Scope))},
				{Label: resourceOwnerValue, Value: stringOrDash(formatResourceOwner(item.Owner))},
				{Label: resourceSourceValue, Value: stringOrDash(formatResourceSource(item.Source))},
				{Label: resourceCreatedValue, Value: stringOrDash(formatTime(item.CreatedAt))},
				{Label: resourceUpdatedValue, Value: stringOrDash(formatTime(item.UpdatedAt))},
				{Label: "Spec", Value: stringOrDash(compactJSON(item.Spec))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(resourceResourceKey, []string{
				resourceKindKey,
				"id",
				versionKey,
				automationScopeKey,
				taskOwnerKey,
				automationSourceKey,
				resourceCreatedAtKey,
				automationUpdatedAtKey,
				"spec",
			}, []string{
				string(item.Kind),
				item.ID,
				fmt.Sprintf("%d", item.Version),
				formatResourceScope(item.Scope),
				formatResourceOwner(item.Owner),
				formatResourceSource(item.Source),
				formatTime(item.CreatedAt),
				formatTime(item.UpdatedAt),
				compactJSON(item.Spec),
			}), nil
		},
	}
}

func resourceDeleteBundle(kind string, id string, version int64) outputBundle {
	item := struct {
		Kind            string `json:"kind"`
		ID              string `json:"id"`
		ExpectedVersion int64  `json:"expected_version"`
		Status          string `json:"status"`
	}{
		Kind:            strings.TrimSpace(kind),
		ID:              strings.TrimSpace(id),
		ExpectedVersion: version,
		Status:          resourceDeletedKey,
	}
	return outputBundle{
		jsonValue: item,
		human: func() (string, error) {
			return renderHumanSection("Resource", []keyValue{
				{Label: resourceKindValue, Value: stringOrDash(item.Kind)},
				{Label: "ID", Value: stringOrDash(item.ID)},
				{Label: "Expected Version", Value: fmt.Sprintf("%d", item.ExpectedVersion)},
				{Label: resourceStatusValue, Value: item.Status},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				resourceResourceKey,
				[]string{resourceKindKey, "id", "expected_version", resourceStatusKey},
				[]string{item.Kind, item.ID, fmt.Sprintf("%d", item.ExpectedVersion), item.Status},
			), nil
		},
	}
}

func resourceRow(item ResourceRecord) []string {
	return []string{
		string(item.Kind),
		item.ID,
		fmt.Sprintf("%d", item.Version),
		formatResourceScope(item.Scope),
		formatResourceOwner(item.Owner),
		formatResourceSource(item.Source),
	}
}

func formatResourceScope(scope resources.ResourceScope) string {
	normalized := scope.Normalize()
	if normalized.ID == "" {
		return string(normalized.Kind)
	}
	return string(normalized.Kind) + ":" + normalized.ID
}

func formatResourceOwner(owner resources.ResourceOwner) string {
	normalized := owner.Normalize()
	if normalized.ID == "" {
		return string(normalized.Kind)
	}
	return string(normalized.Kind) + ":" + normalized.ID
}

func formatResourceSource(source resources.ResourceSource) string {
	normalized := source.Normalize()
	if normalized.ID == "" {
		return string(normalized.Kind)
	}
	return string(normalized.Kind) + ":" + normalized.ID
}
