import { useState } from "react";

import { toast } from "@agh/ui";
import {
  networkParticipationDraftFromPayload,
  isNetworkParticipationDraftValid,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/systems/network";

import {
  buildConfigOverrides,
  initialOverrideDraft,
  type LoopOverrideDraft,
} from "../lib/loop-overrides";
import {
  initialRunInputs,
  isRunFormValid,
  type LoopRunInputs,
  missingRequiredInputs,
  serializeRunInputs,
} from "../lib/loop-run-form";
import type { LoopDetail, LoopDryRunPreview, LoopEffectiveConfig } from "../types";
import { useRunLoop } from "./use-loop-actions";

interface UseLoopRunFormOptions {
  workspaceId: string;
  loop: LoopDetail;
  effectiveConfig: LoopEffectiveConfig;
  onRunStarted?: (runId: string) => void;
}

/**
 * View-model for the run form (§4.3): owns the input + override draft (local UI
 * state), derives required-input validity, and drives the Dry run / Run calls through
 * the sanctioned mutation hook. A Dry run stores the returned gen-1 plan; a Run hands
 * the started run's id back for navigation. Any input/override edit clears the last
 * plan (it no longer matches).
 */
export function useLoopRunForm({
  workspaceId,
  loop,
  effectiveConfig,
  onRunStarted,
}: UseLoopRunFormOptions) {
  const contract = loop.definition.contract;
  const schema = loop.definition.inputs;
  const [inputs, setInputs] = useState<LoopRunInputs>(() => initialRunInputs(schema));
  const [overrides, setOverrides] = useState<LoopOverrideDraft>(() =>
    initialOverrideDraft(effectiveConfig)
  );
  const [networkParticipation, setNetworkParticipation] = useState<NetworkParticipationDraft>(() =>
    networkParticipationDraftFromPayload(loop.definition.network_participation)
  );
  const [networkParticipationOverridden, setNetworkParticipationOverridden] = useState(false);
  const [plan, setPlan] = useState<LoopDryRunPreview | null>(null);
  const [submitAttempted, setSubmitAttempted] = useState(false);

  const runMutation = useRunLoop();
  const dryMutation = useRunLoop();
  const missing = new Set(missingRequiredInputs(schema, inputs));
  const valid =
    isRunFormValid(schema, inputs) &&
    (!networkParticipationOverridden ||
      isNetworkParticipationDraftValid(networkParticipation, ["named", "loop_run"]));
  const busy = runMutation.isPending || dryMutation.isPending;
  const configOverrides = buildConfigOverrides(overrides, effectiveConfig);

  function requestBody() {
    return {
      inputs: serializeRunInputs(schema, inputs),
      config_overrides: configOverrides,
      ...(networkParticipationOverridden
        ? { network_participation: serializeNetworkParticipation(networkParticipation) }
        : {}),
    };
  }

  function setInput(name: string, value: unknown) {
    setPlan(null);
    setInputs(prev => ({ ...prev, [name]: value }));
  }

  function setOverridesDraft(next: LoopOverrideDraft) {
    setPlan(null);
    setOverrides(next);
  }

  function setNetworkParticipationDraft(next: NetworkParticipationDraft) {
    setPlan(null);
    setNetworkParticipationOverridden(true);
    setNetworkParticipation(next);
  }

  function handleDryRun() {
    setSubmitAttempted(true);
    if (!valid || busy) return;
    dryMutation.mutate(
      { workspaceId, name: loop.name, data: requestBody(), dry: true },
      {
        onSuccess: result => {
          setPlan(result.dry_run ?? null);
          toast.success("Dry run passed — inputs valid, plan rendered");
        },
        onError: error => toast.error(error instanceof Error ? error.message : "Dry run failed"),
      }
    );
  }

  function handleRun() {
    setSubmitAttempted(true);
    if (!valid || busy) return;
    runMutation.mutate(
      { workspaceId, name: loop.name, data: requestBody(), dry: false },
      {
        onSuccess: result => {
          const runId = result.run?.id;
          toast.success(runId ? `Run started · ${runId}` : "Run started");
          if (runId) onRunStarted?.(runId);
        },
        onError: error =>
          toast.error(error instanceof Error ? error.message : `Failed to run ${loop.name}`),
      }
    );
  }

  return {
    contract,
    schema,
    inputs,
    overrides,
    networkParticipation,
    configOverrides,
    plan,
    submitAttempted,
    missing,
    valid,
    busy,
    setInput,
    setOverridesDraft,
    setNetworkParticipationDraft,
    handleDryRun,
    handleRun,
  };
}
