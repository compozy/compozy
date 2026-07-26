import { useState } from "react";

import { toast } from "@agh/ui";

import { buildCheckDescriptors, type LoopConfigCheckDescriptor } from "../lib/loop-config-checks";
import {
  buildLoopConfigRequest,
  initialConfigDraft,
  resetConfigDraft,
  type LoopConfigDraft,
  type LoopReattemptStrategy,
} from "../lib/loop-config-draft";
import type { LoopOverrideDraft } from "../lib/loop-overrides";
import type { LoopConfig, LoopDetail, LoopEffectiveConfig } from "../types";
import { usePutLoopConfig } from "./use-loop-actions";

interface UseLoopConfigureOptions {
  workspaceId: string;
  loop: LoopDetail;
  /** The stored per-loop config (`loop_config`), or null when the loop uses inherited defaults. */
  config: LoopConfig | null;
  effectiveConfig: LoopEffectiveConfig;
  /** Called after a successful save so the route can close the sheet. */
  onSaved?: () => void;
}

export interface UseLoopConfigureResult {
  descriptors: LoopConfigCheckDescriptor[];
  draft: LoopConfigDraft;
  busy: boolean;
  toggleCheck: (id: string, enabled: boolean) => void;
  setCheckCommand: (id: string, command: string) => void;
  setHumanGate: (enabled: boolean) => void;
  setStrategy: (strategy: LoopReattemptStrategy) => void;
  setLimits: (limits: LoopOverrideDraft) => void;
  handleReset: () => void;
  handleSave: () => void;
}

/**
 * View-model for the no-fork configure sheet (§4.7): owns the editable draft (seeded from
 * the stored `loop_config` overlaid on the contract defaults) and drives the `PUT /config`
 * write through the sanctioned mutation hook. Reset restores the inherited defaults locally
 * (no write); Save persists the whole draft and hands control back for closing.
 */
export function useLoopConfigure({
  workspaceId,
  loop,
  config,
  effectiveConfig,
  onSaved,
}: UseLoopConfigureOptions): UseLoopConfigureResult {
  const contract = loop.definition.contract;
  const descriptors = buildCheckDescriptors(contract);
  const [draft, setDraft] = useState<LoopConfigDraft>(() =>
    initialConfigDraft(descriptors, config, effectiveConfig)
  );
  const mutation = usePutLoopConfig();

  function toggleCheck(id: string, enabled: boolean) {
    setDraft(prev => ({
      ...prev,
      checks: { ...prev.checks, [id]: { ...prev.checks[id], enabled } },
    }));
  }

  function setCheckCommand(id: string, command: string) {
    setDraft(prev => ({
      ...prev,
      checks: { ...prev.checks, [id]: { ...prev.checks[id], command } },
    }));
  }

  function setHumanGate(enabled: boolean) {
    setDraft(prev => ({ ...prev, humanGateEnabled: enabled }));
  }

  function setStrategy(strategy: LoopReattemptStrategy) {
    setDraft(prev => ({ ...prev, reattemptStrategy: strategy }));
  }

  function setLimits(limits: LoopOverrideDraft) {
    setDraft(prev => ({ ...prev, limits }));
  }

  function handleReset() {
    setDraft(resetConfigDraft(descriptors, effectiveConfig));
    toast.success("Reset to loop defaults");
  }

  function handleSave() {
    mutation.mutate(
      {
        workspaceId,
        name: loop.name,
        data: buildLoopConfigRequest(draft, descriptors),
      },
      {
        onSuccess: () => {
          toast.success("Configuration saved · applies to future runs");
          onSaved?.();
        },
        onError: error =>
          toast.error(
            error instanceof Error ? error.message : `Failed to save ${loop.name} configuration`
          ),
      }
    );
  }

  return {
    descriptors,
    draft,
    busy: mutation.isPending,
    toggleCheck,
    setCheckCommand,
    setHumanGate,
    setStrategy,
    setLimits,
    handleReset,
    handleSave,
  };
}
