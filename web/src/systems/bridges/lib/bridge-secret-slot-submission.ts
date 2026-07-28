import type { BridgeProviderSecretSlot } from "../types";

/** A slot value the operator typed in the create dialog. Never read back. */
export interface BridgeSecretSlotDraft {
  name: string;
  value: string;
}

export interface BridgeSecretSlotFailure {
  name: string;
  message: string;
}

export interface BridgeSecretSlotOutcome {
  bridgeId: string;
  /** Slot names whose binding was written. */
  succeeded: string[];
  /** Slots that still need a value; their plaintext is deliberately dropped. */
  failed: BridgeSecretSlotFailure[];
}

export type BridgeSecretSlotWriter = (input: {
  bridgeId: string;
  name: string;
  value: string;
}) => Promise<unknown>;

/** Slots the operator actually filled, in provider-declared order. */
export function collectFilledBridgeSecretSlots(
  slots: readonly BridgeProviderSecretSlot[] | undefined,
  values: Readonly<Record<string, string>>
): BridgeSecretSlotDraft[] {
  if (!slots?.length) return [];
  return slots.flatMap(slot => {
    const value = values[slot.name]?.trim() ?? "";
    return value.length > 0 ? [{ name: slot.name, value }] : [];
  });
}

/** Required slots left empty; these gate submit before the bridge is created. */
export function missingRequiredBridgeSecretSlots(
  slots: readonly BridgeProviderSecretSlot[] | undefined,
  values: Readonly<Record<string, string>>
): string[] {
  if (!slots?.length) return [];
  return slots.flatMap(slot => {
    if (slot.required === false) return [];
    return (values[slot.name]?.trim() ?? "").length > 0 ? [] : [slot.name];
  });
}

/**
 * Phase two of bridge creation: bind the filled slots to the bridge that phase
 * one already created.
 *
 * `CreateBridgeRequest` carries no secret field, so the plaintext can only reach
 * the daemon through per-slot binding writes after the instance exists. Every
 * slot is attempted even when an earlier one fails — the bridge is already
 * durable, so the operator needs the complete list of what still has to be
 * bound, not the first error. Failures carry the slot name and message only:
 * the value never travels back so it can never be re-displayed.
 */
export async function submitBridgeSecretSlots(
  bridgeId: string,
  drafts: readonly BridgeSecretSlotDraft[],
  write: BridgeSecretSlotWriter
): Promise<BridgeSecretSlotOutcome> {
  const succeeded: string[] = [];
  const failed: BridgeSecretSlotFailure[] = [];
  for (const draft of drafts) {
    try {
      await write({ bridgeId, name: draft.name, value: draft.value });
      succeeded.push(draft.name);
    } catch (error) {
      failed.push({
        name: draft.name,
        message:
          error instanceof Error && error.message.trim().length > 0
            ? error.message
            : `Failed to bind ${draft.name}.`,
      });
    }
  }
  return { bridgeId, succeeded, failed };
}
