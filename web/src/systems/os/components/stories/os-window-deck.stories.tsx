import type { Meta, StoryObj } from "@storybook/react-vite";
import { ListChecks, Plus } from "lucide-react";
import { HttpResponse } from "msw";
import { fn } from "storybook/test";

import {
  Button,
  createTopbarSlotStore,
  Pill,
  PillGroup,
  type TopbarSlotStore,
  type TopbarSlotValue,
} from "@compozy/ui";

import { sessionFixtures } from "@/systems/session/mocks";
import { rehydrateActiveWorkspaceStore, setActiveWorkspaceId } from "@/systems/workspace";
import { storyDefaultWorkspaceId, storySessionIds } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import type { OsWindowFrameModel } from "../../lib/group-projection";
import type { OsAppId, OsWindow } from "../../lib/os-types";
import { OsWindowDeck } from "../os-window-deck";
import { ListBody, RunBody, SessionBody, WindowDeckScene } from "./os-window-deck-scene";

const STORY_DESKTOP_ID = "desktop:story";

const needsInputSession = {
  ...sessionFixtures.find(session => session.id === storySessionIds.product)!,
  badge: "waiting-for-auth" as const,
  id: "sess_docs_preview",
  name: "Deploy docs preview",
};

const polishCheckoutSession = {
  ...sessionFixtures.find(session => session.id === storySessionIds.cto)!,
  id: "sess_polish_checkout",
  name: "Polish checkout flow",
};

const refactorBillingSession = {
  ...sessionFixtures.find(session => session.id === storySessionIds.product)!,
  id: "sess_refactor_billing",
  name: "Refactor billing plans",
};

const deckSessions = [
  ...sessionFixtures.filter(
    session => session.id === storySessionIds.cto || session.id === storySessionIds.product
  ),
  needsInputSession,
  polishCheckoutSession,
  refactorBillingSession,
];

function storyWindow(id: string, app: OsAppId, overrides: Partial<OsWindow> = {}): OsWindow {
  return {
    id,
    app,
    instanceKey: app === "session" ? storySessionIds.cto : null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    pinned: false,
    desktopId: STORY_DESKTOP_ID,
    placement: "floating",
    rect: { x: 240, y: 118, w: 960, h: 450 },
    layer: 2,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: "stack:window-tabs-story",
    stackActive: false,
    parentAxis: null,
    ...overrides,
  };
}

function storyFrame(members: readonly string[], activeWindowId: string): OsWindowFrameModel {
  return {
    id: "stack:window-tabs-story",
    desktopId: STORY_DESKTOP_ID,
    kind: "floating",
    rect: { x: 240, y: 118, w: 960, h: 450 },
    members,
    activeWindowId,
    stackId: "stack:window-tabs-story",
    minimized: false,
    zoomed: false,
    adapted: false,
    layer: 2,
    zone: null,
    resizableEdges: { left: true, right: true, top: true, bottom: true },
  };
}

function slot(value: TopbarSlotValue): TopbarSlotStore {
  const store = createTopbarSlotStore();
  store.publishSlot({}, value);
  return store;
}

function runningStatus(label: string) {
  return (
    <span className="inline-flex items-center gap-1.5 text-small-body text-muted">
      <Pill.Dot pulse size="sm" tone="success" />
      {label}
    </span>
  );
}

function attentionStatus(label: string) {
  return (
    <span className="inline-flex items-center gap-1.5 text-small-body text-accent-strong">
      <Pill.Dot size="sm" tone="accent" />
      {label}
    </span>
  );
}

function sessionHeadGlyph(state: "running" | "needs-input") {
  return (
    <Pill.Dot
      aria-hidden="true"
      pulse={state === "running"}
      size="sm"
      tone={state === "running" ? "success" : "accent"}
    />
  );
}

