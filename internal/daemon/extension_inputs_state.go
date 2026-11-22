package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/vault"
)

const extensionInputTypeSecret = "secret"

type extensionInputReader struct {
	inputs   extensioninput.Store
	bindings extensionpkg.EnvBindingStore
	secrets  interface {
		GetMetadata(context.Context, string) (vault.Metadata, error)
		ResolveRef(context.Context, string) (string, error)
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
		if binding.InputID == "" || preferStoredInput(manifest, state, binding) {
			continue
		}
		if err := r.addBindingState(ctx, &state, manifest, binding.InputID, binding); err != nil {
			return state, err
		}
	}
	if manifest == nil || instance.ProfileID == "" {
		return state, nil
	}
	err = r.addRequiresEnvInputs(ctx, instance, manifest, &state)
	return state, err
}

func (r extensionInputReader) addRequiresEnvInputs(
	ctx context.Context,
	instance extensioninput.Instance,
	manifest *extensionpkg.Manifest,
	state *extensionpkg.InputState,
) error {
	// Preserve requires_env inheritance; typed input bindings address their exact instance above.
	effective, err := r.bindings.ResolveEnvBindings(ctx, instance.Extension, instance.ProfileID, instance.WorkspaceID)
	if err != nil {
		return err
	}
	headerInputs := manifest.AgentPluginHeaderSecretInputs()
	for _, input := range manifest.Inputs {
		if input.Type != extensionInputTypeSecret && !manifest.SubprocessInput(input) {
			continue
		}
		if _, exact := headerInputs[input.Binding.Name]; exact {
			continue
		}
		if _, found := state.Values[input.ID]; found {
			continue
		}
		for _, binding := range effective {
			if binding.InputID == "" && binding.EnvName == input.Binding.Name && binding.MCPServer == "" {
				if err := r.addBindingState(ctx, state, manifest, input.ID, binding); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func preferStoredInput(
	manifest *extensionpkg.Manifest, state extensionpkg.InputState, binding extensionpkg.EnvBinding,
) bool {
	row, exists := state.Values[binding.InputID]
	if !exists {
		return false
	}
	// A retired env name must not overwrite the current binding for the same input ID.
	if row.Type == extensionInputTypeSecret {
		return row.Active && binding.Inactive
	}
	if manifest != nil {
		for _, input := range manifest.Inputs {
			if input.ID != binding.InputID {
				continue
			}
			if manifest.SubprocessInput(input) {
				// An active binding outranks a retired row for the same subprocess input.
				return row.Active || binding.Inactive
			}
			return input.Type != extensionInputTypeSecret
		}
	}
	return row.Active && binding.Inactive
}

// addBindingState records a binding-backed input under its declared type. A non-secret subprocess
// value is resolved inside the daemon and re-checked against the current manifest type (rule 3a).
func (r extensionInputReader) addBindingState(
	ctx context.Context, state *extensionpkg.InputState, manifest *extensionpkg.Manifest, id string,
	binding extensionpkg.EnvBinding,
) error {
	record := extensionpkg.InputValueRecord{
		Type:      extensionInputTypeSecret,
		Active:    !binding.Inactive,
		UpdatedAt: binding.UpdatedAt,
	}
	input, declared := manifestInput(manifest, id)
	if declared && input.Type != extensionInputTypeSecret && manifest.SubprocessInput(input) {
		record.Type = input.Type
		if record.Active {
			if err := r.recheckBindingValue(ctx, &record, input, binding.SecretRef); err != nil {
				return err
			}
		}
		state.Values[id] = record
		return nil
	}
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

// recheckBindingValue keeps the plaintext inside the daemon: errors carry the input id and a reason
// class only, and the value is registered for redaction while it is checked.
func (r extensionInputReader) recheckBindingValue(
	ctx context.Context, record *extensionpkg.InputValueRecord, input extensionpkg.ManifestInput, ref string,
) error {
	if r.secrets == nil {
		record.Invalid = "stored value is unavailable"
		return nil
	}
	value, err := r.secrets.ResolveRef(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) || errors.Is(err, vault.ErrMissingSecret) {
		record.Invalid = "stored value is unavailable"
		return nil
	}
	if err != nil {
		return fmt.Errorf("daemon: resolve extension input %q value: %w", input.ID, err)
	}
	cleanup := diagnostics.RegisterDynamicSecret(value)
	defer cleanup()
	normalized, err := marketplace.NormalizeMCPInputValue(input.Type, value)
	if err != nil {
		record.Invalid = "stored value does not match the manifest type"
		return nil
	}
	raw := json.RawMessage(normalized)
	if input.Type != "boolean" {
		raw, _ = json.Marshal(normalized)
	}
	record.Value = raw
	record.SecretRef = ref
	return nil
}

func manifestInput(manifest *extensionpkg.Manifest, id string) (extensionpkg.ManifestInput, bool) {
	if manifest == nil {
		return extensionpkg.ManifestInput{}, false
	}
	for _, input := range manifest.Inputs {
		if input.ID == id {
			return input, true
		}
	}
	return extensionpkg.ManifestInput{}, false
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
