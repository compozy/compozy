import { useState } from "react";
import { toast } from "sonner";

import {
  buildBridgeSecretBindingRequest,
  useDeleteBridgeSecretBinding,
  usePutBridgeSecretBinding,
  type BridgeProvider,
  type BridgeSecretBinding,
  type BridgeSecretRotation,
  type BridgeSummary,
} from "@/systems/bridges";

interface BridgeSecretRotationsInput {
  bridge: BridgeSummary | undefined;
  bindings: readonly BridgeSecretBinding[];
  provider: BridgeProvider | undefined;
  /** Invalidates the setup evidence a rotated credential just made stale. */
  onCredentialChanged: (bridgeId: string) => void;
}

function bridgeSecretDraftKey(bridgeID: string, bindingName: string) {
  return `${bridgeID}:${bindingName}`;
}

/**
 * Write-only credential state for one bridge.
 *
 * Drafts are keyed by `${bridgeId}:${binding}` so switching bridges can never
 * surface a value typed for another one, and a committed rotation clears its
 * draft immediately — the plaintext exists only between keystroke and PUT.
 */
export function useBridgeSecretRotations({
  bridge,
  bindings,
  provider,
  onCredentialChanged,
}: BridgeSecretRotationsInput) {
  const [inputValues, setInputValues] = useState<Record<string, string>>({});
  const [editingByName, setEditingByName] = useState<Record<string, boolean>>({});
  const putBinding = usePutBridgeSecretBinding();
  const deleteBinding = useDeleteBridgeSecretBinding();

  // Strip the `${bridgeId}:` prefix so consumers see bare binding names.
  let draftsByName: Record<string, string> = {};
  if (bridge) {
    const entries = new Map<string, string>();
    const prefix = `${bridge.id}:`;
    for (const [key, value] of Object.entries(inputValues)) {
      if (!key.startsWith(prefix)) continue;
      entries.set(key.slice(prefix.length), value);
    }
    draftsByName = Object.fromEntries(entries.entries());
  }

  const setDraft = (bindingName: string, value: string) => {
    if (!bridge) return;
    setInputValues(current => ({
      ...current,
      [bridgeSecretDraftKey(bridge.id, bindingName)]: value,
    }));
  };

  const setEditing = (bindingName: string, editing: boolean) => {
    setEditingByName(current => ({ ...current, [bindingName]: editing }));
  };

  const save = async (bindingName: string): Promise<boolean> => {
    if (!bridge) return false;

    const requestResult = buildBridgeSecretBindingRequest(
      bridge.id,
      bindingName,
      draftsByName[bindingName] ?? "",
      bindingName
    );
    if (!requestResult.ok) {
      toast.error(requestResult.error);
      return false;
    }

    try {
      const binding = await putBinding.mutateAsync({
        bindingName,
        data: requestResult.data,
        id: bridge.id,
      });

      setInputValues(current => ({
        ...current,
        [bridgeSecretDraftKey(bridge.id, binding.binding_name)]: "",
      }));
      setEditing(bindingName, false);
      onCredentialChanged(bridge.id);
      toast.success(`Updated secret binding ${bindingName} for ${bridge.display_name}.`);
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update bridge secret");
      return false;
    }
  };

  const remove = async (bindingName: string) => {
    if (!bridge) return;

    try {
      await deleteBinding.mutateAsync({ bindingName, id: bridge.id });
      setInputValues(current => ({
        ...current,
        [bridgeSecretDraftKey(bridge.id, bindingName)]: "",
      }));
      onCredentialChanged(bridge.id);
      toast.success(`Deleted secret binding ${bindingName} for ${bridge.display_name}.`);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete bridge secret");
    }
  };

  const rotations: BridgeSecretRotation[] = (provider?.secret_slots ?? []).map(slot => {
    const binding = bindings.find(entry => entry.binding_name === slot.name);
    return {
      editing: editingByName[slot.name] ?? false,
      name: slot.name,
      present: Boolean(binding),
      saving: putBinding.isPending,
      secretRef: binding?.secret_ref,
      value: draftsByName[slot.name] ?? "",
    };
  });

  const pendingRotationNames: string[] = [];
  for (const [name, value] of Object.entries(draftsByName)) {
    if (value.trim().length > 0) pendingRotationNames.push(name);
  }

  /** Commits every typed rotation; stops at the first failure so it can be retried. */
  const commitPending = async (): Promise<boolean> => {
    for (const bindingName of pendingRotationNames) {
      if (!(await save(bindingName))) return false;
    }
    return true;
  };

  return {
    commitPending,
    draftsByName,
    hasPending: pendingRotationNames.length > 0,
    isPending: putBinding.isPending || deleteBinding.isPending,
    remove,
    rotations,
    save,
    setDraft,
    setEditing,
  };
}