const tasksSlot = slot({
  crumb: "Tasks",
  count: 6,
  glyph: <ListChecks aria-hidden="true" className="size-3.5" />,
  status: runningStatus("2 running"),
  actions: (
    <Button onClick={fn()} size="sm" type="button">
      <Plus aria-hidden="true" className="size-3" />
      New task
    </Button>
  ),
  toolbar: (
    <div className="flex w-full min-w-max items-center gap-2.5">
      <PillGroup
        aria-label="Task views"
        items={[
          { value: "list", label: "List" },
          { value: "kanban", label: "Kanban" },
        ]}
        onChange={fn()}
        value="list"
      />
      <PillGroup
        aria-label="Task filter"
        items={[
          { value: "all", label: "All", badge: 6 },
          { value: "running", label: "Running", badge: 2 },
          { value: "needs", label: "Needs you", badge: 1 },
        ]}
        onChange={fn()}
        size="sm"
        value="all"
      />
      <span className="min-w-4 flex-1" />
      <PillGroup
        aria-label="Task display"
        items={[
          { value: "rows", label: "Rows" },
          { value: "cards", label: "Cards" },
        ]}
        onChange={fn()}
        size="sm"
        value="rows"
      />
    </div>
  ),
});

const drillSlot = slot({
  crumb: "Run #418",
  crumbs: [
    { id: "tasks", label: "Tasks", onSelect: fn() },
    { id: "nightly-triage", label: "Nightly triage", onSelect: fn() },
  ],
  onBack: fn(),
  status: runningStatus("Running"),
  actions: (
    <Button onClick={fn()} size="sm" type="button">
      Open session
    </Button>
  ),
});

const sessionSlot = slot({
  crumb: "Executive launch review",
  glyph: sessionHeadGlyph("running"),
  glyphPresentation: "state",
  status: runningStatus("Running"),
});

const refactorBillingSlot = slot({
  crumb: "Refactor billing plans",
  glyph: sessionHeadGlyph("running"),
  glyphPresentation: "state",
  status: runningStatus("Running"),
});

const attentionSlot = slot({
  crumb: "Deploy docs preview",
  glyph: sessionHeadGlyph("needs-input"),
  glyphPresentation: "state",
  status: attentionStatus("Needs you"),
});

const sweepSlot = slot({ crumb: "Sweep flaky tests" });

const SLOT_STORES = new Map<string, TopbarSlotStore>([
  ["w-tasks", tasksSlot],
  ["w-run", drillSlot],
  ["w-session", sessionSlot],
  ["w-attention", attentionSlot],
]);

const DENSITY_SLOT_STORES = new Map(SLOT_STORES);
DENSITY_SLOT_STORES.set("w-run", refactorBillingSlot);
DENSITY_SLOT_STORES.set("w-jobs", sweepSlot);

const threeTabs: Record<string, OsWindow> = {
  "w-tasks": storyWindow("w-tasks", "tasks"),
  "w-run": storyWindow("w-run", "tasks"),
  "w-session": storyWindow("w-session", "session", { instanceKey: storySessionIds.cto }),
};

const densityTabs: Record<string, OsWindow> = {
  "w-tasks": storyWindow("w-tasks", "tasks"),
  "w-session": storyWindow("w-session", "session", { instanceKey: polishCheckoutSession.id }),
  "w-run": storyWindow("w-run", "session", { instanceKey: refactorBillingSession.id }),
  "w-attention": storyWindow("w-attention", "session", { instanceKey: needsInputSession.id }),
  "w-jobs": storyWindow("w-jobs", "jobs"),
  "w-agents": storyWindow("w-agents", "agents"),
  "w-marketplace": storyWindow("w-marketplace", "marketplace"),
};

const pinnedTabs: Record<string, OsWindow> = {
  "w-tasks": storyWindow("w-tasks", "tasks", { pinned: true }),
  "w-run": storyWindow("w-run", "agents", { pinned: true }),
  "w-session": storyWindow("w-session", "session", { instanceKey: storySessionIds.cto }),
  "w-attention": storyWindow("w-attention", "session", { instanceKey: needsInputSession.id }),
  "w-marketplace": storyWindow("w-marketplace", "marketplace"),
};

