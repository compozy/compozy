import { primarySessionFixture, sessionFixtures } from "@/systems/session/mocks/fixtures";
import type { SessionPayload } from "@/systems/session";

import type { OsAppId, OsWindow } from "../../lib/os-types";

/** A projected window for tab and split specimens; sessions carry their session ID as the key. */
export function storyOsWindow(
  id: string,
  app: OsAppId,
  instanceKey: string | null = null
): OsWindow {
  return {
    id,
    app,
    instanceKey,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: "desktop:story",
    tabId: `tab:${id}`,
    nodeId: `leaf:${id}`,
    pinned: false,
    inSplit: false,
    tabActive: false,
    shown: true,
  };
}

const waitingSource =
  sessionFixtures.find(session => session.pending_interactions.length > 0) ?? primarySessionFixture;

const STORY_SESSIONS: Record<string, SessionPayload> = {
  "session-auth": {
    ...primarySessionFixture,
    id: "session-auth",
    name: "Auth refactor",
    badge: "idle",
    pending_interactions: [],
  },
  "session-tabs": {
    ...primarySessionFixture,
    id: "session-tabs",
    name: "Window tabs",
    badge: "running",
    pending_interactions: [],
  },
  "session-review": {
    ...waitingSource,
    id: "session-review",
    name: "Review release notes",
    badge: "needs-attention",
  },
};

export const STORY_WINDOWS = {
  auth: storyOsWindow("w-auth", "session", "session-auth"),
  tabs: storyOsWindow("w-tabs", "session", "session-tabs"),
  review: storyOsWindow("w-review", "session", "session-review"),
  dash: storyOsWindow("w-dash", "dashboard"),
  agents: storyOsWindow("w-agents", "agents"),
  tasks: storyOsWindow("w-tasks", "tasks"),
  terminal: storyOsWindow("w-terminal", "terminal"),
  files: storyOsWindow("w-files", "files"),
  docs: storyOsWindow("w-docs", "files"),
} as const;

export function storySessionOf(win: OsWindow): SessionPayload | undefined {
  return win.app === "session" ? STORY_SESSIONS[win.instanceKey ?? ""] : undefined;
}
