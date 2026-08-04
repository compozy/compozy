import { useEffect, type ReactNode } from "react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import type { SessionPayload, SessionRuntimeEffective, SessionRuntimeSelection } from "../types";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../stores/session-prompt-runtime-store";
import { SessionPromptRuntimeContext } from "./session-prompt-runtime-context-value";

export interface SessionPromptRuntimeProviderProps {
  session: SessionPayload;
  canPrompt: boolean;
  children: ReactNode;
}

function runtimeInputFromValues(
  agentName: string,
  canPrompt: boolean,
  workspaceId: string | undefined,
  sessionId: string,
  provider: SessionRuntimeEffective["provider"] | undefined,
  model: SessionRuntimeEffective["model"] | undefined,
  reasoningEffort: SessionRuntimeEffective["reasoning_effort"] | undefined,
  speed: SessionRuntimeEffective["speed"] | undefined,
  selectedProvider: SessionRuntimeSelection["provider"] | undefined,
  selectedModel: SessionRuntimeSelection["model"] | undefined,
  selectedReasoningEffort: SessionRuntimeSelection["reasoning_effort"] | undefined,
  selectedSpeed: SessionRuntimeSelection["speed"] | undefined,
  selectionRevision: number
) {
  return sessionPromptRuntimeInput({
    agentName,
    canPrompt,
    effectiveRuntime:
      provider === undefined
        ? undefined
        : {
            provider,
            ...(model === undefined ? {} : { model }),
            ...(reasoningEffort === undefined ? {} : { reasoning_effort: reasoningEffort }),
            ...(speed === undefined ? {} : { speed }),
          },
    selectedRuntime:
      selectedProvider === undefined
        ? undefined
        : {
            provider: selectedProvider,
            ...(selectedModel === undefined ? {} : { model: selectedModel }),
            ...(selectedReasoningEffort === undefined
              ? {}
              : { reasoning_effort: selectedReasoningEffort }),
            ...(selectedSpeed === undefined ? {} : { speed: selectedSpeed }),
          },
    selectionRevision,
    sessionId,
    workspaceId,
  });
}

export function SessionPromptRuntimeProvider({
  session,
  canPrompt,
  children,
}: SessionPromptRuntimeProviderProps) {
  const effectiveProvider = session.runtime?.effective?.provider;
  const effectiveModel = session.runtime?.effective?.model;
  const effectiveReasoningEffort = session.runtime?.effective?.reasoning_effort;
  const effectiveSpeed = session.runtime?.effective?.speed;
  const selectedProvider = session.runtime?.selected?.provider;
  const selectedModel = session.runtime?.selected?.model;
  const selectedReasoningEffort = session.runtime?.selected?.reasoning_effort;
  const selectedSpeed = session.runtime?.selected?.speed;
  const selectionRevision = session.runtime.selection_revision;
  const { store } = useStoreBinding(session.id, () =>
    sessionPromptRuntimeStoreLogic.createStore(
      runtimeInputFromValues(
        session.agent_name,
        canPrompt,
        session.workspace_id,
        session.id,
        effectiveProvider,
        effectiveModel,
        effectiveReasoningEffort,
        effectiveSpeed,
        selectedProvider,
        selectedModel,
        selectedReasoningEffort,
        selectedSpeed,
        selectionRevision
      )
    )
  );

  useEffect(() => {
    store.trigger.inputUpdated(
      runtimeInputFromValues(
        session.agent_name,
        canPrompt,
        session.workspace_id,
        session.id,
        effectiveProvider,
        effectiveModel,
        effectiveReasoningEffort,
        effectiveSpeed,
        selectedProvider,
        selectedModel,
        selectedReasoningEffort,
        selectedSpeed,
        selectionRevision
      )
    );
  }, [
    canPrompt,
    effectiveModel,
    effectiveProvider,
    effectiveReasoningEffort,
    effectiveSpeed,
    selectedModel,
    selectedProvider,
    selectedReasoningEffort,
    selectedSpeed,
    selectionRevision,
    session.agent_name,
    session.id,
    session.workspace_id,
    store,
  ]);

  return <SessionPromptRuntimeContext value={store}>{children}</SessionPromptRuntimeContext>;
}
