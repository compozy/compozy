import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { ChevronRight, Search } from "lucide-react";

import { Button, useTopbarSlot } from "@compozy/ui";

import {
  projectTerminalBadge,
  terminalsRunning,
  type TerminalBadgeInput,
  type TerminalRunState,
} from "@/systems/terminal/parts";

import { OsDock, type OsDockItemData } from "../os-dock";
import { OsWindowFrame } from "../os-window-frame";
import { buildDeskItems, DesktopShell, DESK_ITEMS } from "./_desktop";

const meta: Meta<typeof OsDock> = {
  title: "systems/os/components/OsDock",
  component: OsDock,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The rail: the dock as a vertical launcher column on the chrome surface beside the desktop. A dot at the left marks a running app, the focused app sits on the selected plate, minimized shows a hollow ring with a dimmed glyph, and badges bind to runtime projections (capped at 9+). Up/Down move focus between launchers.",
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
 * Resting — full rail order with running / focused / minimized / badge
 * fixtures and the Sessions window over the ember desktop.
 */
const SESSIONS_DESK = buildDeskItems({
  open: ["dashboard"],
  active: "sessions",
  minimized: ["loops"],
});

export const Resting: Story = {
  args: { items: SESSIONS_DESK, onSelect: fn() },
  render: () => (
    <DesktopShell dockItems={SESSIONS_DESK}>
      <OsWindowFrame
        title="Sessions"
        focused
        onTrafficLight={fn()}
        className="absolute top-[38px] left-[56px] h-[560px] w-[420px]"
      >
        <SessionsBody />
      </OsWindowFrame>
    </DesktopShell>
  ),
};

/** Standalone rail column at a fixed height, as the shell grid sizes it. */
function RailFrame({ children }: { children: React.ReactNode }) {
  return <div className="flex h-[560px] w-fit bg-desk">{children}</div>;
}

/**
 * Badge cap — counts above 9 collapse to "9+"; zero renders no badge.
 */
export const BadgeCap: Story = {
  args: {
    items: [
      { id: "sessions", name: "Sessions", icon: "sessions", running: true, badge: 12 },
      { id: "tasks", name: "Tasks", icon: "tasks", badge: 9 },
      { id: "agents", name: "Agents", icon: "agents", badge: 0 },
    ] satisfies OsDockItemData[],
    onSelect: fn(),
  },
  render: args => (
    <RailFrame>
      <OsDock {...args} />
    </RailFrame>
  ),
};

/**
 * Presentation-only — no callback, items render as inert chrome.
 */
export const PresentationOnly: Story = {
  args: { items: DESK_ITEMS },
  render: args => (
    <RailFrame>
      <OsDock items={args.items ?? DESK_ITEMS} />
    </RailFrame>
  ),
};

/**
 * VC-03 — the terminal launcher, in the three states its runtime projection
 * produces. The count is never authored here: `projectTerminalBadge` counts the
 * questions and approvals waiting on this person under the profile they are
 * working as, and `terminalsRunning` decides the indicator. Zero renders
 * nothing at all, so the resting launcher is indistinguishable from any other
 * closed app.
 *
 * The launcher is registered by the activation tranche; this story composes the
 * dock with the projection to hold the badge to its contract before then.
 */
const TERMINAL_BADGE_STATES: { label: string; input: TerminalBadgeInput }[] = [
  {
    label: "Nothing waiting",
    input: {
      scopeKey: "8:ws-atlas4:work",
      profileId: "work",
      inputRequests: [],
      pendingApprovalCount: 0,
    },
  },
  {
    label: "One question waiting",
    input: {
      scopeKey: "8:ws-atlas4:work",
      profileId: "work",
      inputRequests: [{ profile_id: "work" }],
      pendingApprovalCount: 0,
    },
  },
  {
    label: "Three waiting",
    input: {
      scopeKey: "8:ws-atlas4:work",
      profileId: "work",
      // The last row belongs to another profile and must not be counted.
      inputRequests: [{ profile_id: "work" }, { profile_id: "work" }, { profile_id: "personal" }],
      pendingApprovalCount: 1,
    },
  },
];

const TERMINAL_STRIP_TERMINALS: { state: TerminalRunState }[][] = [
  [],
  [{ state: "running" }],
  [{ state: "running" }, { state: "exited" }],
];

export const TerminalLauncher: Story = {
  args: { items: DESK_ITEMS },
  parameters: {
    docs: {
      description: {
        story:
          "Visual Contract VC-03. Left to right: resting, one question waiting, three waiting. The third strip's fixture includes a row owned by another profile to prove the count stays inside the profile you are working as.",
      },
    },
  },
  render: () => (
    <div className="flex h-[240px] w-fit gap-4 bg-desk">
      {TERMINAL_BADGE_STATES.map((state, index) => (
        <OsDock
          key={state.label}
          aria-label={`Dock — ${state.label}`}
          items={
            [
              { id: "sessions", name: "Sessions", icon: "sessions" },
              {
                id: "terminal",
                name: "Terminal",
                icon: "terminal",
                running: terminalsRunning(TERMINAL_STRIP_TERMINALS[index]),
                badge: projectTerminalBadge(state.input).count,
              },
              { id: "tasks", name: "Tasks", icon: "tasks" },
            ] satisfies OsDockItemData[]
          }
        />
      ))}
    </div>
  ),
};
