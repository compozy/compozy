package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
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
		if !found || prior.Type != "secret" || prior.SecretRef == "" {
			return nil
		}
		prior.Active = true
		plan.state.Values[input.ID] = prior
		before, exact := plan.previous[input.Binding.Name]
		if exact && before.SecretRef == prior.SecretRef && (before.Inactive || before.InputID != input.ID) {
			plan.writes = append(plan.writes, preparedExtensionSecret{
				envName: input.Binding.Name, ref: prior.SecretRef, inputID: input.ID,
			})
		}
		return nil
	}
	if b.service.secretVault == nil || b.service.envBindings == nil {
		return errors.New("daemon: extension input secret storage is required")
	}
	request := contract.ExtensionSecretBindingInput{EnvName: input.Binding.Name, VaultRef: value.VaultRef}
	if value.VaultRef == nil {
		raw, err := extensionpkg.NormalizeInputJSON("secret", value.Value)
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
	write, err := b.service.prepareExtensionSecret(ctx, plan.key, plan.instance.ProfileID, request)
	if err != nil {
		return inputInvalid(input.ID, "secret could not be prepared")
	}
	write.inputID = input.ID
	if write.value != nil {
		snapshot, err := b.service.snapshotExtensionSecret(ctx, write.ref)
		if err != nil {
			return inputInvalid(input.ID, "secret could not be snapshotted")
		}
		write.snapshot = &snapshot
	}
	plan.writes = append(plan.writes, write)
	plan.state.Values[input.ID] = extensionpkg.InputValueRecord{
		Type: "secret", SecretRef: write.ref, Active: true, UpdatedAt: b.service.now().UTC(),
	}
	return nil
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
