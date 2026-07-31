import { useEffect, type ReactNode } from "react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import type { SessionPayload, SessionRuntimeEffective } from "../types";
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
  provider: SessionRuntimeEffective["provider"] | undefined,
  model: SessionRuntimeEffective["model"] | undefined,
  reasoningEffort: SessionRuntimeEffective["reasoning_effort"] | undefined,
  speed: SessionRuntimeEffective["speed"] | undefined
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
  const { store } = useStoreBinding(session.id, () =>
    sessionPromptRuntimeStoreLogic.createStore(
      runtimeInputFromValues(
        session.agent_name,
        canPrompt,
        session.workspace_id,
        effectiveProvider,
        effectiveModel,
        effectiveReasoningEffort,
        effectiveSpeed
      )
    )
  );

  useEffect(() => {
    store.trigger.inputUpdated(
      runtimeInputFromValues(
        session.agent_name,
        canPrompt,
        session.workspace_id,
        effectiveProvider,
        effectiveModel,
        effectiveReasoningEffort,
        effectiveSpeed
      )
    );
  }, [
    canPrompt,
    effectiveModel,
    effectiveProvider,
    effectiveReasoningEffort,
    effectiveSpeed,
    session.agent_name,
    session.workspace_id,
    store,
  ]);

  return <SessionPromptRuntimeContext value={store}>{children}</SessionPromptRuntimeContext>;
}
