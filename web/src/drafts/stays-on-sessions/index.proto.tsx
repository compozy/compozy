import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  SessionsWindow,
  type RailGroup,
  type SessionRailProps,
} from "../stays-on-agents-shared/session-frame";
import {
  ADVOS_ROW,
  COMPOZY_ROW,
  CREW,
  CROWDED,
  EACH_STATE,
  GLOBAL_ROW,
  RELOADDEV_ROW,
  TWO_NEED_YOU,
} from "../stays-on-agents-shared/sessions-fixtures";
import {
  AgentTurn,
  DraftComposer,
  TranscriptColumn,
  UserTurn,
} from "../stays-on-agents-shared/transcript";
import { INSTANCES, RELEASE, SCOUT } from "../stays-on-agents-shared/fixtures";
import { KanbanWindow } from "./kanban";
import { NeedsYouToast } from "./needs-you-toast";
import { emptyColumns, liveColumns } from "./kanban-columns";

export const meta = definePrototype({
  title: "Stays-on sessions",
  spec: "always-on-agent",
  surface: "S10 S15",
  route: "/sessions",
  states: [
    { id: "sidebar-rows", label: "S10 · Global and project rows" },
    { id: "sidebar-hover", label: "S10 · Hover card, working" },
    { id: "sidebar-hover-needs", label: "S10 · Hover card, failed" },
    { id: "sidebar-crew", label: "S10 · Twelve agents in one project" },
    { id: "sidebar-crew-open", label: "S10 · The whole crew" },
    { id: "sidebar-two-need-you", label: "S10 · Two agents need you" },
    { id: "sidebar-toast", label: "S10 · Needs you arrives elsewhere" },
    { id: "sidebar-none", label: "S10 · No instances, no row" },
    { id: "sidebar-overflow", label: "S10 · Overflow +N" },
    { id: "sidebar-collapsed", label: "S10 · Collapsed project signal" },
    { id: "sidebar-states", label: "S10 · Each avatar state" },
    { id: "sidebar-focus", label: "S10 · Keyboard focus" },
    { id: "sidebar-menu", label: "S10 · Context menu" },
    { id: "sidebar-paused-menu", label: "S10 · Context menu, paused with reason" },
    { id: "sidebar-add-menu", label: "S10 · Project + menu" },
    { id: "sidebar-definition-menu", label: "S10 · Resume blocked, definition gone" },
    { id: "add-picker", label: "S16 · Project + picks an agent" },
    { id: "add-picker-none", label: "S16 · Nothing available" },
    { id: "add-picker-cap", label: "S16 · Cap reached" },
    { id: "add-picker-auth", label: "S16 · Provider not authenticated" },
    { id: "sidebar-flat", label: "S10 · Flat mode" },
    { id: "sidebar-spawned", label: "S10 · Spawned sessions, creator badge" },
    { id: "kanban-rows", label: "S15 · Global and project rows" },
    { id: "kanban-crew", label: "S15 · Twelve agents in one project" },
    { id: "kanban-hover", label: "S15 · Hover card" },
    { id: "kanban-none", label: "S15 · No instances" },
    { id: "kanban-overflow", label: "S15 · Overflow +N" },
    { id: "kanban-attention", label: "S15 · Needing attention" },
    { id: "kanban-hide-children", label: "S15 · Hide child sessions on" },
    { id: "kanban-instance-only", label: "S15 · Scope with only an instance" },
  ],
});

const COMPOZY: RailGroup = {
  name: "compozy",
  count: 4,
  rows: ["s4", "s2", "s9"],
  instances: COMPOZY_ROW,
  currentRow: "s4",
};
const RELOADDEV: RailGroup = {
  name: "reloaddev",
  count: 2,
  rows: ["s1"],
  instances: RELOADDEV_ROW,
};
const ADVOS: RailGroup = { name: "advos", count: 1, rows: ["s14"], instances: ADVOS_ROW };
const PLAIN = [COMPOZY, RELOADDEV, ADVOS];

