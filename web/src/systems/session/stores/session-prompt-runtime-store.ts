import { createStoreLogic } from "@xstate/store";

import { type RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";

import type { SessionRuntimeEffective, SessionRuntimeSelection } from "../types";

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
  runtimePersistenceFailed: {};
  runtimePersisted: { revision: number; runtime: SessionRuntimeSelection | undefined };
  runtimeSelected: { value: RuntimeSelectorValue };
  speedSelected: { speed: RuntimeSpeed };
};

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
      if (sameInput(context.input, event)) return undefined;
      const selectionChanged =
        context.input.selectionRevision !== event.selectionRevision ||
        !sameRuntimeSelection(context.input.selectedRuntime, event.selectedRuntime);
      return {
        ...context,
        input: event,
        ...(selectionChanged ? selectedContext(event.selectedRuntime) : {}),
      };
    },
    runtimePersistenceFailed: context => ({
      ...context,
      ...selectedContext(context.input.selectedRuntime),
    }),
    runtimePersisted: (context, event) => ({
      ...context,
      input: {
        ...context.input,
        selectedRuntime: event.runtime,
        selectionRevision: event.revision,
      },
      ...selectedContext(event.runtime),
    }),
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
