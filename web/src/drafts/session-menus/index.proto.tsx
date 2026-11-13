/**
 * Grouped session menus with list/Kanban parity. Linear's pattern: a short top level of frequent
 * verbs, related verbs behind a submenu (Copy ▸, New from this ▸, Pull request ▸, Move ▸). One
 * tree serves right-click and `…` on list rows and Kanban cards; the selection menu is the same
 * tree minus the verbs that only mean something for one session. Right-click any row or card in
 * any state to try it; each state pins one menu open for review.
 */
import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ParityMatrix } from "./session-menu-parity";
import { COORDINATOR_FIRST_LINK } from "./session-menus-fixtures";
import { BoardSurface, ListSurface, type PinnedMenu } from "./session-menu-surfaces";

export const meta = definePrototype({
  title: "Session menus: grouped, list and Kanban parity",
  route: "/sessions",
  states: [
    { id: "list-row", label: "List · session menu" },
    { id: "list-copy", label: "List · Copy submenu" },
    { id: "list-move", label: "List · Move submenu" },
    { id: "list-derive", label: "List · New from this submenu" },
    { id: "list-linked-many", label: "List · Linked work, 13 items" },
    { id: "list-linked-copy", label: "List · Copy, several pull requests" },
    { id: "board-card", label: "Kanban · card menu" },
    { id: "board-pr", label: "Kanban · Pull request submenu" },
    { id: "board-move", label: "Kanban · Move submenu (no manual order)" },
    { id: "board-linked-many", label: "Kanban · Linked work, 13 items" },
    { id: "list-selection", label: "List · 3 selected, More actions" },
    { id: "board-selection", label: "Kanban · 3 selected, Move submenu" },
    { id: "parity", label: "Parity matrix" },
  ],
});

const SELECTED = ["s9", "s7", "s12"];

const PINNED: Record<string, PinnedMenu> = {
  "list-row": { sessionId: "s4" },
  "list-copy": { sessionId: "s4", openSubs: ["copy"] },
  "list-move": { sessionId: "s9", openSubs: ["move"] },
  "list-derive": { sessionId: "s4", openSubs: ["derive"] },
  "list-linked-many": { sessionId: "s1", openSubs: ["linked-work", COORDINATOR_FIRST_LINK] },
  "list-linked-copy": { sessionId: "s1", openSubs: ["copy"] },
  "board-card": { sessionId: "s4" },
  "board-pr": { sessionId: "s2", openSubs: ["pull-request"] },
  "board-move": { sessionId: "s9", openSubs: ["move"] },
  "board-linked-many": { sessionId: "s1", openSubs: ["linked-work"] },
  "list-selection": { selection: true },
  "board-selection": { selection: true, openSubs: ["move"] },
};

export default function SessionMenusDraft({ state }: PrototypeProps) {
  if (state === "parity") {
    return (
      <main className="min-h-dvh bg-canvas p-8">
        <ParityMatrix />
      </main>
    );
  }
  const pinned = PINNED[state] ?? {};
  if (state.startsWith("board")) {
    return (
      <main className="min-h-dvh bg-canvas">
        <BoardSurface pinned={pinned} selected={state === "board-selection" ? SELECTED : []} />
      </main>
    );
  }
  return (
    <main className="min-h-dvh bg-canvas p-8">
      <ListSurface pinned={pinned} selected={state === "list-selection" ? SELECTED : []} />
    </main>
  );
}
