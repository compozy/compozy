import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";

import { sessionInspectorStore } from "./use-session-inspector-state";

export type SessionInspectorFocusSection = "subagents";

interface SessionInspectorFocusContext {
  /** One pending request: the session whose inspector should land on a section. */
  request: { sessionId: string; section: SessionInspectorFocusSection } | null;
}

const sessionInspectorFocusLogic = createStoreLogic({
  context: (): SessionInspectorFocusContext => ({ request: null }),
  on: {
    sectionRequested: (
      context,
      event: { sessionId: string; section: SessionInspectorFocusSection }
    ) => ({ ...context, request: { sessionId: event.sessionId, section: event.section } }),
    sectionLanded: (context, event: { sessionId: string }) =>
      context.request?.sessionId === event.sessionId ? { ...context, request: null } : context,
  },
});

/** Transient, never persisted: a landing request is consumed once by the section it names. */
export const sessionInspectorFocusStore = sessionInspectorFocusLogic.createStore();

/**
 * Opens a session's inspector on one section (the sidebar chip opens Subagents,
 * S9). The inspector preference is per session, so the request survives the
 * window retargeting to that session before the inspector mounts.
 */
export function requestSessionInspectorSection(
  sessionId: string,
  section: SessionInspectorFocusSection
): void {
  if (!sessionId) return;
  sessionInspectorFocusStore.trigger.sectionRequested({ sessionId, section });
  sessionInspectorStore.trigger.inspectorVisibilityChanged({ sessionId, open: true });
}

/** Whether this session's inspector should land on `section`; `land` consumes the request. */
export function useSessionInspectorFocus(
  sessionId: string,
  section: SessionInspectorFocusSection
): { requested: boolean; land: () => void } {
  const requested = useSelector(
    sessionInspectorFocusStore,
    snapshot =>
      snapshot.context.request?.sessionId === sessionId &&
      snapshot.context.request.section === section
  );
  return {
    requested,
    land: () => sessionInspectorFocusStore.trigger.sectionLanded({ sessionId }),
  };
}
