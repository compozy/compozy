import { useEffect, useEffectEvent } from "react";
import {
  networkParticipationDraftFromPayload,
  isNetworkParticipationDraftValid,
  serializeNetworkParticipation,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";

import {
  buildConfigOverrides,
  isLoopEnvironmentOverrideValid,
  type LoopOverrideDraft,
} from "../lib/loop-overrides";
import { isRunFormValid, missingRequiredInputs, serializeRunInputs } from "../lib/loop-run-form";
import type { LoopDetail, LoopEffectiveConfig } from "../types";
import { useRunLoop } from "./use-loop-actions";
import { useLoopRunFormState } from "./use-loop-run-form-state";

interface UseLoopRunFormOptions {
  workspaceId: string;
  loop: LoopDetail;
  effectiveConfig: LoopEffectiveConfig;
  gitBacked: boolean;
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
  gitBacked,
  onRunStarted,
}: UseLoopRunFormOptions) {
  const contract = loop.definition.contract;
  const schema = loop.definition.inputs;
  const formState = useLoopRunFormState({
    effectiveConfig,
    networkParticipation: networkParticipationDraftFromPayload(
      loop.definition.network_participation
    ),
    schema,
    scope: { loopName: loop.name, workspaceId },
  });
  const {
    inputs,
    networkParticipation,
    networkParticipationOverridden,
    overrides,
    pendingRequest,
    plan,
    submitAttempted,
  } = formState;

  const runMutation = useRunLoop();
  const dryMutation = useRunLoop();
  const missing = new Set(missingRequiredInputs(schema, inputs));
  const valid =
    isRunFormValid(schema, inputs) &&
    isLoopEnvironmentOverrideValid(overrides.environment, gitBacked) &&
    (!networkParticipationOverridden ||
      isNetworkParticipationDraftValid(networkParticipation, ["named", "loop_run"]));
  const busy = pendingRequest !== null;
  const configOverrides = buildConfigOverrides(overrides, effectiveConfig);
  const handleRunStarted = useEffectEvent((runId: string) => onRunStarted?.(runId));

  useEffect(() => {
    const runStarted = formState.store.on("runStarted", event => {
      if (event.runId) handleRunStarted(event.runId);
    });
    return () => {
      runStarted.unsubscribe();
    };
  }, [formState.store]);

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
    formState.setInput(name, value);
  }

  function setOverridesDraft(next: LoopOverrideDraft) {
    formState.setOverrides(next);
  }

  function setNetworkParticipationDraft(next: NetworkParticipationDraft) {
    formState.setNetworkParticipation(next);
  }

  function handleDryRun() {
    if (!valid) {
      formState.markSubmissionAttempted();
      return;
    }
    if (busy) return;
    const body = requestBody();
    formState.requestDryRun(() =>
      dryMutation.mutateAsync({ workspaceId, name: loop.name, data: body, dry: true })
    );
  }

  function handleRun() {
    if (!valid) {
      formState.markSubmissionAttempted();
      return;
    }
    if (busy) return;
    const body = requestBody();
    formState.requestRun(loop.name, () =>
      runMutation.mutateAsync({ workspaceId, name: loop.name, data: body, dry: false })
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
