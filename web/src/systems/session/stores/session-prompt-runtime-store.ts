import { createStoreLogic } from "@xstate/store";

import { type RuntimeSpeed } from "@/lib/api-contract";

import type { SessionRuntimeEffective, SessionRuntimeSelection } from "../types";
import type { RuntimeSelectorValue } from "@/systems/runtime";

interface SessionPromptRuntimeInput {
  canPrompt: boolean;
  effectiveRuntime: SessionRuntimeEffective | undefined;
  selectedRuntime: SessionRuntimeSelection | undefined;
  selectionRevision: number;
  sessionId: string;
  workspaceId: string;
  agentName: string;
}

export interface SessionPromptRuntimeInputSource {
  agentName: string;
  canPrompt: boolean;
  effectiveRuntime: SessionRuntimeEffective | null | undefined;
  selectedRuntime: SessionRuntimeSelection | null | undefined;
  selectionRevision: number;
  sessionId: string;
  workspaceId: string | undefined;
}

interface SessionPromptRuntimeStoreContext {
  defaultSpeed: RuntimeSpeed;
  defaultValue: RuntimeSelectorValue;
  input: SessionPromptRuntimeInput;
  selectedSpeed: RuntimeSpeed | null;
  selectedValue: RuntimeSelectorValue | null;
}

type SessionPromptRuntimeStoreEvents = {
  defaultRuntimeResolved: { speed: RuntimeSpeed; value: RuntimeSelectorValue };
  inputUpdated: SessionPromptRuntimeInput;
  runtimePersistenceFailed: SessionRuntimeIdentity;
  runtimePersisted: SessionRuntimeIdentity & {
    revision: number;
    runtime: SessionRuntimeSelection | undefined;
  };
  runtimeSelected: { value: RuntimeSelectorValue };
  speedSelected: { speed: RuntimeSpeed };
};

interface SessionRuntimeIdentity {
  sessionId: string;
  workspaceId: string;
}

export const sessionPromptRuntimeStoreLogic = createStoreLogic<
  SessionPromptRuntimeStoreContext,
  SessionPromptRuntimeStoreEvents,
  {},
  SessionPromptRuntimeInput
>({
  context: input => selectionContext(input),
  on: {
    defaultRuntimeResolved: (context, event) =>
      context.defaultSpeed === event.speed && sameValue(context.defaultValue, event.value)
        ? undefined
        : {
            ...context,
            defaultSpeed: event.speed,
            defaultValue: event.value,
          },
    inputUpdated: (context, event) => {
      const nextInput = preserveNewerRuntimeSelection(context.input, event);
      if (sameInput(context.input, nextInput)) return undefined;
      const selectionChanged =
        context.input.selectionRevision !== nextInput.selectionRevision ||
        !sameRuntimeSelection(context.input.selectedRuntime, nextInput.selectedRuntime);
      return {
        ...context,
        input: nextInput,
        ...(selectionChanged ? selectedContext(nextInput.selectedRuntime) : {}),
      };
    },
    runtimePersistenceFailed: (context, event) =>
      sameSessionIdentity(context.input, event)
        ? {
            ...context,
            ...selectedContext(context.input.selectedRuntime),
          }
        : undefined,
    runtimePersisted: (context, event) => {
      if (
        !sameSessionIdentity(context.input, event) ||
        event.revision < context.input.selectionRevision
      ) {
        return undefined;
      }
      return {
        ...context,
        input: {
          ...context.input,
          selectedRuntime: event.runtime,
          selectionRevision: event.revision,
        },
        ...selectedContext(event.runtime),
      };
    },
    runtimeSelected: (context, event) =>
      sameValue(context.selectedValue, event.value)
        ? undefined
        : {
            ...context,
            selectedValue: event.value,
          },
    speedSelected: (context, event) =>
      context.selectedSpeed === event.speed
        ? undefined
        : {
            ...context,
            selectedSpeed: event.speed,
          },
  },
});

export type SessionPromptRuntimeStore = ReturnType<
  typeof sessionPromptRuntimeStoreLogic.createStore
>;

export function sessionPromptRuntimeInput(
  source: SessionPromptRuntimeInputSource
): SessionPromptRuntimeInput {
  const workspaceId = source.workspaceId?.trim() ?? "";
  return {
    agentName: source.agentName,
    canPrompt: source.canPrompt && workspaceId.length > 0,
    effectiveRuntime: source.effectiveRuntime ?? undefined,
    selectedRuntime: source.selectedRuntime ?? undefined,
    selectionRevision: source.selectionRevision,
    sessionId: source.sessionId,
    workspaceId,
  };
}

function selectionContext(input: SessionPromptRuntimeInput): SessionPromptRuntimeStoreContext {
  return {
    defaultSpeed: input.effectiveRuntime?.speed ?? "normal",
    defaultValue: runtimeValue(input.effectiveRuntime),
    input,
    ...selectedContext(input.selectedRuntime),
  };
}

function selectedContext(runtime: SessionRuntimeSelection | undefined) {
  return {
    selectedSpeed: runtime ? (runtime.speed ?? "normal") : null,
    selectedValue: runtime ? runtimeValue(runtime) : null,
  };
}

function runtimeValue(
  runtime: SessionRuntimeEffective | SessionRuntimeSelection | undefined
): RuntimeSelectorValue {
  return {
    model: runtime?.model?.trim() ?? "",
    provider: runtime?.provider?.trim() ?? "",
    reasoning_effort: runtime?.reasoning_effort ?? "",
  };
}

function sameInput(left: SessionPromptRuntimeInput, right: SessionPromptRuntimeInput): boolean {
  return (
    left.agentName === right.agentName &&
    left.canPrompt === right.canPrompt &&
    left.sessionId === right.sessionId &&
    left.workspaceId === right.workspaceId &&
    left.effectiveRuntime?.provider === right.effectiveRuntime?.provider &&
    left.effectiveRuntime?.model === right.effectiveRuntime?.model &&
    left.effectiveRuntime?.reasoning_effort === right.effectiveRuntime?.reasoning_effort &&
    left.effectiveRuntime?.speed === right.effectiveRuntime?.speed &&
    left.selectionRevision === right.selectionRevision &&
    sameRuntimeSelection(left.selectedRuntime, right.selectedRuntime)
  );
}

function preserveNewerRuntimeSelection(
  current: SessionPromptRuntimeInput,
  incoming: SessionPromptRuntimeInput
): SessionPromptRuntimeInput {
  if (
    !sameSessionIdentity(current, incoming) ||
    incoming.selectionRevision >= current.selectionRevision
  ) {
    return incoming;
  }
  return {
    ...incoming,
    selectedRuntime: current.selectedRuntime,
    selectionRevision: current.selectionRevision,
  };
}

function sameSessionIdentity(left: SessionRuntimeIdentity, right: SessionRuntimeIdentity): boolean {
  return left.sessionId === right.sessionId && left.workspaceId === right.workspaceId;
}

function sameRuntimeSelection(
  left: SessionRuntimeSelection | undefined,
  right: SessionRuntimeSelection | undefined
): boolean {
  return (
    left?.provider === right?.provider &&
    left?.model === right?.model &&
    left?.reasoning_effort === right?.reasoning_effort &&
    left?.speed === right?.speed
  );
}

function sameValue(left: RuntimeSelectorValue | null, right: RuntimeSelectorValue): boolean {
  return (
    left?.provider === right.provider &&
    left?.model === right.model &&
    left?.reasoning_effort === right.reasoning_effort
  );
}
