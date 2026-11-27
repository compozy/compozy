/**
 * Sessions Kanban over several computers. It shares the List's window choices: Separate gives
 * each computer a band, with one lane per project inside under Group by Project; Together keeps
 * flat project · computer lanes; a scope on one computer shows that computer's board.
 */
import { definePrototype, type PrototypeProps } from "@protohq/cli";
import { useState } from "react";

import type {
  SessionListComputerLayout,
  SessionListGroupBy,
} from "@/systems/session/lib/session-list-preferences";
import type { SessionPayload } from "@/systems/session/types";

import { BoardWindow } from "../sessions-by-computer-kit/board";
import { useOpenOnMount } from "../sessions-by-computer-kit/hooks/use-open-on-mount";
import {
  ComputerScopePicker,
  type ComputerScope,
} from "../sessions-by-computer-kit/computer-scope-picker";
import {
  DEVBOX,
  ENTRIES,
  LOCAL,
  REMOTES,
  STUDIO,
  WORKSTATION,
  type ComputerSession,
} from "../sessions-by-computer-kit/fixtures";
import { railComputers, type ComputerRead } from "../sessions-by-computer-kit/model";

export const meta = definePrototype({
  title: "Sessions by computer · Kanban",
  spec: "remote-computers",
  surface: "Sessions Kanban",
  route: "/sessions",
  states: [
    {
      id: "kanban-bands-projects",
      label: "Separate, by project: one band per computer, project lanes",
    },
    { id: "kanban-view-options", label: "View options: the List's choices, shared by the window" },
    {
      id: "kanban-separate-none",
      label: "Separate, no grouping: one lane per computer, one failed",
    },
    { id: "kanban-together-project", label: "Together, by project: flat project · computer lanes" },
    { id: "kanban-scoped-studio", label: "Showing studio only: its board, grouped by project" },
  ],
});

interface Preset {
  layout: SessionListComputerLayout;
  groupBy: SessionListGroupBy;
  scope: ComputerScope;
  folded?: readonly string[];
  reads?: Readonly<Record<string, { read?: ComputerRead }>>;
  openOnMount?: string;
}

const BASE: Preset = {
  layout: "separate",
  groupBy: "project",
  scope: "all",
  folded: ["local", STUDIO.id, WORKSTATION.id],
};

const PRESETS: Record<string, Preset> = {
  "kanban-bands-projects": BASE,
  "kanban-view-options": {
    ...BASE,
    openOnMount: '[data-testid="draft-board-view-options-trigger"]',
  },
  "kanban-separate-none": {
    ...BASE,
    groupBy: "none",
    folded: [WORKSTATION.id],
    reads: { [DEVBOX.id]: { read: "failed" } },
  },
  "kanban-together-project": { ...BASE, layout: "together", folded: [] },
  "kanban-scoped-studio": { ...BASE, scope: STUDIO.id, folded: [] },
};

function byComputer(entries: readonly ComputerSession[]) {
  const map = new Map<string, SessionPayload[]>();
  for (const item of entries)
    map.set(item.computerId, [...(map.get(item.computerId) ?? []), item.session]);
  return map;
}

function SessionsKanban({ initial }: { initial: Preset }) {
  const [scope, setScope] = useState<ComputerScope>(initial.scope);
  const [groupBy, setGroupBy] = useState<SessionListGroupBy>(initial.groupBy);
  const [layout, setLayout] = useState<SessionListComputerLayout>(initial.layout);
  useOpenOnMount(initial.openOnMount);
  const all = railComputers(LOCAL, REMOTES, ENTRIES, initial.reads);
  const computers = scope === "all" ? all : all.filter(computer => computer.key === scope);
  return (
    <BoardWindow
      computers={computers}
      folded={initial.folded}
      groupBy={groupBy}
      layout={scope === "all" ? layout : "scoped"}
      onGroupByChange={setGroupBy}
      onLayoutChange={setLayout}
      scope={
        <ComputerScopePicker
          computers={REMOTES}
          local={LOCAL}
          onChange={setScope}
          sessionsByComputer={byComputer(ENTRIES)}
          value={scope}
        />
      }
    />
  );
}

export default function SessionsByComputerKanban({ state }: PrototypeProps) {
  return <SessionsKanban key={state} initial={PRESETS[state] ?? BASE} />;
}
