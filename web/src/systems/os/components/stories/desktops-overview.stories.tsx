import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewProps,
} from "../desktops-overview";
import { DesktopShell } from "./_desktop";

function DesktopThumbnail({ layout }: { layout: "control" | "build" | "research" }) {
  if (layout === "build") {
    return (
      <span className="grid h-full w-full grid-cols-[1fr_1.35fr] gap-1 p-1.5">
        <span className="rounded-xs border border-line bg-canvas-soft" />
        <span className="rounded-xs border border-line bg-elevated" />
      </span>
    );
  }
  if (layout === "research") {
    return (
      <span className="grid h-full w-full grid-rows-2 gap-1 p-1.5">
        <span className="rounded-xs border border-line bg-canvas-soft" />
        <span className="ml-[18%] rounded-xs border border-line bg-elevated" />
      </span>
    );
  }
  return (
    <span className="relative h-full w-full p-1.5">
      <span className="absolute inset-y-1.5 left-1.5 w-[58%] rounded-xs border border-line bg-canvas-soft" />
      <span className="absolute right-1.5 bottom-1.5 h-[58%] w-[48%] rounded-xs border border-line bg-elevated shadow-raised" />
    </span>
  );
}

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Control",
    purpose: "standard",
    thumbnail: <DesktopThumbnail layout="control" />,
    windows: [
      { id: "dashboard", title: "Dashboard", detail: "Workspace status" },
      { id: "sessions", title: "Sessions", detail: "6 active agents" },
    ],
  },
  {
    id: "build",
    name: "Build",
    purpose: "focus",
    thumbnail: <DesktopThumbnail layout="build" />,
    windows: [
      { id: "agents", title: "Agents", detail: "Implement window manager" },
      { id: "tasks", title: "Tasks", detail: "Desktop persistence" },
    ],
  },
  {
    id: "research",
    name: "Research",
    purpose: "standard",
    thumbnail: <DesktopThumbnail layout="research" />,
    windows: [{ id: "knowledge", title: "Knowledge", detail: "Reference projects" }],
  },
];

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
          "On-demand desktop management over an authoritative snapshot: switch, create, rename, reorder, transfer on delete, and move individual windows.",
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
  render: args => (
    <DesktopShell wallpaper="carbon">
      <DesktopsOverview {...args} />
    </DesktopShell>
  ),
};

/** Initial snapshot acquisition keeps the final card geometry stable. */
export const Loading: Story = {
  args: { open: true, state: { status: "loading" }, ...ACTIONS },
  render: args => (
    <DesktopShell wallpaper="carbon">
      <DesktopsOverview {...args} />
    </DesktopShell>
  ),
};

/** Transport failure exposes a single recovery action without fabricated desktop data. */
export const Error: Story = {
  args: {
    open: true,
    state: { status: "error", message: "The workspace desktop snapshot is unavailable." },
    onRetry: fn(),
    ...ACTIONS,
  },
  render: args => (
    <DesktopShell wallpaper="carbon">
      <DesktopsOverview {...args} />
    </DesktopShell>
  ),
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
  render: args => (
    <DesktopShell wallpaper="carbon">
      <DesktopsOverview {...args} />
    </DesktopShell>
  ),
};

/** A real empty workspace offers desktop creation as its only next action. */
export const Empty: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: [], activeDesktopId: null },
    ...ACTIONS,
  },
  render: args => (
    <DesktopShell wallpaper="carbon">
      <DesktopsOverview {...args} />
    </DesktopShell>
  ),
};
