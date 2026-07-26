import { Plus, X } from "lucide-react";
import { useState } from "react";

import { Button, Eyebrow, Input, RequiredMark, SecretField, Switch } from "@agh/ui";

import {
  addProviderCredentialSlot,
  isVaultSecretRef,
  providerCredentialSecretValue,
  removeProviderCredentialSlot,
  setProviderCredentialSecretValue,
  updateProviderCredentialSlot,
} from "../lib/provider-draft";
import type { ProviderCredentialSlotDraft, ProviderDraft, SettingsProviderEntry } from "../types";
import type { ProviderDraftChange } from "./provider-edit-form";
import { ModalSettingsFieldRow } from "./settings-field-row";

interface ProviderCredentialFieldsProps {
  mode: "create" | "edit";
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

type CredentialPresence = { present: boolean; secretRef: string };

/**
 * Credential slots for `auth_mode = bound_secret`.
 *
 * Only a `vault:` ref can hold a value AGH writes; an `env:` ref is read from
 * the operator environment at launch, so no write control is offered for it.
 * On edit the stored value is never read back — presence comes from
 * `entry.credentials[].present` and replacing it is an explicit rotation.
 */
export function ProviderCredentialFields({
  mode,
  draft,
  entry,
  onChange,
}: ProviderCredentialFieldsProps) {
  const [rotating, setRotating] = useState<Record<number, boolean>>({});
  const presence = credentialPresence(entry);

  const removeSlot = (index: number) => {
    onChange(current => removeProviderCredentialSlot(current, index));
    setRotating(current => shiftRotationState(current, index));
  };

  return (
    <div className="flex flex-col gap-3" data-testid="settings-providers-editor-credential-slots">
      {draft.credential_slots.map((slot, index) => (
        <CredentialSlotBlock
          index={index}
          isEditing={Boolean(rotating[index])}
          key={slot.key}
          mode={mode}
          onChange={onChange}
          onEditingChange={editing => setRotating(current => ({ ...current, [index]: editing }))}
          onRemove={draft.credential_slots.length > 1 ? () => removeSlot(index) : undefined}
          presence={presence.get(slot.name.trim())}
          secretValue={providerCredentialSecretValue(draft, index)}
          slot={slot}
        />
      ))}
      <Button
        className="w-fit"
        data-testid="settings-providers-editor-add-credential-slot"
        onClick={() => onChange(addProviderCredentialSlot)}
        size="sm"
        type="button"
        variant="outline"
      >
        <Plus aria-hidden="true" className="size-3" />
        Add credential slot
      </Button>
    </div>
  );
}

interface CredentialSlotBlockProps {
  index: number;
  mode: "create" | "edit";
  slot: ProviderCredentialSlotDraft;
  secretValue: string;
  presence: CredentialPresence | undefined;
  isEditing: boolean;
  onEditingChange: (editing: boolean) => void;
  onChange: ProviderDraftChange;
  onRemove?: () => void;
}

function CredentialSlotBlock({
  index,
  mode,
  slot,
  secretValue,
  presence,
  isEditing,
  onEditingChange,
  onChange,
  onRemove,
}: CredentialSlotBlockProps) {
  const testId = `settings-providers-editor-credential-slot-${index}`;
  const vaultBacked = isVaultSecretRef(slot.secret_ref);
  const present = mode === "edit" && Boolean(presence?.present);

  return (
    <section className="flex flex-col gap-3 rounded-md bg-canvas px-3 py-3" data-testid={testId}>
      <header className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-mono text-mono-id tracking-normal text-muted">
          {slot.name.trim() || `slot ${index + 1}`}
        </span>
        {onRemove ? (
          <Button
            aria-label={`Remove credential slot ${index + 1}`}
            data-testid={`${testId}-remove`}
            onClick={onRemove}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        ) : null}
      </header>

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56 font-mono"
            data-testid={`${testId}-name-input`}
            onChange={event =>
              onChange(current =>
                updateProviderCredentialSlot(current, index, { name: event.target.value })
              )
            }
            placeholder="api_key"
            value={slot.name}
          />
        }
        data-testid={`${testId}-name`}
        description="Stable credential slot declared by the provider."
        label={
          <>
            Slot name
            <RequiredMark />
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56 font-mono"
            data-testid={`${testId}-target-env-input`}
            onChange={event =>
              onChange(current =>
                updateProviderCredentialSlot(current, index, { target_env: event.target.value })
              )
            }
            placeholder="OPENAI_API_KEY"
            value={slot.target_env}
          />
        }
        data-testid={`${testId}-target-env`}
        description="Environment variable injected into the provider subprocess."
        label={
          <>
            Target env
            <RequiredMark />
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid={`${testId}-secret-ref-input`}
            onChange={event =>
              onChange(current =>
                updateProviderCredentialSlot(current, index, { secret_ref: event.target.value })
              )
            }
            placeholder="vault:providers/openai/api-key"
            value={slot.secret_ref}
          />
        }
        data-testid={`${testId}-secret-ref`}
        description="Bound credential source. Use env: to read the operator environment, vault: to store the value in AGH."
        label={
          <>
            Secret ref
            <RequiredMark />
          </>
        }
      />

      {vaultBacked ? (
        <SecretField
          badges={<Eyebrow className="text-subtle">write-only</Eyebrow>}
          description={
            present
              ? "The stored value is never revealed. Rotate only when replacing it."
              : "Stored in the AGH vault under this ref. It is never displayed again."
          }
          editing={isEditing}
          id={`${testId}-value`}
          label="Credential value"
          onEditingChange={onEditingChange}
          onValueChange={next =>
            onChange(current => setProviderCredentialSecretValue(current, index, next))
          }
          placeholder="Enter write-only value"
          present={present}
          presenceLabel={`${presence?.secretRef || slot.secret_ref} → ${slot.target_env}`}
          replaceLabel="Rotate"
          testIdPrefix={`${testId}-value`}
          value={secretValue}
        />
      ) : (
        <p className="text-form-hint text-subtle" data-testid={`${testId}-value-unavailable`}>
          Set a value only when the ref is managed by the AGH vault. This ref resolves at launch
          from outside AGH.
        </p>
      )}

      <div className="flex items-center gap-4">
        <div className="min-w-0 flex-1">
          <p className="text-form-label font-medium text-fg">Required credential</p>
          <p className="text-form-hint text-subtle">
            Block provider activation until this slot reference resolves.
          </p>
        </div>
        <Switch
          aria-label={`Credential slot ${index + 1} required`}
          checked={slot.required}
          data-testid={`${testId}-required`}
          onCheckedChange={checked =>
            onChange(current => updateProviderCredentialSlot(current, index, { required: checked }))
          }
        />
      </div>
    </section>
  );
}

function credentialPresence(entry: SettingsProviderEntry | null): Map<string, CredentialPresence> {
  const presence = new Map<string, CredentialPresence>();
  for (const credential of entry?.credentials ?? []) {
    presence.set(credential.name, {
      present: credential.present,
      secretRef: credential.secret_ref,
    });
  }
  return presence;
}

/** Keeps open rotations attached to their slot after one is removed. */
function shiftRotationState(
  rotating: Record<number, boolean>,
  removedIndex: number
): Record<number, boolean> {
  const next: Record<number, boolean> = {};
  for (const [key, value] of Object.entries(rotating)) {
    const index = Number(key);
    if (index === removedIndex) continue;
    next[index > removedIndex ? index - 1 : index] = value;
  }
  return next;
}
