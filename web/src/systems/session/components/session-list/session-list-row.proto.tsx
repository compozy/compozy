import { PageContent } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "#gallery/contract";

import {
  SESSIONS_BOARD_ARCHIVED,
  SESSIONS_BOARD_SESSIONS,
} from "../../mocks/sessions-board-fixtures";
import { boardRuntimeFixture } from "../../mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "../../mocks/session-row-fixtures";
import { SessionListRowView } from "./session-list-row-view";

export const meta = definePrototype({
  title: "Session list row",
  route: "/sessions",
  states: [
    { id: "working", label: "Working on a pull request" },
    { id: "needs-you", label: "Needs you" },
    { id: "coordinator", label: "Coordinator" },
    { id: "in-review", label: "In review" },
    { id: "merged", label: "Merged" },
    { id: "current", label: "Open session" },
    { id: "untitled", label: "Untitled" },
    { id: "archived", label: "Archived" },
  ],
});

const sessions = [...Object.values(SESSIONS_BOARD_SESSIONS).flat(), ...SESSIONS_BOARD_ARCHIVED];

const scenarios: Record<string, { id: string; current?: boolean }> = {
  working: { id: "s4" },
  "needs-you": { id: "s2" },
  coordinator: { id: "s1" },
  "in-review": { id: "s9" },
  merged: { id: "s14" },
  current: { id: "s4", current: true },
  untitled: { id: "s18" },
  archived: { id: "a1" },
};

const actions = sessionLifecycleActionsFixture();

export default function SessionListRowPrototype({ state }: PrototypeProps) {
  const scenario = scenarios[state] ?? scenarios.working!;
  const session = sessions.find(item => item.id === scenario.id);
  if (!session) throw new Error(`Missing session fixture "${scenario.id}"`);
  return (
    <PageContent className="max-w-sm" density="compact">
      <SessionListRowView
        current={scenario.current}
        onSelect={() => undefined}
        preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
        runtime={boardRuntimeFixture(session)}
        session={session}
        sessionActions={actions}
        showActions={false}
        testIdPrefix="prototype"
      />
    </PageContent>
  );
}
