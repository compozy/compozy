import { createContext, use } from "react";

import type { SessionComposerController } from "./use-session-composer-controller";

export const SessionComposerStateContext = createContext<SessionComposerController["state"] | null>(
  null
);
export const SessionComposerActionsContext = createContext<
  SessionComposerController["actions"] | null
>(null);
export const SessionComposerMetaContext = createContext<SessionComposerController["meta"] | null>(
  null
);

export function useSessionComposerStateContext(): SessionComposerController["state"] {
  const state = use(SessionComposerStateContext);
  if (state === null) throw new Error("Session composer state is unavailable");
  return state;
}

export function useSessionComposerActionsContext(): SessionComposerController["actions"] {
  const actions = use(SessionComposerActionsContext);
  if (actions === null) throw new Error("Session composer actions are unavailable");
  return actions;
}

export function useSessionComposerMetaContext(): SessionComposerController["meta"] {
  const meta = use(SessionComposerMetaContext);
  if (meta === null) throw new Error("Session composer metadata is unavailable");
  return meta;
}
