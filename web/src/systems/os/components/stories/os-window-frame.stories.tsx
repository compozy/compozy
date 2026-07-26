import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { Button, useTopbarSlot } from "@agh/ui";

import { OsWindowFrame } from "../os-window-frame";
import { buildDeskItems, DesktopShell } from "./_desktop";

const meta: Meta<typeof OsWindowFrame> = {
  title: "systems/os/components/OsWindowFrame",
  component: OsWindowFrame,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Floating window frame — the shell chrome around one app's route subtree. Head carries traffic lights, left-aligned identity (glyph + title), and a trailing status/actions slot; optional context strip below. Frame depth (border + cast shadow) is the sanctioned shell-carve-out; window-body content stays flat.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function AgentsBody() {
  return (
    <ul className="flex flex-col py-1">
      {[
        {
          chip: "WG",
          name: "webgen",
          meta: "Claude Code · claude-fable-5",
          sessions: 3,
          live: true,
        },
        { chip: "RS", name: "research", meta: "Hermes · hermes-4", sessions: 2, live: false },
        { chip: "IF", name: "infra", meta: "OpenClaw · openclaw-2", sessions: 2, live: true },
      ].map(agent => (
        <li
          key={agent.name}
          className="flex items-center gap-3 border-b border-line-soft px-4 py-2.5 last:border-0"
        >
          <span className="grid size-workspace-avatar shrink-0 place-items-center rounded-sm border border-line-strong bg-elevated font-mono text-badge font-semibold text-muted">
            {agent.chip}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-small-body font-medium text-fg">{agent.name}</span>
            <span className="block truncate font-mono text-micro text-subtle">{agent.meta}</span>
          </span>
          <span className="flex items-center gap-1.5 text-micro text-muted">
            <span
              className={
                agent.live
                  ? "size-[7px] rounded-full bg-accent"
                  : "size-[7px] rounded-full bg-faint"
              }
            />
            {agent.sessions} sessions
          </span>
        </li>
      ))}
    </ul>
  );
}

function DashboardBody() {
  return (
    <div className="flex flex-col gap-4 p-4">
      <div className="grid grid-cols-4 gap-3">
        {[
          ["34", "runs today"],
          ["92%", "success"],
          ["$4.12", "cost today"],
          ["3", "active sessions"],
        ].map(([v, l]) => (
          <div key={l} className="rounded-md border border-line bg-canvas-soft p-3">
            <div className="font-mono text-metric-value text-fg-strong">{v}</div>
            <div className="text-micro text-muted">{l}</div>
          </div>
        ))}
      </div>
      <div className="rounded-md border border-line bg-canvas-soft p-3">
        <div className="mb-2 flex items-center justify-between">
          <span className="text-small-body font-semibold text-fg-strong">Working now</span>
          <span className="flex items-center gap-1.5 text-micro font-semibold text-accent">
            <span className="size-[7px] rounded-full bg-accent" />
            live
          </span>
        </div>
        <ul className="flex flex-col gap-2">
          {[
            ["Checkout flow polish", "webgen · claude-fable-5", "2m"],
            ["Competitor pricing scan", "research · hermes-4", "3h"],
            ["Nightly deps sweep", "infra · openclaw-2", "41m"],
          ].map(([t, m, time]) => (
            <li
              key={t}
              className="flex items-center gap-2 rounded-md border border-line bg-canvas px-3 py-2"
            >
              <span className="size-[7px] shrink-0 rounded-full bg-accent" />
              <span className="min-w-0 flex-1">
                <span className="block truncate text-small-body font-medium text-fg">{t}</span>
                <span className="block truncate text-micro text-muted">{m}</span>
              </span>
              <span className="font-mono text-micro text-subtle">{time}</span>
            </li>
          ))}
        </ul>
      </div>
      <div className="rounded-md border border-line bg-canvas-soft p-3">
        <div className="mb-2 text-small-body font-semibold text-fg-strong">Outcomes</div>
        <p className="mb-2 text-micro text-muted">Task runs — last 14 days</p>
        <div className="flex h-16 items-end gap-1">
          {[11, 14, 9, 16, 12, 18, 15, 10, 17, 13, 19, 14, 16, 21].map((v, i) => (
            <span
              key={i}
              className="min-w-0 flex-1 rounded-sm bg-fg/30"
              style={{ height: `${(v / 21) * 100}%` }}
            />
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * Focused — a single agents window with the sharp `--color-line-focus` border
 * and the cast `--shadow-window` over the ember desktop.
 */
export const Focused: Story = {
  args: { title: "Agents", focused: true, onTrafficLight: fn() },
  render: args => (
    <DesktopShell dockItems={buildDeskItems({ open: ["agents"] })}>
      <OsWindowFrame {...args} className="absolute top-[118px] left-[260px] h-[390px] w-[540px]">
        <AgentsBody />
      </OsWindowFrame>
    </DesktopShell>
  ),
};

/**
 * Unfocused — the agents window dimmed and on the lighter
 * `--shadow-window-unfocused`, beside a focused dashboard window.
 */
export const Unfocused: Story = {
  args: { title: "Agents", focused: false, onTrafficLight: fn() },
  render: args => (
    <DesktopShell dockItems={buildDeskItems({ open: ["agents", "dashboard"] })}>
      <OsWindowFrame {...args} className="absolute top-[80px] left-[120px] h-[390px] w-[540px]">
        <AgentsBody />
      </OsWindowFrame>
      <OsWindowFrame
        title="Dashboard"
        focused
        onTrafficLight={fn()}
        className="absolute top-[170px] left-[700px] h-[540px] w-[680px]"
      >
        <DashboardBody />
      </OsWindowFrame>
    </DesktopShell>
  ),
};

/**
 * Presentation-only — no traffic-light callback, so the controls render as
 * non-interactive chrome (truthful; no dead buttons).
 */
export const PresentationOnly: Story = {
  args: { title: "Tasks" },
  render: args => (
    <DesktopShell>
      <OsWindowFrame {...args} className="absolute top-[118px] left-[260px] h-[390px] w-[540px]">
        <AgentsBody />
      </OsWindowFrame>
    </DesktopShell>
  ),
};

function RouteActionPublisher() {
  useTopbarSlot({
    actions: (
      <Button size="sm" onClick={fn()}>
        New task
      </Button>
    ),
  });
  return <AgentsBody />;
}

/**
 * Route action published into the window head — the app's route calls
 * `useTopbarSlot` and its action renders in the trailing zone of this
 * window's `<Topbar>`, scoped to this window's provider.
 */
export const WithRouteAction: Story = {
  args: { title: "Tasks", focused: true, onTrafficLight: fn() },
  render: args => (
    <DesktopShell>
      <OsWindowFrame {...args} className="absolute top-[118px] left-[260px] h-[390px] w-[540px]">
        <RouteActionPublisher />
      </OsWindowFrame>
    </DesktopShell>
  ),
};
