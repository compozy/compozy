import type { SessionSideItem } from "../adapters/session-side-api";
import type { SessionPayload } from "../types";
import { sessionFixtures } from "./fixtures";

/**
 * Side chat fixtures, shared by the side panel prototype and its Vitest cases (`_tests.md` S3).
 */

/** A root user session that holds side chats; its badge drives the main status chip. */
export const sideChatMainSessionFixture: SessionPayload = {
  ...sessionFixtures[1]!,
  id: "sess-7f2a91c04b1e",
  name: "Refactor the agent session config",
  type: "user",
  lineage: undefined,
  archived_at: null,
  state: "active",
  badge: "running",
};

/** The main session with `badge` (and `state`) as given, for the status chip states. */
export function sideChatMainSession(
  badge: SessionPayload["badge"],
  state: SessionPayload["state"] = "active"
): SessionPayload {
  return { ...sideChatMainSessionFixture, badge, state };
}

function sideItem(overrides: Partial<SessionSideItem> & Pick<SessionSideItem, "id" | "name">) {
  return {
    state: "active",
    prompting: false,
    pending_permission_count: 0,
    pending_clarify_count: 0,
    archived_at: null,
    created_at: "2026-11-19T15:02:11Z",
    updated_at: "2026-11-19T15:04:40Z",
    ...overrides,
  } satisfies SessionSideItem;
}

/** Answering, newest first. */
export const sideChatAnsweringFixture = sideItem({
  id: "sess-41d0c2e9aa73",
  name: "Why the refactor touches config file",
  prompting: true,
});

export const sideChatIdleFixture = sideItem({
  id: "sess-9b3e7c11d020",
  name: "Permission enum validation",
  updated_at: "2026-11-19T14:10:02Z",
});

/** Waits on a permission: the head toggle shows the attention dot (US-004.EC-1). */
export const sideChatPermissionFixture = sideItem({
  id: "sess-2c8a4f61be90",
  name: "Can we drop the legacy loader?",
  pending_permission_count: 1,
  updated_at: "2026-11-19T15:05:12Z",
});

/** Waits on a clarifying answer. */
export const sideChatClarifyFixture = sideItem({
  id: "sess-5e17b09d33c4",
  name: "Which migration runs first?",
  pending_clarify_count: 1,
  updated_at: "2026-11-19T15:05:30Z",
});

/** Released while idle: sending resumes it (US-007.EC-1). */
export const sideChatStoppedFixture = sideItem({
  id: "sess-8d61e2a7f415",
  name: "What does compactReplay do?",
  state: "stopped",
  updated_at: "2026-11-19T13:44:51Z",
});

/** Closed earlier: listed only with `archived=true` (the delete confirmation line). */
export const sideChatArchivedFixture = sideItem({
  id: "sess-0a9c3d5b7e21",
  name: "Earlier question",
  state: "stopped",
  archived_at: "2026-11-19T12:00:00Z",
  updated_at: "2026-11-19T12:00:00Z",
});

export const sideChatListFixture: SessionSideItem[] = [
  sideChatAnsweringFixture,
  sideChatIdleFixture,
  sideChatStoppedFixture,
];

/** Twenty-four side chats: the switcher scrolls and lists every one (US-006.EC-1). */
export const sideChatLongListFixture: SessionSideItem[] = Array.from({ length: 24 }, (_, index) =>
  sideItem({
    id: `sess-long-${String(index).padStart(2, "0")}`,
    name: `Side question ${index + 1}`,
    updated_at: new Date(Date.UTC(2026, 10, 19, 15, 0, 0) - index * 60_000).toISOString(),
  })
);

/** The question a side chat opened with, as the `opening` and `failed` states show it. */
export const sideChatQuestionFixture = "why does the refactor touch config_agent_session.go?";
