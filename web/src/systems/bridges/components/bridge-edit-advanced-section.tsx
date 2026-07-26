import { KeyRound, Waypoints } from "lucide-react";

import {
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
  FormSection,
  SecretField,
  Textarea,
} from "@agh/ui";

import { describeBridgeProviderConfigSchema } from "../lib/bridge-formatters";
import type { BridgeProvider, BridgeUpdateDraft } from "../types";
import { BridgeDeliveryFields, BridgeRoutingFields } from "./bridge-delivery-fields";

/** Rotation state for one provider-declared credential slot. */
export interface BridgeSecretRotation {
  name: string;
  /** Presence reported by the read path — a value is never returned. */
  present: boolean;
  /** The stored reference, safe to display; the secret behind it is not. */
  secretRef?: string;
  value: string;
  editing: boolean;
  saving?: boolean;
  rotated?: boolean;
  error?: string;
}

export interface BridgeEditAdvancedSectionProps {
  draft: BridgeUpdateDraft;
  onDraftChange: (draft: BridgeUpdateDraft) => void;
  provider?: BridgeProvider;
  providerConfigError?: string;
  rotations: readonly BridgeSecretRotation[];
  onRotationValueChange?: (name: string, value: string) => void;
  onRotationEditingChange?: (name: string, editing: boolean) => void;
}

/**
 * Advanced tier of the bridge editor: rotation, routing, delivery, and the
 * provider config blob.
 *
 * Credentials hydrate as presence plus their stored reference — never a value —
 * because every AGH secret read path returns presence only.
 */
export function BridgeEditAdvancedSection({
  draft,
  onDraftChange,
  provider,
  providerConfigError,
  rotations,
  onRotationValueChange,
  onRotationEditingChange,
}: BridgeEditAdvancedSectionProps) {
  const configSchema = describeBridgeProviderConfigSchema(provider?.config_schema);

  return (
    <>
      {rotations.length > 0 && onRotationValueChange && onRotationEditingChange ? (
        <FormSection
          data-testid="bridge-edit-section-credentials"
          description="Only presence is returned. Rotate a slot to send a new write-only value."
          icon={KeyRound}
          title="Credentials"
        >
          {rotations.map(rotation => (
            <SecretField
              editing={rotation.editing}
              error={rotation.error}
              id={`bridge-edit-secret-${rotation.name}`}
              key={rotation.name}
              label={rotation.name}
              labelClassName="font-mono"
              onEditingChange={editing => onRotationEditingChange(rotation.name, editing)}
              onValueChange={value => onRotationValueChange(rotation.name, value)}
              placeholder={`New value for ${rotation.name}`}
              present={rotation.present}
              presenceLabel={rotation.secretRef}
              replaceLabel="Rotate"
              rotated={rotation.rotated}
              saving={rotation.saving}
              testIdPrefix={`bridge-edit-secret-${rotation.name}`}
              value={rotation.value}
            />
          ))}
        </FormSection>
      ) : null}

      <FormSection
        data-testid="bridge-edit-section-routing"
        description="Optional policies from the bridge update contract."
        icon={Waypoints}
        title="Routing & delivery"
      >
        <BridgeRoutingFields
          onChange={routingPolicy => onDraftChange({ ...draft, routingPolicy })}
          testIdPrefix="bridge-edit"
          value={draft.routingPolicy}
        />

        <BridgeDeliveryFields
          onChange={deliveryDefaults => onDraftChange({ ...draft, deliveryDefaults })}
          testIdPrefix="bridge-edit"
          value={draft.deliveryDefaults}
        />

        <Field>
          <FieldContent>
            <FieldTitle>Provider configuration</FieldTitle>
            <FieldDescription>
              Non-secret JSON for provider settings such as tenant identifiers or mode flags.{" "}
              {configSchema}
            </FieldDescription>
          </FieldContent>
          <Textarea
            aria-invalid={Boolean(providerConfigError)}
            aria-label="Provider configuration JSON"
            className="min-h-32 font-mono text-xs"
            data-testid="bridge-edit-provider-config-input"
            onChange={event => onDraftChange({ ...draft, providerConfigText: event.target.value })}
            placeholder={`{\n  "mode": "bot"\n}`}
            spellCheck={false}
            value={draft.providerConfigText}
          />
          {providerConfigError ? (
            <p
              className="text-small-body text-danger"
              data-testid="bridge-edit-provider-config-error"
            >
              {providerConfigError}
            </p>
          ) : null}
        </Field>
      </FormSection>
    </>
  );
}
