import { useState } from "react";

import type { RuntimeSpeed } from "@/lib/api-contract";
import {
  type AgentPayload,
  normalizeRuntimeSpeed,
  resolveAgentRuntimeValue,
  useAgents,
} from "@/systems/agent";
import type { RuntimeSelectorValue } from "@/systems/runtime";

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

function runtimeRequest(override: RuntimeOverride): ContinueSessionRequest["runtime"] {
  const provider = override.value.provider.trim();
  if (provider === "") return undefined;
  const model = override.value.model.trim();
  return {
    provider,
    ...(model ? { model } : {}),
    ...(override.value.reasoning_effort
      ? { reasoning_effort: override.value.reasoning_effort }
      : {}),
    ...(override.speed === "fast" ? { speed: override.speed } : {}),
    ...(override.value.acp_options ? { acp_options: override.value.acp_options } : {}),
  };
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
  const idempotencyKey = useSessionDeriveIdempotencyKey();
  const preview = useSessionDerivePreview(workspaceId, source.id, { enabled: open });
  const mutation = useSessionContinue();

  const agentName = chosenAgent ?? defaultContinueAgentName(agents, source.agent_name);
  const agent = agents.find(candidate => candidate.name === agentName);
  const routes: SessionDeriveRouteOption[] = sessionDeriveRouteOptions(agent?.fallback_chain);
  const selectedRoute = routes.some(option => option.route === route)
    ? route
    : SESSION_CONTINUE_DEFAULT_ROUTE;
  const runtimeValue = runtime?.value ?? resolveAgentRuntimeValue(agent);
  const runtimeSpeed =
    runtime?.speed ?? (normalizeRuntimeSpeed(agent?.effective_runtime?.speed) || "normal");
  const isSubmitting = mutation.isPending;
  const measured = preview.view.state === "ready" || preview.view.state === "truncated";
  const canSubmit = !isSubmitting && measured && agent !== undefined;

  const selectAgent = (next: string) => {
    setChosenAgent(next);
    // Runtime and route are scoped to the agent; a new agent starts from its defaults.
    setRuntime(null);
    setRoute(SESSION_CONTINUE_DEFAULT_ROUTE);
    setSubmitError(null);
  };

  const submit = () => {
    if (!canSubmit) return;
    const trimmedMessage = message.trim();
    const runtimeBody =
      selectedRoute === SESSION_CONTINUE_DEFAULT_ROUTE && runtime ? runtimeRequest(runtime) : null;
    const request: ContinueSessionRequest = {
      agent_name: agentName,
      idempotency_key: idempotencyKey,
      ...(trimmedMessage ? { message: trimmedMessage } : {}),
      ...(selectedRoute !== SESSION_CONTINUE_DEFAULT_ROUTE ? { route: selectedRoute } : {}),
      ...(runtimeBody ? { runtime: runtimeBody } : {}),
    };
    setSubmitError(null);
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
    onRuntimeChange: (value: RuntimeSelectorValue, speed?: RuntimeSpeed) =>
      setRuntime({ value, speed: speed ?? runtimeSpeed }),
    onRuntimeSpeedChange: (speed: RuntimeSpeed) => setRuntime({ value: runtimeValue, speed }),
    routes,
    route: selectedRoute,
    onRouteChange: setRoute,
    message,
    onMessageChange: setMessage,
    placement,
    onPlacementChange: setPlacement,
    canChoosePlacement: handlers.openInThisWindow !== undefined,
    previewView: preview.view,
    submitError,
    isSubmitting,
    canSubmit,
    submit,
  };
}

export type SessionContinueDialogModel = ReturnType<typeof useSessionContinueDialog>;
