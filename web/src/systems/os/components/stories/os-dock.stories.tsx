import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { ChevronRight, Search } from "lucide-react";

import { Button, useTopbarSlot } from "@agh/ui";

import { OsDock, OsDockZone, type OsDockItemData } from "../os-dock";
import { OsWindowFrame } from "../os-window-frame";
import { buildDeskItems, DesktopShell, DESK_APP_ITEMS, DESK_ITEMS } from "./_desktop";

const meta: Meta<typeof OsDock> = {
  title: "systems/os/components/OsDock",
  component: OsDock,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The dock: a centered glass strip of app launchers floating over the desktop, with a detached New Session control. Running shows a filled indicator, minimized a hollow one with a dimmed glyph, and badges bind to runtime projections (capped at 9+).",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const SESSION_ROWS = [
  { title: "Checkout flow polish", agent: "webgen", state: "running", time: "2m", live: true },
  {
    title: "Marketplace empty states",
    agent: "webgen",
    state: "waiting",
    time: "18m",
    live: false,
  },
  { title: "Dashboard spacing audit", agent: "webgen", state: "done", time: "1h", live: false },
  { title: "Competitor pricing scan", agent: "research", state: "running", time: "3h", live: true },
  { title: "ACP spec digest", agent: "research", state: "done", time: "5h", live: false },
  { title: "Nightly deps sweep", agent: "infra", state: "running", time: "41m", live: true },
] as const;

function SessionsNewAction() {
  useTopbarSlot({
    actions: (
      <Button size="sm" variant="ghost" onClick={fn()}>
        New session
      </Button>
    ),
  });
  return null;
}

function SessionsBody() {
  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <SessionsNewAction />
      <header className="flex shrink-0 items-center gap-2 px-3.5 pt-3 pb-1.5">
        <p className="flex-1 font-mono text-[10px] font-semibold tracking-[0.08em] text-subtle uppercase">
          Sessions <span className="ml-1.5 font-mono tracking-normal text-faint">7</span>
        </p>
      </header>
      <div className="shrink-0 px-3.5 pb-1.5">
        <label className="flex h-8 items-center gap-2 rounded-md border border-line bg-canvas-soft px-2.5 text-muted">
          <Search className="size-3.5 shrink-0" aria-hidden="true" />
          <input
            readOnly
            placeholder="Filter sessions…"
            aria-label="Filter sessions"
            className="min-w-0 flex-1 bg-transparent text-small-body text-fg outline-none placeholder:text-subtle"
          />
        </label>
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        <ul className="flex flex-col py-1">
          {SESSION_ROWS.map(row => (
            <li key={row.title} className="flex items-center gap-2 px-3.5 py-2">
              <span
                className={
                  (row.live
                    ? "bg-accent"
                    : row.state === "waiting"
                      ? "bg-warning"
                      : row.state === "done"
                        ? "bg-success"
                        : "bg-faint") + " size-[7px] shrink-0 rounded-full"
                }
              />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-small-body font-medium text-fg">
                  {row.title}
                </span>
                <span className="block truncate text-micro text-muted">
                  <b className="font-semibold text-subtle">{row.agent}</b>
                  <span className="text-faint"> · {row.state}</span>
                </span>
              </span>
              <span className="font-mono text-micro text-subtle">{row.time}</span>
            </li>
          ))}
        </ul>
        <button
          type="button"
          className="flex w-full items-center justify-between px-3.5 py-2.5 text-small-body text-muted hover:bg-hover hover:text-fg"
        >
          Show all sessions
          <ChevronRight className="size-3.5" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}

/**
 * Resting — full dock order with separators, detached New Session, running /
 * minimized / badge fixtures, and the Sessions window (search + seven rows +
 * Show all) over the ember desktop.
 */
const SESSIONS_DESK = buildDeskItems({ open: ["sessions"], minimized: ["loops"] });

export const Resting: Story = {
  args: { items: SESSIONS_DESK, onSelect: fn() },
  render: () => (
    <DesktopShell dock={false} dockItems={SESSIONS_DESK}>
      <OsWindowFrame
        title="Sessions"
        focused
        onTrafficLight={fn()}
        className="absolute top-[38px] left-[56px] h-[560px] w-[420px]"
      >
        <SessionsBody />
      </OsWindowFrame>
      <OsDockZone items={SESSIONS_DESK} onSelect={fn()} onNewSession={fn()} />
    </DesktopShell>
  ),
};

/**
 * Badge cap — counts above 9 collapse to "9+"; zero renders no badge.
 */
export const BadgeCap: Story = {
  args: {
    items: [
      { id: "sessions", name: "Sessions", icon: DESK_APP_ITEMS[0].icon, running: true, badge: 12 },
      { id: "tasks", name: "Tasks", icon: DESK_APP_ITEMS[4].icon, badge: 9 },
      { id: "agents", name: "Agents", icon: DESK_APP_ITEMS[2].icon },
    ] satisfies OsDockItemData[],
    onSelect: fn(),
  },
  render: args => (
    <DesktopShell dock={false}>
      <div className="absolute inset-x-0 bottom-2.5 flex justify-center">
        <OsDock {...args} />
      </div>
    </DesktopShell>
  ),
};

/**
 * Presentation-only — no callback, items render as inert chrome.
 */
export const PresentationOnly: Story = {
  args: { items: DESK_ITEMS },
  render: args => (
    <DesktopShell dock={false}>
      <OsDockZone items={args.items ?? DESK_ITEMS} />
    </DesktopShell>
  ),
};

/**
 * Hover magnify — OpenDesign proximity field (neighbors lift + scale). Move
 * the pointer across the dock strip to verify the falloff bulge.
 */
export const HoverMagnify: Story = {
  args: { items: DESK_ITEMS, onSelect: fn(), magnify: true },
  parameters: {
    docs: {
      description: {
        story:
          "Pointer proximity magnification matching OpenDesign `MAG_RADIUS=96` / `MAG_SCALE=0.34`. Compact presentation and prefers-reduced-motion disable the effect.",
      },
    },
  },
  render: () => (
    <DesktopShell dock={false}>
      <OsDockZone items={DESK_ITEMS} onSelect={fn()} onNewSession={fn()} magnify />
    </DesktopShell>
  ),
};
