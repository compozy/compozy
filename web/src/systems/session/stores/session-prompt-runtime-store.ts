import { createStoreLogic } from "@xstate/store";

import { type RuntimeSpeed } from "@/lib/api-contract";
import type { RuntimeSelectorValue } from "@/systems/runtime";

import type { SessionRuntimeEffective } from "../types";

interface SessionPromptRuntimeInput {
  canPrompt: boolean;
  effectiveRuntime: SessionRuntimeEffective | undefined;
  workspaceId: string;
  agentName: string;
}

export interface SessionPromptRuntimeInputSource {
  agentName: string;
  canPrompt: boolean;
  effectiveRuntime: SessionRuntimeEffective | null | undefined;
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
  runtimeSelected: { value: RuntimeSelectorValue };
  speedSelected: { speed: RuntimeSpeed };
};

export const sessionPromptRuntimeStoreLogic = createStoreLogic<
  SessionPromptRuntimeStoreContext,
  SessionPromptRuntimeStoreEvents,
  {},
  SessionPromptRuntimeInput
>({
  context: input => ({
    defaultSpeed: input.effectiveRuntime?.speed ?? "normal",
    defaultValue: {
      model: input.effectiveRuntime?.model?.trim() ?? "",
      provider: input.effectiveRuntime?.provider?.trim() ?? "",
      reasoning_effort: input.effectiveRuntime?.reasoning_effort ?? "",
    },
    input,
    selectedSpeed: null,
    selectedValue: null,
  }),
  on: {
    defaultRuntimeResolved: (context, event) =>
      context.defaultSpeed === event.speed && sameValue(context.defaultValue, event.value)
        ? undefined
        : {
            ...context,
            defaultSpeed: event.speed,
            defaultValue: event.value,
          },
    inputUpdated: (context, event) =>
      sameInput(context.input, event)
        ? undefined
        : {
            ...context,
            input: event,
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
    workspaceId,
  };
}

function sameInput(left: SessionPromptRuntimeInput, right: SessionPromptRuntimeInput): boolean {
  return (
    left.agentName === right.agentName &&
    left.canPrompt === right.canPrompt &&
    left.workspaceId === right.workspaceId &&
    left.effectiveRuntime?.provider === right.effectiveRuntime?.provider &&
    left.effectiveRuntime?.model === right.effectiveRuntime?.model &&
    left.effectiveRuntime?.reasoning_effort === right.effectiveRuntime?.reasoning_effort &&
    left.effectiveRuntime?.speed === right.effectiveRuntime?.speed
  );
}

function sameValue(left: RuntimeSelectorValue | null, right: RuntimeSelectorValue): boolean {
  return (
    left?.provider === right.provider &&
    left?.model === right.model &&
    left?.reasoning_effort === right.reasoning_effort
  );
}
