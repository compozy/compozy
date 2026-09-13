package daemon

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"slices"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
)

// The lifecycle coordinator owns the instance lock throughout Prepare, Commit, and Rollback.
type extensionInputBinder struct{ service *daemonExtensionService }

type extensionInputPlan struct {
	instance      extensioninput.Instance
	key           extensionpkg.InstanceKey
	state         extensionpkg.InputState
	rows          []extensioninput.Mutation
	writes        []preparedExtensionSecret
	previous      map[string]extensionpkg.EnvBinding
	mutations     []extensionSecretMutation
	rowsCommitted bool
	committed     bool
	rolledBack    bool
}

func (b extensionInputBinder) Prepare(
	ctx context.Context, key extensionpkg.InstanceKey, profile extensionpkg.ProfileLens,
	manifest *extensionpkg.Manifest, values map[string]extensioninput.Value,
) (*extensionInputPlan, error) {
	declared, err := validateExtensionInputPreparation(key, profile, manifest, values)
	if err != nil {
		return nil, err
	}
	instance := extensioninput.Instance{Extension: key.Name, ProfileID: profile.ID, WorkspaceID: key.WorkspaceID}
	prior, err := b.service.inputReader().load(ctx, instance, manifest)
	if err != nil {
		return nil, err
	}
	plan := &extensionInputPlan{
		instance: instance,
		key:      key,
		state:    prior,
		previous: map[string]extensionpkg.EnvBinding{},
	}
	for id, record := range plan.state.Values {
		record.Active = false
		plan.state.Values[id] = record
	}
	if b.service.envBindings != nil {
		bindings, err := b.service.envBindings.ListEnvBindings(ctx, key.Name, profile.ID, key.WorkspaceID)
		if err != nil {
			return nil, err
		}
		plan.previous = extensionBindingsByName(bindings)
	}
	for _, input := range manifest.Inputs {
		value, supplied := values[input.ID]
		if supplied && ((len(value.Value) == 0) == (value.VaultRef == nil)) {
			return nil, inputInvalid(input.ID, "supply exactly one of value or vault_ref")
		}
		if input.Type == extensionInputTypeSecret {
			if err := b.prepareSecret(ctx, plan, input, value, supplied); err != nil {
				return nil, err
			}
		} else if err := b.prepareValue(plan, input, value, supplied); err != nil {
			return nil, err
		}
	}
	if err := b.prepareRows(ctx, plan, declared); err != nil {
		return nil, err
	}
	b.prepareRetiredSecrets(plan, declared)
	readiness := extensionpkg.InputReadiness(manifest, plan.state, b.service.getenv)
	if len(manifest.Inputs) > 0 {
		if err := readiness.RequiredError(manifest); err != nil {
			return nil, err
		}
	}
	return plan, nil
}

func (b extensionInputBinder) prepareValue(
	plan *extensionInputPlan, input extensionpkg.ManifestInput, value extensioninput.Value, supplied bool,
) error {
	if supplied {
		if value.VaultRef != nil {
			return inputInvalid(input.ID, "vault_ref is allowed only for secret inputs")
		}
		normalized, err := extensionpkg.NormalizeInputJSON(input.Type, value.Value)
		if err != nil {
			return inputInvalid(input.ID, err.Error())
		}
		plan.state.Values[input.ID] = extensionpkg.InputValueRecord{
			Type: input.Type, Value: normalized, Active: true, UpdatedAt: b.service.now().UTC(),
		}
		return nil
	}
	if row, found := plan.state.Values[input.ID]; found && row.Type == input.Type {
		row.Active = true
		plan.state.Values[input.ID] = row
	}
	return nil
}

func (b extensionInputBinder) prepareRows(
	ctx context.Context, plan *extensionInputPlan, declared map[string]extensionpkg.ManifestInput,
) error {
	before := map[string]extensioninput.Record{}
	if b.service.inputs != nil {
		rows, err := b.service.inputs.List(ctx, plan.instance)
		if err != nil {
			return err
		}
		before = rows
	}
	after := maps.Clone(before)
	for id, row := range after {
		row.Active = false
		after[id] = row
	}
	for id, record := range plan.state.Values {
		input, found := declared[id]
		if !found || input.Type == extensionInputTypeSecret || record.Type != input.Type || !record.Active {
			continue
		}
		after[id] = extensioninput.Record{
			Type:      record.Type,
			Value:     record.Value,
			Active:    true,
			UpdatedAt: record.UpdatedAt,
		}
	}
	for _, id := range slices.Sorted(maps.Keys(after)) {
		row := after[id]
		previous, found := before[id]
		if found && previous.Type == row.Type && previous.Active == row.Active &&
			bytes.Equal(previous.Value, row.Value) {
			continue
		}
		row.UpdatedAt = b.service.now().UTC()
		if record, exists := plan.state.Values[id]; exists && record.Type == row.Type {
			record.UpdatedAt = row.UpdatedAt
			plan.state.Values[id] = record
		}
		mutation := extensioninput.Mutation{InputID: id, After: &row}
		if found {
			mutation.Before = &previous
		}
		plan.rows = append(plan.rows, mutation)
	}
	if len(plan.rows) > 0 && b.service.inputs == nil {
		return errors.New("daemon: extension input store is required")
	}
	return nil
}

func (b extensionInputBinder) prepareRetiredSecrets(
	plan *extensionInputPlan, declared map[string]extensionpkg.ManifestInput,
) {
	written := make(map[string]bool, len(plan.writes))
	for _, write := range plan.writes {
		written[write.envName] = true
	}
	for _, envName := range slices.Sorted(maps.Keys(plan.previous)) {
		before := plan.previous[envName]
		if before.InputID == "" || written[envName] {
			continue
		}
		input, present := declared[before.InputID]
		inactive := !present || input.Type != extensionInputTypeSecret || input.Binding.Name != envName
		if before.Inactive == inactive {
			continue
		}
		plan.writes = append(plan.writes, preparedExtensionSecret{
			envName: envName, ref: before.SecretRef, inputID: before.InputID, inactive: inactive,
			mcpServer: before.MCPServer, headerName: before.HeaderName,
		})
	}
}

func inputInvalid(id, reason string) error {
	return &extensionpkg.InputValidationError{InputID: id, Reason: reason}
}

func validateExtensionInputPreparation(
	key extensionpkg.InstanceKey, profile extensionpkg.ProfileLens, manifest *extensionpkg.Manifest,
	values map[string]extensioninput.Value,
) (map[string]extensionpkg.ManifestInput, error) {
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if profile.ID == "" || profile.Name == "" {
		return nil, errors.New("daemon: input profile is required")
	}
	if manifest == nil {
		return nil, errors.New("daemon: input manifest is required")
	}
	if err := extensionpkg.ValidateManifestInputs(manifest); err != nil {
		return nil, err
	}
	declared := make(map[string]extensionpkg.ManifestInput, len(manifest.Inputs))
	for _, input := range manifest.Inputs {
		declared[input.ID] = input
	}
	for _, id := range slices.Sorted(maps.Keys(values)) {
		if _, found := declared[id]; !found {
			return nil, inputInvalid(id, "input is not declared by the manifest")
		}
	}
	return declared, nil
}
