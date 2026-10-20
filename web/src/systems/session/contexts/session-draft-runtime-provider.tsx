import { useEffect, type ReactNode } from "react";
import { z } from "zod";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { notifyUser } from "@/lib/user-feedback";
import { useProfileReadScope } from "@/systems/profiles";
import { normalizeRuntimeACPSelections } from "@/systems/runtime";
import {
  sessionPromptRuntimeInput,
  sessionPromptRuntimeStoreLogic,
} from "../stores/session-prompt-runtime-store";
import {
  SessionPromptRuntimeContext,
  SessionPromptRuntimePersistenceContext,
  type SessionPromptRuntimeSnapshot,
} from "./session-prompt-runtime-context-value";

const draftRuntimeSchema = z.object({
  provider: z.string().min(1),
  model: z.string().optional(),
  reasoning_effort: z.string().optional(),
  speed: z.enum(["normal", "fast"]).optional(),
  acp_options: z
    .array(
      z.object({
        id: z.string(),
        value_id: z.string().optional(),
        bool_value: z.boolean().optional(),
      })
    )
    .optional(),
});

/** Reads optional draft intent, discarding invalid stored shapes before they reach the selector. */
function readDraftRuntime(key: string): SessionPromptRuntimeSnapshot | undefined {
  try {
    const value = localStorage.getItem(key);
    if (!value) return undefined;
    const parsed = draftRuntimeSchema.safeParse(JSON.parse(value));
    if (!parsed.success) return undefined;
    const { acp_options, ...runtime } = parsed.data;
    const options = normalizeRuntimeACPSelections(acp_options);
    return { ...runtime, ...(options ? { acp_options: options } : {}) };
  } catch (error) {
    console.error("Couldn't read the draft runtime", error);
    return undefined;
  }
}

/** The session selector with local intent persistence before session creation. */
export function SessionDraftRuntimeProvider({
  draftId,
  workspaceId,
  agentName,
  canPrompt,
  children,
}: {
  draftId: string;
  workspaceId: string;
  agentName: string;
  canPrompt: boolean;
  children: ReactNode;
}) {
  const { destination } = useProfileReadScope();
  const key = `compozy:prompt-runtime:${JSON.stringify([destination, draftId, agentName])}`;
  const { store } = useStoreBinding(key, () =>
    sessionPromptRuntimeStoreLogic.createStore(
      sessionPromptRuntimeInput({
        sessionId: draftId,
        workspaceId,
        agentName,
        canPrompt,
        effectiveRuntime: undefined,
        selectedRuntime: undefined,
        selectionRevision: 0,
      })
    )
  );
  useEffect(() => {
    store.trigger.inputUpdated({
      ...store.getSnapshot().context.input,
      selectedRuntime: readDraftRuntime(key),
    });
  }, [key, store]);

  useEffect(() => {
    store.trigger.inputUpdated({
      ...store.getSnapshot().context.input,
      canPrompt,
    });
  }, [canPrompt, store]);

  const persist = (runtime: SessionPromptRuntimeSnapshot) => {
    try {
      localStorage.setItem(key, JSON.stringify(runtime));
    } catch {
      notifyUser({ message: "Couldn't save the runtime choice for reload.", tone: "error" });
    }
  };

  return (
    <SessionPromptRuntimeContext value={store}>
      <SessionPromptRuntimePersistenceContext value={persist}>
        {children}
      </SessionPromptRuntimePersistenceContext>
    </SessionPromptRuntimeContext>
  );
}
