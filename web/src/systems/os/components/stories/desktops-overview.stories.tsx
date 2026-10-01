import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn } from "storybook/test";

import type { OsWindow } from "../../lib/os-types";
import type { LayoutProjection } from "../../lib/window-manager-types";
import { DesktopLayoutThumbnail } from "../desktop-layout-thumbnail";

import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewProps,
} from "../desktops-overview";
import { DesktopShell } from "./_desktop";

const WORK_AREA = { x: 0, y: 0, w: 1380, h: 848 };

function storyWindow(
  id: string,
  app: OsWindow["app"],
  rect: OsWindow["rect"],
  extra: Partial<OsWindow> = {}
): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    pinned: false,
    desktopId: "story",
    placement: "tiled",
    rect,
    layer: 1,
    minimized: false,
    zoomed: false,
    groupId: null,
    nodeId: null,
    stackId: null,
    stackActive: true,
    parentAxis: null,
    ...extra,
  } as OsWindow;
}

/**
 * A thumbnail over a real-shaped projection: windows sharing a `stackId` tile
 * as one stack; `floating` windows (single or decked) stay out of the tiling.
 */
function thumbnail(tiled: OsWindow[], floating: OsWindow[] = []) {
  const stacks = new Map<string, OsWindow[]>();
  for (const window of tiled) {
    if (window.stackId) stacks.set(window.stackId, [...(stacks.get(window.stackId) ?? []), window]);
  }
  const zone = { x: 0, y: 0, w: 1, h: 1 };
  const projection: LayoutProjection = {
    revision: 1,
    desktopId: "story",
    workArea: WORK_AREA,
    windows: tiled.map(window => ({
      windowId: window.id,
      nodeId: window.id,
      groupId: "story",
      rect: window.rect,
      zone,
      stackId: window.stackId,
      active: window.stackActive,
      adapted: false,
      parentAxis: null,
    })),
    stacks: [...stacks].map(([nodeId, members]) => ({
      nodeId,
      groupId: "story",
      kind: "explicit" as const,
      windowIds: members.map(window => window.id),
      activeWindowId: members.find(window => window.stackActive)?.id ?? members[0].id,
      rect: members[0].rect,
      zone,
    })),
    seams: [],
    frameSeams: [],
    diagnostics: [],
  };
  return <DesktopLayoutThumbnail projection={projection} windows={[...tiled, ...floating]} />;
}

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Main",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃1",
    thumbnail: thumbnail([
      storyWindow(
        "s1",
        "session",
        { x: 0, y: 0, w: 760, h: 848 },
        { stackId: "st", stackActive: true }
      ),
      storyWindow(
        "s2",
        "session",
        { x: 0, y: 0, w: 760, h: 848 },
        { stackId: "st", stackActive: false }
      ),
      storyWindow("tasks", "tasks", { x: 760, y: 0, w: 620, h: 480 }),
      storyWindow("term", "terminal", { x: 760, y: 480, w: 620, h: 368 }),
    ]),
    windows: [
      { id: "s1", title: "Session" },
      { id: "s2", title: "Session" },
      { id: "tasks", title: "Tasks" },
      { id: "term", title: "Terminal" },
    ],
  },
  {
    id: "build",
    name: "Review",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃2",
    needsYou: true,
    thumbnail: thumbnail(
      [
        storyWindow("agents", "agents", { x: 0, y: 0, w: 860, h: 848 }),
        storyWindow("loops", "loops", { x: 860, y: 0, w: 520, h: 848 }),
      ],
      // A floating tab deck: one tile, its active tab +N.
      ["t1", "t2", "t3"].map((id, index) =>
        storyWindow(
          id,
          "terminal",
          { x: 620, y: 420, w: 560, h: 340 },
          { placement: "stacked", stackId: "deck", stackActive: index === 0, layer: 2 }
        )
      )
    ),
    windows: [
      { id: "agents", title: "Agents" },
      { id: "loops", title: "Loops" },
      { id: "t1", title: "Terminal" },
      { id: "t2", title: "Terminal" },
      { id: "t3", title: "Terminal" },
    ],
  },
  {
    id: "research",
    name: "Research",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃3",
    thumbnail: thumbnail([
      storyWindow("knowledge", "knowledge", { x: 0, y: 0, w: 1380, h: 424 }),
      storyWindow("vault", "vault", { x: 0, y: 424, w: 1380, h: 424 }),
    ]),
    windows: [
      { id: "knowledge", title: "Knowledge" },
      { id: "vault", title: "Vault" },
    ],
  },
];

/** Mounts the overview in a desk-sized overlay host, as `DesktopManagerSurfaces` does. */
function InDesk(props: DesktopsOverviewProps) {
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  return (
    <DesktopShell>
      <div
        ref={setHost}
        className="contain-paint pointer-events-none absolute inset-0 z-40 [&:not(:empty)]:pointer-events-auto"
      />
      <DesktopsOverview {...props} container={host} />
    </DesktopShell>
  );
}

const ACTIONS = {
  onOpenChange: fn<DesktopsOverviewProps["onOpenChange"]>(),
  onCreateDesktop: fn<DesktopsOverviewProps["onCreateDesktop"]>(),
  onSwitchDesktop: fn<DesktopsOverviewProps["onSwitchDesktop"]>(),
  onRenameDesktop: fn<DesktopsOverviewProps["onRenameDesktop"]>(),
  onReorderDesktop: fn<DesktopsOverviewProps["onReorderDesktop"]>(),
  onDeleteDesktop: fn<DesktopsOverviewProps["onDeleteDesktop"]>(),
  onMoveWindow: fn<DesktopsOverviewProps["onMoveWindow"]>(),
};

const meta: Meta<typeof DesktopsOverview> = {
  title: "systems/os/components/DesktopsOverview",
  component: DesktopsOverview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "On-demand desktop management over the desk (shell-rail v2 `.ov`): live layout thumbnails, keycaps, needs-you dots, and New desktop; each card's menu renames, reorders, moves a window, or deletes with transfer. Arrows move across cards, Enter switches, Esc closes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Resolved workspace state with one current desktop and one focus desktop. */
export const Ready: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** Initial snapshot acquisition keeps the final card geometry stable. */
export const Loading: Story = {
  args: { open: true, state: { status: "loading" }, ...ACTIONS },
  render: args => <InDesk {...args} />,
};

/** Transport failure exposes a single recovery action without fabricated desktop data. */
export const Error: Story = {
  args: {
    open: true,
    state: { status: "error", message: "The workspace desktop snapshot is unavailable." },
    onRetry: fn(),
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** Revision conflict asks the shell to reload before accepting more mutations. */
export const Conflict: Story = {
  args: {
    open: true,
    state: {
      status: "conflict",
      message: "Another client changed the desktop order after this view opened.",
    },
    onResolveConflict: fn(),
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** A real empty workspace offers desktop creation as its only next action. */
export const Empty: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: [], activeDesktopId: null },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};
