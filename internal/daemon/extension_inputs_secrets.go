package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionenv"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/vault"
)

func (b extensionInputBinder) prepareSecret(
	ctx context.Context,
	plan *extensionInputPlan,
	input extensionpkg.ManifestInput,
	value extensioninput.Value,
	supplied bool,
) error {
	if !supplied {
		prior, found := plan.state.Values[input.ID]
		if !found || prior.Type != extensionInputTypeSecret || prior.SecretRef == "" {
			return nil
		}
		prior.Active = true
		plan.state.Values[input.ID] = prior
		write := preparedExtensionSecret{
			envName: input.Binding.Name, ref: prior.SecretRef, inputID: input.ID,
		}
		if !inputSecretUnchanged(plan, write) {
			plan.writes = append(plan.writes, write)
		}
		return nil
	}
	if b.service.secretVault == nil || b.service.envBindings == nil {
		return errors.New("daemon: extension input secret storage is required")
	}
	request := contract.ExtensionSecretBindingInput{EnvName: input.Binding.Name, VaultRef: value.VaultRef}
	if value.VaultRef == nil {
		raw, err := extensionpkg.NormalizeInputJSON(extensionInputTypeSecret, value.Value)
		if err != nil {
			return inputInvalid(input.ID, err.Error())
		}
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return inputInvalid(input.ID, "secret must be a string")
		}
		request.Value = &text
	} else if err := validateExtensionInputSecretRef(plan, *value.VaultRef); err != nil {
		return inputInvalid(input.ID, err.Error())
	}
	ref, err := b.prepareInputBinding(ctx, plan, input.ID, request)
	if err != nil {
		return err
	}
	plan.state.Values[input.ID] = extensionpkg.InputValueRecord{
		Type: extensionInputTypeSecret, SecretRef: ref, Active: true, UpdatedAt: b.service.now().UTC(),
	}
	return nil
}

// prepareSubprocessValue stores a supplied non-secret subprocess input as an instance-owned Vault
// secret bound to its env name; an omitted value keeps its prior binding (rule 3a).
func (b extensionInputBinder) prepareSubprocessValue(
	ctx context.Context,
	plan *extensionInputPlan,
	input extensionpkg.ManifestInput,
	value extensioninput.Value,
	supplied bool,
) error {
	if !supplied || value.VaultRef != nil {
		return b.prepareValue(plan, input, value, supplied)
	}
	normalized, err := extensionpkg.NormalizeInputJSON(input.Type, value.Value)
	if err != nil {
		return inputInvalid(input.ID, err.Error())
	}
	var decoded any
	if err := json.Unmarshal(normalized, &decoded); err != nil {
		return inputInvalid(input.ID, "value must be valid JSON")
	}
	text := fmt.Sprint(decoded)
	if strings.TrimSpace(text) == "" {
		return inputInvalid(input.ID, "subprocess input value must not be blank")
	}
	if b.service.secretVault == nil || b.service.envBindings == nil {
		return errors.New("daemon: extension input secret storage is required")
	}
	ref, err := b.prepareInputBinding(ctx, plan, input.ID,
		contract.ExtensionSecretBindingInput{EnvName: input.Binding.Name, Value: &text})
	if err != nil {
		return err
	}
	plan.state.Values[input.ID] = extensionpkg.InputValueRecord{
		Type: input.Type, Value: normalized, SecretRef: ref, Active: true, UpdatedAt: b.service.now().UTC(),
	}
	return nil
}

func (b extensionInputBinder) prepareInputBinding(
	ctx context.Context, plan *extensionInputPlan, inputID string, request contract.ExtensionSecretBindingInput,
) (string, error) {
	write, err := b.service.prepareExtensionSecret(ctx, plan.key, plan.instance.ProfileID, request)
	if err != nil {
		return "", inputInvalidWithCause(inputID, "value could not be prepared", err)
	}
	write.inputID = inputID
	if write.value != nil {
		snapshot, err := b.service.snapshotExtensionSecret(ctx, write.ref)
		if err != nil {
			return "", inputInvalidWithCause(inputID, "value could not be snapshotted", err)
		}
		write.snapshot = &snapshot
	}
	if !inputSecretUnchanged(plan, write) {
		plan.writes = append(plan.writes, write)
	}
	return write.ref, nil
}

func validateExtensionInputSecretRef(plan *extensionInputPlan, value string) error {
	ref := vault.NormalizeRef(value)
	if err := vault.ValidateSecretRefNamespace(ref, "extensions"); err != nil {
		return errors.New("invalid extension secret reference")
	}
	if !strings.HasPrefix(
		ref,
		vault.ExtensionProfileSecretOwnerPrefix(plan.key.Name, plan.instance.ProfileID, plan.key.WorkspaceID),
	) {
		return errors.New("secret reference is outside the extension instance")
	}
	return nil
}

func inputSecretUnchanged(plan *extensionInputPlan, write preparedExtensionSecret) bool {
	before, found := plan.previous[write.envName]
	if !found || before.SecretRef != write.ref || before.InputID != write.inputID ||
		before.Kind != extensionenv.BindingKind || before.Inactive ||
		before.MCPServer != write.mcpServer || before.HeaderName != write.headerName {
		return false
	}
	return write.value == nil ||
		(write.snapshot != nil && write.snapshot.existed && write.snapshot.value == *write.value)
}
