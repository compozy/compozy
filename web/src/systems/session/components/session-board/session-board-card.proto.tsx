import { PageContent } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "#gallery/contract";

import {
  SESSIONS_BOARD_ARCHIVED,
  SESSIONS_BOARD_SESSIONS,
} from "../../mocks/sessions-board-fixtures";
import { boardRuntimeFixture } from "../../mocks/sessions-board-view-fixtures";
import { SessionBoardCardView } from "./session-board-card-view";

export const meta = definePrototype({
  title: "Session board card",
  spec: "sessions-kanban",
  surface: "S5",
  route: "/sessions?view=board",
  states: [
    { id: "needs-you", label: "Needs you · failing checks" },
    { id: "coordinator", label: "Coordinator with children" },
    { id: "running-draft", label: "Running on a draft PR" },
    { id: "ready-to-merge", label: "Approved and ready to merge" },
    { id: "merged", label: "Merged" },
    { id: "untitled", label: "Untitled" },
    { id: "current", label: "Open session" },
    { id: "archived", label: "Archived" },
  ],
});

const sessions = Object.values(SESSIONS_BOARD_SESSIONS).flat();

function session(id: string) {
  const found = [...sessions, ...SESSIONS_BOARD_ARCHIVED].find(item => item.id === id);
  if (!found) throw new Error(`Missing board fixture "${id}"`);
  return found;
}

const scenarios: Record<string, { id: string; current?: boolean; archived?: boolean }> = {
  "needs-you": { id: "s2" },
  coordinator: { id: "s1" },
  "running-draft": { id: "s4" },
  "ready-to-merge": { id: "s9" },
  merged: { id: "s14" },
  untitled: { id: "s18" },
  current: { id: "s4", current: true },
  archived: { id: "a1", archived: true },
};

export default function SessionBoardCardPrototype({ state }: PrototypeProps) {
  const scenario = scenarios[state] ?? scenarios["needs-you"];
  const card = session(scenario.id);
  return (
    <PageContent className="max-w-96" density="compact">
      <SessionBoardCardView
        archived={scenario.archived}
        current={scenario.current}
        onOpen={() => undefined}
        runtime={boardRuntimeFixture(card)}
        session={card}
      />
    </PageContent>
  );
}
