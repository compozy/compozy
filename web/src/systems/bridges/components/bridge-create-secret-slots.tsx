import { KeyRound } from "lucide-react";

import { Alert, AlertDescription, FormSection, RequiredMark, SecretField } from "@agh/ui";

import { describeBridgeSecretSlot } from "../lib/bridge-formatters";
import type { BridgeProviderSecretSlot } from "../types";

export interface BridgeCreateSecretSlotsProps {
  slots: readonly BridgeProviderSecretSlot[] | undefined;
  values: Readonly<Record<string, string>>;
  onValueChange: (name: string, value: string) => void;
  /** Slot names whose binding write failed after the bridge was created. */
  failures?: Readonly<Record<string, string>>;
  /** Slot names already bound — rendered as presence, never as a value. */
  bound?: readonly string[];
  /** Set once phase one committed, so a bound slot can name its real reference. */
  bridgeId?: string | null;
  saving?: boolean;
  /** Manifest providers hand credentials over only after the remote app exists. */
  deferredHint?: string;
}

/**
 * Credential slots declared by the bridge provider manifest.
 *
 * The values never enter `provider_config` — the contract rejects secret-shaped
 * JSON there — and never enter the create body either. They are written as
 * per-slot bindings once the bridge exists, so a failed write leaves the slot
 * empty rather than re-displaying what was typed.
 */
export function BridgeCreateSecretSlots({
  slots,
  values,
  onValueChange,
  failures,
  bound,
  bridgeId,
  saving,
  deferredHint,
}: BridgeCreateSecretSlotsProps) {
  const declared = slots ?? [];
  const boundSlots = new Set(bound ?? []);
  return (
    <FormSection
      data-testid="bridge-create-section-credentials"
      description={
        declared.length > 0
          ? "Values go straight to the vault — the bridge configuration stores only references, never plaintext."
          : "This provider declares no credential slots in its manifest."
      }
      icon={KeyRound}
      title="Credentials"
    >
      {deferredHint && declared.length > 0 ? (
        <Alert data-testid="bridge-create-secret-deferred" variant="neutral">
          <AlertDescription>{deferredHint}</AlertDescription>
        </Alert>
      ) : null}

      {declared.map(slot => {
        const required = slot.required !== false;
        const isBound = boundSlots.has(slot.name);
        return (
          <SecretField
            badges={required ? <RequiredMark /> : null}
            description={describeBridgeSecretSlot(slot)}
            error={failures?.[slot.name]}
            id={`bridge-create-secret-${slot.name}`}
            key={slot.name}
            label={slot.name}
            labelClassName="font-mono"
            onValueChange={next => onValueChange(slot.name, next)}
            placeholder={`slot · ${slot.name}`}
            present={isBound}
            presenceLabel={
              isBound && bridgeId ? `vault:bridges/${bridgeId}/${slot.name}` : undefined
            }
            required={required}
            saving={saving}
            testIdPrefix={`bridge-create-secret-${slot.name}`}
            value={values[slot.name] ?? ""}
          />
        );
      })}
    </FormSection>
  );
}
