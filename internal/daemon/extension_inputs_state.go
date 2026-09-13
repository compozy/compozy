package daemon

import (
	"context"
	"errors"
	"fmt"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/vault"
)

type extensionInputReader struct {
	inputs   extensioninput.Store
	bindings extensionpkg.EnvBindingStore
	secrets  interface {
		GetMetadata(context.Context, string) (vault.Metadata, error)
	}
}

func (r extensionInputReader) load(
	ctx context.Context, instance extensioninput.Instance, manifest *extensionpkg.Manifest,
) (extensionpkg.InputState, error) {
	state := extensionpkg.InputState{Values: map[string]extensionpkg.InputValueRecord{}}
	if r.inputs != nil {
		rows, err := r.inputs.List(ctx, instance)
		if err != nil {
			return state, err
		}
		for id, row := range rows {
			state.Values[id] = extensionpkg.InputValueRecord{
				Type:      row.Type,
				Value:     row.Value,
				Active:    row.Active,
				UpdatedAt: row.UpdatedAt,
			}
		}
	}
	if r.bindings == nil {
		return state, nil
	}
	bindings, err := r.bindings.ListEnvBindings(ctx, instance.Extension, instance.ProfileID, instance.WorkspaceID)
	if err != nil {
		return state, err
	}
	for _, binding := range bindings {
		if binding.InputID == "" || preferStoredNonSecretInput(manifest, state, binding) {
			continue
		}
		if err := r.addSecretState(ctx, &state, binding.InputID, binding); err != nil {
			return state, err
		}
	}
	if manifest == nil || instance.ProfileID == "" {
		return state, nil
	}
	return state, r.addLegacyInputs(ctx, instance, manifest, &state)
}

func (r extensionInputReader) addLegacyInputs(
	ctx context.Context,
	instance extensioninput.Instance,
	manifest *extensionpkg.Manifest,
	state *extensionpkg.InputState,
) error {
	// Preserve legacy requires_env inheritance; typed input bindings address their exact instance above.
	effective, err := r.bindings.ResolveEnvBindings(ctx, instance.Extension, instance.ProfileID, instance.WorkspaceID)
	if err != nil {
		return err
	}
	for _, input := range manifest.Inputs {
		if input.Type != "secret" {
			continue
		}
		if _, found := state.Values[input.ID]; found {
			continue
		}
		for _, binding := range effective {
			if binding.InputID == "" && binding.EnvName == input.Binding.Name && binding.MCPServer == "" {
				if err := r.addSecretState(ctx, state, input.ID, binding); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func preferStoredNonSecretInput(
	manifest *extensionpkg.Manifest, state extensionpkg.InputState, binding extensionpkg.EnvBinding,
) bool {
	row, exists := state.Values[binding.InputID]
	if !exists {
		return false
	}
	if manifest != nil {
		for _, input := range manifest.Inputs {
			if input.ID == binding.InputID {
				return input.Type != "secret"
			}
		}
	}
	return row.Active && binding.Inactive
}

func (r extensionInputReader) addSecretState(
	ctx context.Context, state *extensionpkg.InputState, id string, binding extensionpkg.EnvBinding,
) error {
	record := extensionpkg.InputValueRecord{Type: "secret", Active: !binding.Inactive, UpdatedAt: binding.UpdatedAt}
	if r.secrets != nil {
		metadata, err := r.secrets.GetMetadata(ctx, binding.SecretRef)
		if err != nil && !errors.Is(err, vault.ErrSecretNotFound) {
			return fmt.Errorf("daemon: inspect extension input %q secret metadata: %w", id, err)
		}
		if err == nil && metadata.Present {
			record.SecretRef = binding.SecretRef
		}
	}
	state.Values[id] = record
	return nil
}

func withDaemonExtensionInputs(inputs extensioninput.Store) daemonExtensionServiceOption {
	return func(service *daemonExtensionService) { service.inputs = inputs }
}

func (s *daemonExtensionService) inputReader() extensionInputReader {
	return extensionInputReader{inputs: s.inputs, bindings: s.envBindings, secrets: s.secretVault}
}

func (s *daemonExtensionService) projectExtensionInputKitItems(
	ctx context.Context, ext *extensionpkg.Extension, profileID string,
) ([]extensionpkg.KitItem, error) {
	state, err := s.inputReader().load(ctx, extensioninput.Instance{
		Extension: ext.Info.Name, ProfileID: profileID, WorkspaceID: ext.Status.WorkspaceID,
	}, ext.Manifest)
	if err != nil {
		return nil, err
	}
	return projectExtensionKitItems(ctx, ext, s.resourceCodecs, s.getenv, state)
}
