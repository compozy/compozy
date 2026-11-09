import { useState } from "react";

import type { RuntimeSpeed } from "@/lib/api-contract";
import {
  type AgentPayload,
  normalizeRuntimeSpeed,
  resolveAgentSessionRuntimeValue,
  useAgents,
} from "@/systems/agent";
import {
  modelSupportsReasoningEffort,
  normalizeRuntimeSelectorValue,
  runtimeACPSelectionsEqual,
  sanitizeRuntimeACPSelections,
  type RuntimeACPOption,
  type RuntimeACPOptionSelection,
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

function isUnchangedInherited(
  selection: RuntimeACPOptionSelection,
  value: RuntimeSelectorValue,
  inherited: RuntimeSelectorValue
): boolean {
  return (
    value.provider === inherited.provider &&
    (inherited.acp_options ?? []).some(option => runtimeACPSelectionsEqual([selection], [option]))
  );
}

function advertisedDefault(option: RuntimeACPOption): RuntimeACPOptionSelection | null {
  if (option.kind === "boolean" && typeof option.current_bool === "boolean") {
    return { id: option.id, bool_value: option.current_bool };
  }
  const value = option.current_value_id?.trim();
  return option.kind === "select" && value && option.values?.some(item => item.value === value)
    ? { id: option.id, value_id: value }
    : null;
}

/**
 * Inherited options the operator returned to the provider default. An omitted option keeps
 * inheriting from the agent, so a reset travels as the advertised default value. Selector
 * normalization only removes selections the descriptors cannot honor, so a still-valid inherited
 * option that is absent was cleared by the operator.
 */
function providerDefaultResets(
  value: RuntimeSelectorValue,
  inherited: RuntimeSelectorValue,
  options: readonly RuntimeACPOption[] | undefined
) {
  const resets: RuntimeACPOptionSelection[] = [];
  const unknown: RuntimeACPOption[] = [];
  if (!options || value.provider !== inherited.provider) return { resets, unknown };
  const optionsByID = new Map(options.map(option => [option.id, option]));
  const selectedIDs = new Set(value.acp_options?.map(selection => selection.id));
  for (const selection of inherited.acp_options ?? []) {
    const option = optionsByID.get(selection.id);
    if (
      !option ||
      selectedIDs.has(selection.id) ||
      !sanitizeRuntimeACPSelections([selection], options)
    ) {
      continue;
    }
    const reset = advertisedDefault(option);
    if (!reset) unknown.push(option);
    else if (!runtimeACPSelectionsEqual([reset], [selection])) resets.push(reset);
  }
  return { resets, unknown };
}

/** Serialize the confirmed runtime without converting omitted speed into agent inheritance. */
function runtimeRequest(
  override: RuntimeOverride,
  inherited: RuntimeSelectorValue,
  resets: readonly RuntimeACPOptionSelection[]
): ContinueSessionRequest["runtime"] {
  const provider = override.value.provider.trim();
  if (provider === "") return undefined;
  const model = override.value.model.trim();
  // Keep inherited values visible, but leave unchanged defaults daemon-owned.
  const acpOverrides = [
    ...(override.value.acp_options ?? []).filter(
      selection => !isUnchangedInherited(selection, override.value, inherited)
    ),
    ...resets,
  ];
  return {
    provider,
    ...(model ? { model } : {}),
    ...(override.value.reasoning_effort
      ? { reasoning_effort: override.value.reasoning_effort }
      : {}),
    // Always explicit: an omitted speed inherits the agent's default, so a
    // fast-default agent switched to normal would still run fast.
    speed: override.speed,
    ...(acpOverrides.length > 0 ? { acp_options: acpOverrides } : {}),
  };
}

/** Match daemon effort inheritance: only the same provider and model keep the agent's effort. */
function continuedRuntimeEffort(value: RuntimeSelectorValue, inherited: RuntimeSelectorValue) {
  if (value.reasoning_effort) return value.reasoning_effort;
  return value.provider === inherited.provider && value.model === inherited.model
    ? inherited.reasoning_effort
    : "";
}

/** Thinking as the daemon resolves it: the selection, then the advertised value, else off. */
function cursorThinking(
  model: RuntimeModelOption,
  selections: readonly RuntimeACPOptionSelection[] | undefined
): boolean {
  const selected = selections?.find(selection => selection.id === "thinking")?.bool_value;
  return (
    selected ?? model.acp_options?.find(option => option.id === "thinking")?.current_bool ?? false
  );
}

function cursorConfigurationError(
  model: RuntimeModelOption,
  effort: RuntimeSelectorValue["reasoning_effort"],
  speed: RuntimeSpeed,
  selections: readonly RuntimeACPOptionSelection[] | undefined
): string | null {
  // The daemon matches an omitted effort against the model default; the wire keeps it omitted.
  const resolvedEffort = effort || model.default_effort || "";
  const pairs = (model.configurations ?? []).filter(
    configuration =>
      (!resolvedEffort ||
        !configuration.reasoning_effort ||
        configuration.reasoning_effort === resolvedEffort) &&
      (configuration.fast == null || configuration.fast === (speed === "fast"))
  );
  if (pairs.length === 0) {
    return "Choose a supported reasoning effort and speed combination before continuing.";
  }
  const thinking = cursorThinking(model, selections);
  return pairs.some(pair => pair.thinking == null || pair.thinking === thinking)
    ? null
    : "Choose a thinking setting supported by this reasoning effort and speed before continuing.";
}

/** Validate observed runtime combinations; unknown capability data stays daemon-owned. */
function runtimeCompatibilityError(
  model: RuntimeModelOption | undefined,
  effort: RuntimeSelectorValue["reasoning_effort"],
  speed: RuntimeSpeed,
  provider: string,
  selections: readonly RuntimeACPOptionSelection[] | undefined
): string | null {
  if (!model) return null;
  if (model.efforts.length > 0 && effort !== "" && !modelSupportsReasoningEffort(model, effort)) {
    return model.efforts.some(level => level !== "none")
      ? "Choose a reasoning effort supported by this model before continuing."
      : "This model does not support the configured reasoning effort. Choose another model or agent.";
  }
  return provider === "cursor" && model.configurations?.length
    ? cursorConfigurationError(model, effort, speed, selections)
    : null;
}

function optionLabel(id: string, options: readonly RuntimeACPOption[]): string {
  return options.find(option => option.id === id)?.label?.trim() || id;
}

/** Refuse advanced choices the current descriptors rule out instead of dropping them silently. */
function advancedOptionsError(
  requested: RuntimeSelectorValue,
  inherited: RuntimeSelectorValue,
  options: readonly RuntimeACPOption[] | undefined,
  unknownDefaults: readonly RuntimeACPOption[]
): string | null {
  if (!options) return null;
  const stale = requested.acp_options?.find(
    selection =>
      !isUnchangedInherited(selection, requested, inherited) &&
      !sanitizeRuntimeACPSelections([selection], options)
  );
  if (stale) {
    return `This model no longer offers the "${optionLabel(stale.id, options)}" setting you chose. Reselect the model to continue without it.`;
  }
  const unknown = unknownDefaults[0];
  return unknown
    ? `This provider doesn't advertise a default for "${optionLabel(unknown.id, options)}". Choose a value before continuing.`
    : null;
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
  const agentRuntime = resolveAgentSessionRuntimeValue(agent);
  const requestedValue = runtime?.value ?? agentRuntime;
  const selectedModel = runtimeOptions.models.find(
    model => model.provider === requestedValue.provider && model.id === requestedValue.model
  );
  const runtimeValue = normalizeRuntimeSelectorValue(requestedValue, selectedModel);
  const runtimeSpeed =
    runtime?.speed ?? (normalizeRuntimeSpeed(agent?.effective_runtime?.speed) || "normal");
  const effort = continuedRuntimeEffort(requestedValue, agentRuntime);
  const provider = runtimeOptions.providers.find(option => option.id === requestedValue.provider);
  const defaultResets = providerDefaultResets(
    requestedValue,
    agentRuntime,
    selectedModel?.acp_options
  );
  const runtimeValidationError =
    selectedRoute === SESSION_CONTINUE_DEFAULT_ROUTE
      ? (runtimeCompatibilityError(
          selectedModel,
          effort,
          runtimeSpeed,
          provider?.runtime_provider ?? requestedValue.provider,
          runtimeValue.acp_options
        ) ??
        advancedOptionsError(
          requestedValue,
          agentRuntime,
          selectedModel?.acp_options,
          defaultResets.unknown
        ))
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
            agentRuntime,
            defaultResets.resets
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