const meta: Meta<typeof OsWindowDeck> = {
  title: "systems/os/components/OsWindowDeck",
  component: OsWindowDeck,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Visual-contract states for the 37px window deck. The deck is the switcher; the active surface owns the head and optional route strip below it.",
      },
    },
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/sessions", () =>
          HttpResponse.json({
            sessions: deckSessions,
            page: { has_more: false, limit: 50, total: deckSessions.length },
          })
        ),
      ],
    }),
  },
  loaders: [
    async () => {
      await rehydrateActiveWorkspaceStore();
      setActiveWorkspaceId(storyDefaultWorkspaceId);
      return {};
    },
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — three tabs with a drilled-in Tasks leaf active; the head carries its full breadcrumb. */
export const VC01ThreeTabsDrilled: Story = {
  args: {
    frame: storyFrame(["w-tasks", "w-run", "w-session"], "w-run"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Run #418"
      frame={storyFrame(["w-tasks", "w-run", "w-session"], "w-run")}
      slotStores={SLOT_STORES}
      windows={threeTabs}
    >
      <RunBody />
    </WindowDeckScene>
  ),
};

/** VC-02 — the same frame with Tasks at root, including views, filters, spacer, and display mode. */
export const VC02TasksRootStrip: Story = {
  args: {
    frame: storyFrame(["w-tasks", "w-run", "w-session"], "w-tasks"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Tasks"
      frame={storyFrame(["w-tasks", "w-run", "w-session"], "w-tasks")}
      slotStores={SLOT_STORES}
      windows={threeTabs}
    >
      <ListBody />
    </WindowDeckScene>
  ),
};

/** VC-03 — the session document owns the active head and its composer beneath the deck. */
export const VC03SessionComposer: Story = {
  args: {
    frame: storyFrame(["w-tasks", "w-run", "w-session"], "w-session"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Executive launch review"
      frame={storyFrame(["w-tasks", "w-run", "w-session"], "w-session")}
      slotStores={SLOT_STORES}
      windows={threeTabs}
    >
      <SessionBody />
    </WindowDeckScene>
  ),
};

/** VC-04 — Tasks · Polish checkout · Refactor billing (active) · Deploy docs · Sweep · Agents · Marketplace. */
export const VC04DensitySignals: Story = {
  args: {
    frame: storyFrame(Object.keys(densityTabs), "w-run"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Refactor billing plans"
      frame={storyFrame(Object.keys(densityTabs), "w-run")}
      slotStores={DENSITY_SLOT_STORES}
      windows={densityTabs}
    >
      <SessionBody agentName="product-manager-agent" />
    </WindowDeckScene>
  ),
};

/** VC-05 — the deck costs nothing at one tab; traffic lights return to the active surface head. */
export const VC05SingleTab: Story = {
  args: {
    frame: storyFrame(["w-tasks"], "w-tasks"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Tasks"
      frame={storyFrame(["w-tasks"], "w-tasks")}
      single
      slotStores={SLOT_STORES}
      windows={{ "w-tasks": threeTabs["w-tasks"]! }}
    >
      <ListBody />
    </WindowDeckScene>
  ),
};

/** VC-06 — two glyph-only pinned tabs precede three unpinned surfaces (D5 and ADR-006). */
export const VC06PinnedMixedSurfaces: Story = {
  args: {
    frame: storyFrame(Object.keys(pinnedTabs), "w-attention"),
    onTrafficLight: fn(),
    slotStores: SLOT_STORES,
    dragHandleClassName: "deck-drag-handle",
  },
  render: () => (
    <WindowDeckScene
      activeTitle="Deploy docs preview"
      frame={storyFrame(Object.keys(pinnedTabs), "w-attention")}
      slotStores={SLOT_STORES}
      windows={pinnedTabs}
    >
      <SessionBody agentName="product-manager-agent" />
    </WindowDeckScene>
  ),
};
