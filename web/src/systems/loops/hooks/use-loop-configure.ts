import { useState } from "react";

import { toast } from "@compozy/ui";

import { buildCheckDescriptors, type LoopConfigCheckDescriptor } from "../lib/loop-config-checks";
import {
  buildLoopConfigRequest,
  initialConfigDraft,
  normalizeLoopEnvironment,
  resetConfigDraft,
  type LoopConfigDraft,
  type LoopReattemptStrategy,
} from "../lib/loop-config-draft";
import type { LoopOverrideDraft } from "../lib/loop-overrides";
import type { LoopConfig, LoopDetail, LoopEffectiveConfig, LoopEnvironmentSpec } from "../types";
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
  /** `null` unpins the loop-level default so every node falls back to the workspace root. */
  setEnvironment: (environment: LoopEnvironmentSpec | null) => void;
  handleReset: () => void;
  handleSave: () => void;
}

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

  function setEnvironment(environment: LoopEnvironmentSpec | null) {
    setDraft(prev => ({ ...prev, environment: normalizeLoopEnvironment(environment) }));
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
    setEnvironment,
    handleReset,
    handleSave,
  };
}
