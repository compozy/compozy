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
import { useProfileReadScope } from "@/systems/profiles";
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

export function useLoopRunForm({
  workspaceId,
  loop,
  effectiveConfig,
  gitBacked,
  onRunStarted,
}: UseLoopRunFormOptions) {
  const contract = loop.definition.contract;
  const schema = loop.definition.inputs;
  // The run lands in the acting profile — `default` while the aggregate is on,
  // which is exactly what the destination chip states (ADR-005).
  const { aggregate, destination } = useProfileReadScope();
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
    fieldErrors,
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
    // A dry run is evaluated as the acting profile but creates nothing, so the
    // selector still rides along while the confirmation claims no owner.
    formState.requestDryRun(() =>
      dryMutation.mutateAsync({
        workspaceId,
        name: loop.name,
        data: body,
        dry: true,
        profile: destination,
      })
    );
  }

  function handleRun() {
    if (!valid) {
      formState.markSubmissionAttempted();
      return;
    }
    if (busy) return;
    const body = requestBody();
    formState.requestRun(
      loop.name,
      () =>
        runMutation.mutateAsync({
          workspaceId,
          name: loop.name,
          data: body,
          dry: false,
          profile: destination,
        }),
      aggregate
    );
  }

  return {
    contract,
    schema,
    inputs,
    fieldErrors,
    overrides,
    networkParticipation,
    configOverrides,
    plan,
    submitAttempted,
    missing,
    valid,
    busy,
    /** The destination chip's value: a profile name under the aggregate, else null. */
    profileDestination: aggregate ? destination : null,
    pendingKind: pendingRequest?.kind ?? null,
    setInput,
    setOverridesDraft,
    setNetworkParticipationDraft,
    handleDryRun,
    handleRun,
  };
}
