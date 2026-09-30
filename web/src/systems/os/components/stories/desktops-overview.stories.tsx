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

function thumbnail(windows: OsWindow[]) {
  return (
    <DesktopLayoutThumbnail
      projection={{ workArea: WORK_AREA } as LayoutProjection}
      windows={windows}
    />
  );
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
    thumbnail: thumbnail([
      storyWindow("agents", "agents", { x: 0, y: 0, w: 860, h: 848 }),
      storyWindow("loops", "loops", { x: 860, y: 0, w: 520, h: 848 }),
    ]),
    windows: [
      { id: "agents", title: "Agents" },
      { id: "loops", title: "Loops" },
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