const RAILS: Record<string, SessionRailProps> = {
  "sidebar-rows": { groups: PLAIN, top: GLOBAL_ROW },
  "sidebar-hover": {
    groups: [
      { ...COMPOZY, rowProps: { cardFor: INSTANCES.orchestratorCompozy.id } },
      RELOADDEV,
      ADVOS,
    ],
    top: GLOBAL_ROW,
  },
  "sidebar-hover-needs": {
    groups: [{ ...COMPOZY, instances: CREW, rowProps: { cardFor: "c-release" } }, RELOADDEV],
    top: GLOBAL_ROW,
  },
  "sidebar-crew": { groups: [{ ...COMPOZY, instances: CREW }, RELOADDEV], top: GLOBAL_ROW },
  "sidebar-crew-open": {
    groups: [{ ...COMPOZY, instances: CREW, rowProps: { crewOpen: true } }, RELOADDEV],
    top: GLOBAL_ROW,
  },
  "sidebar-two-need-you": {
    groups: [{ ...COMPOZY, instances: TWO_NEED_YOU }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "sidebar-toast": {
    groups: [{ ...COMPOZY, currentRow: "s4" }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "sidebar-none": {
    groups: PLAIN.map(group => ({ ...group, instances: [] })),
  },
  "sidebar-overflow": {
    groups: [{ ...COMPOZY, instances: CROWDED, rowProps: { limit: 5 } }, RELOADDEV],
    top: GLOBAL_ROW,
  },
  "sidebar-collapsed": {
    groups: [{ ...COMPOZY, collapsed: true }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "sidebar-states": {
    groups: [{ ...COMPOZY, instances: EACH_STATE }, RELOADDEV],
    top: GLOBAL_ROW,
  },
  "sidebar-focus": {
    groups: [
      { ...COMPOZY, rowProps: { focusedId: INSTANCES.orchestratorCompozy.id } },
      RELOADDEV,
      ADVOS,
    ],
    top: GLOBAL_ROW,
  },
  "sidebar-menu": {
    groups: [
      { ...COMPOZY, rowProps: { openMenuFor: INSTANCES.orchestratorCompozy.id } },
      RELOADDEV,
      ADVOS,
    ],
    top: GLOBAL_ROW,
  },
  "sidebar-paused-menu": {
    groups: [COMPOZY, { ...ADVOS, rowProps: { openMenuFor: INSTANCES.triageAdvos.id } }],
    top: GLOBAL_ROW,
  },
  "sidebar-add-menu": {
    groups: [{ ...COMPOZY, menuOpen: true }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "sidebar-definition-menu": {
    groups: [
      COMPOZY,
      {
        ...ADVOS,
        instances: [INSTANCES.triageAdvosGone],
        rowProps: { openMenuFor: INSTANCES.triageAdvosGone.id },
      },
    ],
    top: GLOBAL_ROW,
  },
  "add-picker": {
    groups: [{ ...COMPOZY, picker: { agents: [RELEASE, SCOUT] } }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "add-picker-none": {
    groups: [{ ...COMPOZY, picker: { agents: [] } }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "add-picker-cap": {
    groups: [{ ...COMPOZY, picker: { agents: [RELEASE, SCOUT], capped: true } }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
  "add-picker-auth": {
    groups: [
      { ...COMPOZY, picker: { agents: [RELEASE, SCOUT], signedOut: ["codex"] } },
      RELOADDEV,
      ADVOS,
    ],
    top: GLOBAL_ROW,
  },
  "sidebar-flat": {
    groups: [],
    top: [...GLOBAL_ROW, ...COMPOZY_ROW],
    flatRows: ["s4", "s2", "s9", "s1", "s14"],
    currentRow: "s4",
  },
  "sidebar-spawned": {
    groups: [{ ...COMPOZY, badges: true }, RELOADDEV, ADVOS],
    top: GLOBAL_ROW,
  },
};

function SessionBody() {
  return (
    <>
      <TranscriptColumn>
        <UserTurn>Open the PR for the topbar chip when checks pass.</UserTurn>
        <AgentTurn>Checks are green. I opened #931 and asked Reviewer to read the diff.</AgentTurn>
      </TranscriptColumn>
      <DraftComposer placeholder="Message Claude" />
    </>
  );
}

export default function StaysOnSessionsDraft({ state }: PrototypeProps) {
  const rail = RAILS[state];
  if (rail) {
    return (
      <SessionsWindow
        overlay={state === "sidebar-toast" ? <NeedsYouToast /> : undefined}
        rail={rail}
        title="Topbar PR chip — web slice"
      >
        <SessionBody />
      </SessionsWindow>
    );
  }
  const live = liveColumns();
  const quiet = emptyColumns();
  switch (state) {
    case "kanban-none":
      return <KanbanWindow lanes={[{ name: "compozy", count: 6, columns: live }]} />;
    case "kanban-overflow":
      return (
        <KanbanWindow
          lanes={[{ name: "compozy", count: 6, columns: live, instances: CROWDED, limit: 5 }]}
          top={GLOBAL_ROW}
        />
      );
    case "kanban-crew":
      return (
        <KanbanWindow
          lanes={[{ name: "compozy", count: 6, columns: live, instances: CREW }]}
          top={GLOBAL_ROW}
        />
      );
    case "kanban-hover":
      return (
        <KanbanWindow
          lanes={[
            {
              name: "compozy",
              count: 6,
              columns: live,
              instances: CREW,
              rowProps: { cardFor: INSTANCES.orchestratorCompozy.id },
            },
          ]}
          top={GLOBAL_ROW}
        />
      );
    case "kanban-attention":
      return (
        <KanbanWindow
          lanes={[{ name: "compozy", count: 6, columns: live, instances: COMPOZY_ROW }]}
          top={GLOBAL_ROW}
        />
      );
    case "kanban-hide-children":
      return (
        <KanbanWindow
          initialHide
          lanes={[{ name: "compozy", count: 3, columns: live, instances: COMPOZY_ROW }]}
          top={GLOBAL_ROW}
        />
      );
    case "kanban-instance-only":
      return (
        <KanbanWindow
          lanes={[
            { name: "advos", count: 0, columns: quiet, instances: ADVOS_ROW },
            { name: "compozy", count: 6, columns: live, instances: COMPOZY_ROW },
          ]}
        />
      );
    default:
      return (
        <KanbanWindow
          lanes={[
            { name: "compozy", count: 6, columns: live, instances: COMPOZY_ROW },
            { name: "reloaddev", count: 0, columns: quiet, instances: RELOADDEV_ROW },
          ]}
          top={GLOBAL_ROW}
        />
      );
  }
}
