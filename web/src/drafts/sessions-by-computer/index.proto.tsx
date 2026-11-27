/**
 * Sessions List over several computers. View options adds Computers: Together merges every
 * computer into the window's grouping; Separate gives each computer its own section with the
 * same grouping inside, so Separate + Project reads computer → project → folders. A folder
 * lives on one computer. The scope beside List | Kanban narrows the window to one computer.
 */
import { definePrototype, type PrototypeProps } from "@protohq/cli";
import { useState } from "react";

import type { ComputerFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import type {
  SessionListComputerLayout,
  SessionListGroupBy,
} from "@/systems/session/lib/session-list-preferences";
import type { SessionSelectionVerbs } from "@/systems/session/lib/session-selection-verbs";
import type { SessionPayload } from "@/systems/session/types";

import { ComputerRail } from "../sessions-by-computer-kit/computer-rail";
import {
  ComputerScopePicker,
  type ComputerScope,
} from "../sessions-by-computer-kit/computer-scope-picker";
import {
  DEVBOX,
  ENTRIES,
  GPU_VM,
  HETZNER,
  HOMELAB,
  LOCAL,
  MINI_BUILD,
  REMOTES,
  SCALE_COMPUTERS,
  SCALE_ENTRIES,
  STUDIO,
  WORKSTATION,
  type ComputerSession,
} from "../sessions-by-computer-kit/fixtures";
import {
  railComputers,
  type ComputerRead,
  type RailLayout,
} from "../sessions-by-computer-kit/model";
import type { RailLook } from "../sessions-by-computer-kit/rail-look";
import {
  ListWindow,
  OfflineNotice,
  OpenSession,
  ScopeRemovedNotice,
} from "../sessions-by-computer-kit/windows";

export const meta = definePrototype({
  title: "Sessions by computer · List",
  spec: "remote-computers",
  surface: "Sessions List",
  route: "/sessions",
  states: [
    {
      id: "list-separate-project",
      label: "Separate, by project: computer, then project, then folders",
    },
    {
      id: "list-view-options",
      label: "View options: Group by, and Computers Together or Separate",
    },
    {
      id: "list-together-project",
      label: "Together, by project: computer mark, project, then computer",
    },
    {
      id: "list-computer-plus",
      label: "A computer's +: New session or New folder on that computer",
    },
    { id: "list-toolbar-plus", label: "Toolbar +: New folder off, folders live on one computer" },
    { id: "list-folder-drop", label: "Drag into a folder of the same project on devbox" },
    { id: "list-drop-refused", label: "Drag over another computer: refused, with the reason" },
    {
      id: "list-scoped-folders",
      label: "Showing devbox only: project folders, session folders, Manual",
    },
    {
      id: "list-separate-none",
      label: "Separate, no grouping: flat rows per computer, with a pager",
    },
    {
      id: "list-computer-states",
      label: "Every computer state: loading, failed, offline, plan, version",
    },
    {
      id: "list-search",
      label: "Filter: computers with no match hide, offline matches from memory",
    },
    {
      id: "list-bulk-two-computers",
      label: "Selection on two computers: Move to folder says why not",
    },
    { id: "list-volume", label: "Nine computers folded, the scope picker open with search" },
    { id: "scope-removed", label: "The scoped computer was removed: back to All computers" },
  ],
});

interface Preset {
  layout: RailLayout;
  groupBy: SessionListGroupBy;
  scope: ComputerScope;
  computers: readonly ComputerFixture[];
  entries: readonly ComputerSession[];
  reads?: Readonly<Record<string, { read?: ComputerRead; more?: boolean }>>;
  look?: Partial<RailLook>;
  openOnMount?: string | readonly string[];
  pickerOpen?: boolean;
  selection?: boolean;
  removed?: string;
}

const FOLDED_OFFLINE = [`computer:${WORKSTATION.id}`];
// The devbox states fold the computers above it, so its rows and menus stay on screen.
const DEVBOX_IN_VIEW = ["computer:local", `computer:${STUDIO.id}`, ...FOLDED_OFFLINE];

const BASE: Preset = {
  layout: "separate",
  groupBy: "project",
  scope: "all",
  computers: REMOTES,
  entries: ENTRIES,
  look: { collapsed: new Set(FOLDED_OFFLINE) },
};

const STATES_COMPUTERS: readonly ComputerFixture[] = [
  STUDIO,
  { ...SCALE_COMPUTERS[3]!, name: "ci-runner-01" },
  HETZNER,
  DEVBOX,
  WORKSTATION,
  HOMELAB,
  MINI_BUILD,
  GPU_VM,
];

const SELECTION: SessionSelectionVerbs = {
  count: 2,
  markSeenable: 2,
  topLevelable: 0,
  stoppable: 0,
  archivable: 2,
  archivableLive: 0,
  unarchivable: 0,
  laterable: 2,
  backFromLaterable: 0,
  disabled: false,
  onDelete: () => undefined,
  onArchive: () => undefined,
  onMoveToLater: () => undefined,
  onMarkSeen: () => undefined,
  onCopyIds: () => undefined,
  onCopyTitles: () => undefined,
  branches: 0,
  onCopyBranches: () => undefined,
  folders: {
    folders: [],
    blockedReason: "Sessions on different computers can't share a folder.",
    sharedFolderId: null,
    canUnfile: false,
    onMove: () => undefined,
    onNewFolder: () => undefined,
  },
};

const PRESETS: Record<string, Preset> = {
  "list-separate-project": BASE,
  "list-view-options": { ...BASE, openOnMount: '[data-testid="draft-rail-view-options-trigger"]' },
  "list-together-project": { ...BASE, layout: "together", look: {} },
  "list-computer-plus": {
    ...BASE,
    look: { collapsed: new Set(DEVBOX_IN_VIEW) },
    openOnMount: `[data-testid="draft-computer-${DEVBOX.id}-new"]`,
  },
  "list-toolbar-plus": { ...BASE, openOnMount: '[data-testid="draft-rail-new"]' },
  "list-folder-drop": {
    ...BASE,
    look: { collapsed: new Set(DEVBOX_IN_VIEW), dragging: "d1", dropInto: "f_spei" },
  },
  "list-drop-refused": {
    ...BASE,
    look: { collapsed: new Set(DEVBOX_IN_VIEW), dragging: "d1", refusedOn: STUDIO.id },
  },
  "list-scoped-folders": { ...BASE, layout: "scoped", scope: DEVBOX.id, look: {} },
  "list-separate-none": {
    ...BASE,
    look: { collapsed: new Set(DEVBOX_IN_VIEW) },
    groupBy: "none",
    reads: { [DEVBOX.id]: { more: true } },
  },
  "list-computer-states": {
    ...BASE,
    computers: STATES_COMPUTERS,
    reads: { [DEVBOX.id]: { read: "failed" } },
    look: { collapsed: new Set(DEVBOX_IN_VIEW) },
  },
  "list-search": { ...BASE, look: { query: "webhook" } },
  "list-bulk-two-computers": {
    ...BASE,
    selection: true,
    look: {
      collapsed: new Set([`computer:${STUDIO.id}`, ...FOLDED_OFFLINE]),
      selected: new Set(["l1", "d1"]),
    },
    openOnMount: [
      '[data-testid="draft-rail-selection-more"]',
      '[data-testid="draft-rail-selection-move"]',
    ],
  },
  "list-volume": {
    ...BASE,
    computers: SCALE_COMPUTERS,
    entries: SCALE_ENTRIES,
    pickerOpen: true,
    look: {
      collapsed: new Set([
        "computer:local",
        ...SCALE_COMPUTERS.map(computer => `computer:${computer.id}`),
      ]),
    },
  },
  "scope-removed": { ...BASE, removed: "gpu-vm" },
};

/** The open session: the devbox question, or the first row the scope holds. */
function openSessionFor(scoped: readonly ComputerSession[]): SessionPayload {
  const chosen =
    scoped.find(item => item.session.id === "d1")?.session ??
    scoped[0]?.session ??
    ENTRIES.find(item => item.session.id === "d1")?.session ??
    ENTRIES[0]?.session;
  if (!chosen) throw new Error("Missing sessions-by-computer fixture entries");
  return chosen;
}

function byComputer(entries: readonly ComputerSession[]) {
  const map = new Map<string, SessionPayload[]>();
  for (const item of entries)
    map.set(item.computerId, [...(map.get(item.computerId) ?? []), item.session]);
  return map;
}

function SessionsList({ initial }: { initial: Preset }) {
  const [scope, setScope] = useState<ComputerScope>(initial.scope);
  const [groupBy, setGroupBy] = useState<SessionListGroupBy>(initial.groupBy);
  const [layout, setLayout] = useState<SessionListComputerLayout>(
    initial.layout === "together" ? "together" : "separate"
  );
  const all = railComputers(LOCAL, initial.computers, initial.entries, initial.reads);
  const computers = scope === "all" ? all : all.filter(computer => computer.key === scope);
  const effectiveLayout: RailLayout = scope === "all" ? layout : "scoped";
  const scoped = initial.entries.filter(item => scope === "all" || item.computerId === scope);
  const open = openSessionFor(scoped);
  const scopeComputer = initial.computers.find(computer => computer.id === scope);
  const offlineScope = scopeComputer?.status === "offline";
  const look: RailLook = { collapsed: new Set(), query: "", currentRow: open.id, ...initial.look };
  let notice = null;
  if (initial.removed) notice = <ScopeRemovedNotice name={initial.removed} />;
  else if (offlineScope && scopeComputer) notice = <OfflineNotice computer={scopeComputer} />;
  return (
    <ListWindow
      rail={
        <ComputerRail
          computers={computers}
          groupBy={groupBy}
          layout={effectiveLayout}
          look={look}
          notice={notice}
          onGroupByChange={setGroupBy}
          onLayoutChange={setLayout}
          openOnMount={initial.openOnMount}
          selection={initial.selection ? SELECTION : undefined}
        />
      }
      scope={
        <ComputerScopePicker
          computers={initial.computers}
          defaultOpen={initial.pickerOpen}
          local={LOCAL}
          onChange={setScope}
          sessionsByComputer={byComputer(initial.entries)}
          value={scope}
        />
      }
      session={open}
    >
      <OpenSession offline={offlineScope} session={open} />
    </ListWindow>
  );
}

export default function SessionsByComputerList({ state }: PrototypeProps) {
  return <SessionsList key={state} initial={PRESETS[state] ?? BASE} />;
}
