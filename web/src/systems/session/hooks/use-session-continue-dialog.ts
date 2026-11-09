import { useState } from "react";

import type { RuntimeSpeed } from "@/lib/api-contract";
import {
  type AgentPayload,
  normalizeRuntimeSpeed,
  resolveAgentRuntimeValue,
  useAgents,
} from "@/systems/agent";
import {
  modelSupportsReasoningEffort,
  normalizeRuntimeSelectorValue,
  runtimeACPSelectionsEqual,
  type RuntimeModelOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import { useSessionContinueRuntimeOptions } from "./use-session-continue-runtime-options";

import type { ContinueSessionRequest } from "../adapters/session-derive-api";
import {
  sessionDeriveRouteOptions,
  type SessionDeriveRouteOption,
} from "../lib/session-derive-view";
import type { SessionPayload } from "../types";
import {
  landDerivedSession,
  type SessionDerivePlacement,
  type SessionDerivePlacementHandlers,
  useSessionContinue,
  useSessionDeriveIdempotencyKey,
  useSessionDerivePreview,
} from "./use-session-derive";
import { useSessionDeriveCommittedChild } from "./use-session-derive-committed-child";

/** `route` 0 is the agent's own provider, model and account. */
export const SESSION_CONTINUE_DEFAULT_ROUTE = 0;

const CHILD_DELETED_MESSAGE =
  "The session this request already created was deleted. Nothing new was created.";

/** Superset's rule: the first agent that is not the source's, else the source's own. */
export function defaultContinueAgentName(
  agents: readonly AgentPayload[],
  sourceAgentName: string
): string {
  const source = sourceAgentName.trim();
  return agents.find(agent => agent.name !== source)?.name ?? source;
}

interface RuntimeOverride {
  value: RuntimeSelectorValue;
  speed: RuntimeSpeed;
}

/** Serialize the confirmed runtime without converting omitted speed into agent inheritance. */
function runtimeRequest(
  override: RuntimeOverride,
  inherited: RuntimeSelectorValue
): ContinueSessionRequest["runtime"] {
  const provider = override.value.provider.trim();
  if (provider === "") return undefined;
  const model = override.value.model.trim();
  // Keep inherited values visible, but leave unchanged defaults daemon-owned.
  const acpOverrides = override.value.acp_options?.filter(
    selection =>
      override.value.provider !== inherited.provider ||
      !inherited.acp_options?.some(inherited => runtimeACPSelectionsEqual([selection], [inherited]))
  );
  return {
    provider,
    ...(model ? { model } : {}),
    ...(override.value.reasoning_effort
      ? { reasoning_effort: override.value.reasoning_effort }
      : {}),
    // Always explicit: an omitted speed inherits the agent's default, so a
    // fast-default agent switched to normal would still run fast.
    speed: override.speed,
    ...(acpOverrides?.length ? { acp_options: acpOverrides } : {}),
  };
}

/** Match daemon effort inheritance: only the same provider and model keep the agent's effort. */
function continuedRuntimeEffort(value: RuntimeSelectorValue, inherited: RuntimeSelectorValue) {
  if (value.reasoning_effort) return value.reasoning_effort;
  return value.provider === inherited.provider && value.model === inherited.model
    ? inherited.reasoning_effort
    : "";
}

/** Validate observed runtime combinations; unknown capability data stays daemon-owned. */
function runtimeCompatibilityError(
  model: RuntimeModelOption | undefined,
  effort: RuntimeSelectorValue["reasoning_effort"],
  speed: RuntimeSpeed,
  provider: string
): string | null {
  if (!model) return null;
  if (model.efforts.length > 0 && effort !== "" && !modelSupportsReasoningEffort(model, effort)) {
    return model.efforts.some(level => level !== "none")
      ? "Choose a reasoning effort supported by this model before continuing."
      : "This model does not support the configured reasoning effort. Choose another model or agent.";
  } else if (
    provider === "cursor" &&
    model.configurations?.length &&
    !model.configurations.some(
      configuration =>
        (!effort || !configuration.reasoning_effort || configuration.reasoning_effort === effort) &&
        (configuration.fast == null || configuration.fast === (speed === "fast"))
    )
  ) {
    return "Choose a supported reasoning effort and speed combination before continuing.";
  }
  return null;
}

export interface UseSessionContinueDialogInput {
  source: SessionPayload;
  workspaceId: string;
  open: boolean;
  onClose: () => void;
  placement: SessionDerivePlacementHandlers;
}

/**
 * Continue-dialog state: agent (defaulted), an explicit runtime XOR a declared
 * route, an optional first message, where the child opens, the measured
 * preview, and the submit. Nothing is created until Continue.
 */
export function useSessionContinueDialog({
  source,
  workspaceId,
  open,
  onClose,
  placement: handlers,
}: UseSessionContinueDialogInput) {
  const agentsQuery = useAgents(workspaceId, { enabled: open });
  const agents = agentsQuery.data ?? [];
  const [chosenAgent, setChosenAgent] = useState<string | null>(null);
  const [runtime, setRuntime] = useState<RuntimeOverride | null>(null);
  const [route, setRoute] = useState(SESSION_CONTINUE_DEFAULT_ROUTE);
  const [message, setMessage] = useState("");
  const [placement, setPlacement] = useState<SessionDerivePlacement>("new-window");
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [runtimeValidationRequested, setRuntimeValidationRequested] = useState(false);
  const idempotencyKey = useSessionDeriveIdempotencyKey();
  const preview = useSessionDerivePreview(workspaceId, source.id, { enabled: open });
  const mutation = useSessionContinue();
  const committedChild = useSessionDeriveCommittedChild({
    workspaceId,
    placement,
    handlers,
    onClose,
  });

  const agentName = chosenAgent ?? defaultContinueAgentName(agents, source.agent_name);
  const agent = agents.find(candidate => candidate.name === agentName);
  const routes: SessionDeriveRouteOption[] = sessionDeriveRouteOptions(agent?.fallback_chain);
  const selectedRoute = routes.some(option => option.route === route)
    ? route
    : SESSION_CONTINUE_DEFAULT_ROUTE;
  const runtimeOptions = useSessionContinueRuntimeOptions(workspaceId, open);
  const agentRuntime = resolveAgentRuntimeValue(agent);
  const requestedValue = runtime?.value ?? agentRuntime;
  const selectedModel = runtimeOptions.models.find(
    model => model.provider === requestedValue.provider && model.id === requestedValue.model
  );
  const runtimeValue = normalizeRuntimeSelectorValue(requestedValue, selectedModel);
  const runtimeSpeed =
    runtime?.speed ?? (normalizeRuntimeSpeed(agent?.effective_runtime?.speed) || "normal");
  const effort = continuedRuntimeEffort(requestedValue, agentRuntime);
  const provider = runtimeOptions.providers.find(option => option.id === requestedValue.provider);
  const runtimeValidationError =
    selectedRoute === SESSION_CONTINUE_DEFAULT_ROUTE
      ? runtimeCompatibilityError(
          selectedModel,
          effort,
          runtimeSpeed,
          provider?.runtime_provider ?? requestedValue.provider
        )
      : null;
  const isSubmitting = mutation.isPending;
  const measured = preview.view.state === "ready" || preview.view.state === "truncated";
  const canSubmit = !isSubmitting && measured && agent !== undefined;

  const selectAgent = (next: string) => {
    setChosenAgent(next);
    // Runtime and route are scoped to the agent; a new agent starts from its defaults.
    setRuntime(null);
    setRoute(SESSION_CONTINUE_DEFAULT_ROUTE);
    setSubmitError(null);
    setRuntimeValidationRequested(false);
  };

  const submit = () => {
    if (!canSubmit) return;
    if (runtimeValidationError) {
      setRuntimeValidationRequested(true);
      return;
    }
    const trimmedMessage = message.trim();
    const runtimeBody =
      selectedRoute === SESSION_CONTINUE_DEFAULT_ROUTE
        ? runtimeRequest(
            {
              value: {
                ...runtimeValue,
                // Catalog refreshes cannot erase an explicit choice; ACP validates unknown support.
                reasoning_effort: effort,
              },
              speed: runtimeSpeed,
            },
            agentRuntime
          )
        : null;
    const request: ContinueSessionRequest = {
      agent_name: agentName,
      idempotency_key: idempotencyKey,
      ...(trimmedMessage ? { message: trimmedMessage } : {}),
      ...(selectedRoute !== SESSION_CONTINUE_DEFAULT_ROUTE ? { route: selectedRoute } : {}),
      ...(runtimeBody ? { runtime: runtimeBody } : {}),
    };
    setSubmitError(null);
    setRuntimeValidationRequested(false);
    mutation.mutate(
      { workspaceId, sourceId: source.id, request },
      {
        onSuccess: result => {
          if (result.derived.child_deleted || !result.session) {
            setSubmitError(CHILD_DELETED_MESSAGE);
            return;
          }
          onClose();
          landDerivedSession(result, placement, handlers);
        },
        onError: error => {
          committedChild.noteFailure(error);
          setSubmitError(error.message);
        },
      }
    );
  };

  return {
    agents,
    agentsLoading: agentsQuery.isLoading,
    agentName,
    selectAgent,
    runtimeValue,
    runtimeSpeed,
    runtimeOptions,
    onRuntimeChange: (value: RuntimeSelectorValue, speed?: RuntimeSpeed) =>
      setRuntime({ value, speed: speed ?? runtimeSpeed }),
    onRuntimeSpeedChange: (speed: RuntimeSpeed) => setRuntime({ value: requestedValue, speed }),
    routes,
    route: selectedRoute,
    onRouteChange: setRoute,
    message,
    onMessageChange: setMessage,
    placement,
    onPlacementChange: setPlacement,
    canChoosePlacement: handlers.openInThisWindow !== undefined,
    previewView: preview.view,
    submitError: (runtimeValidationRequested ? runtimeValidationError : null) ?? submitError,
    isSubmitting,
    canSubmit,
    submit,
    committedChild,
  };
}

export type SessionContinueDialogModel = ReturnType<typeof useSessionContinueDialog>;
