import type { Meta, StoryObj } from "@storybook/react-vite";

import { SessionCreateProvider, type SessionCreateContextValue } from "@/systems/session";

import type { HomeAgentRow } from "../../hooks/use-home-agents";
import type { HomeSystemModel } from "../../hooks/use-home-system";
import type { HomeRunCardModel, HomeWorkingNowStatus } from "../../types";
import { makeHomeOverview } from "../../mocks/fixtures";
import type { HomeActivityEvent } from "../../types";
import { HomeActivityFeed } from "../home-activity-feed";
import { HomeAgentsPanel } from "../home-agents-panel";
import { HomeAttentionZone } from "../home-attention-zone";
import { HomeKpiStrip } from "../home-kpi-strip";
import { HomeNetworkPanel } from "../home-network-panel";
import { HomeOutcomesChart } from "../home-outcomes-chart";
import { HomePageMeta } from "../home-page-meta";
import { HomePulseHeatmap } from "../home-pulse-heatmap";
import { HomeSystemPanel } from "../home-system-panel";
import { HomeUsageChart } from "../home-usage-chart";
import { HomeWorkingNow } from "../home-working-now";

const NOW = Date.parse("2026-07-23T12:00:00Z");

const STORY_SESSION_CREATE: SessionCreateContextValue = {
  openForAgent: () => {},
  isCreating: false,
  pendingAgentName: null,
  hasActiveWorkspace: true,
};

const overview = makeHomeOverview({
  pulse: {
    window_days: 14,
    buckets: Array.from({ length: 7 * 24 }, (_, index) => {
      const weekday = Math.floor(index / 24);
      const hour = index % 24;
      const working = weekday >= 1 && weekday <= 5 && hour >= 9 && hour <= 18;
      const events = working
        ? 6 + ((weekday * 31 + hour * 17) % 13)
        : hour >= 11 && hour <= 15
          ? 2
          : 0;
      return { weekday, hour, events: weekday === 2 && hour === 14 ? 42 : events };
    }),
    busiest: { weekday: 2, hour: 14, events: 42 },
    longest_session: {
      session_id: "sess-long",
      agent_name: "writer",
      duration_seconds: 8040,
      date: "2026-07-22",
    },
  },
  usage: {
    window_days: 30,
    retention_days: 60,
    truncated: false,
    total_tokens: 1_400_000,
    estimated_cost: 21.4,
    cost_currency: "USD",
    cost_status: "estimated",
    days: Array.from({ length: 30 }, (_, index) => ({
      date: `Jun ${index + 1}`,
      tokens: 18_000 + ((index * 37) % 11) * 6_500,
    })),
    agent_share: [
      { agent_name: "writer", tokens: 644_000, fraction: 0.46 },
      { agent_name: "researcher", tokens: 434_000, fraction: 0.31 },
      { agent_name: "ops", tokens: 210_000, fraction: 0.15 },
      { agent_name: "other", tokens: 112_000, fraction: 0.08 },
    ],
  },
});

const workingNowCards: HomeRunCardModel[] = [
  {
    key: "session:sess-1",
    kind: "session",
    agentName: "writer",
    title: "Writer",
    subtitle: "Running Write",
    elapsedBaseSeconds: 728,
    baseAtMs: NOW,
    sessionLink: { agentName: "writer", sessionId: "sess-1" },
  },
  {
    key: "session:sess-2",
    kind: "session",
    agentName: "researcher",
    title: "Researcher",
    subtitle: "Running WebFetch",
    elapsedBaseSeconds: 48,
    baseAtMs: NOW,
    sessionLink: { agentName: "researcher", sessionId: "sess-2" },
  },
  {
    key: "run:run-1",
    kind: "task_run",
    agentName: "publisher",
    title: "Publish changelog",
    subtitle: "Run running",
    elapsedBaseSeconds: 312,
    baseAtMs: NOW,
    runLink: { taskId: "task-pub", runId: "run-1" },
  },
];

const networkRows = [
  { key: "peers", label: "Peers online", value: "2", tone: "success" as const },
  { key: "work", label: "Open work", value: "3" },
  { key: "messages", label: "Messages today", value: "128", mono: true },
  { key: "channels", label: "Channels", value: "3", mono: true },
  { key: "wakes", label: "Wakes used", value: "12", mono: true },
];

const agentRows: HomeAgentRow[] = [
  { name: "writer", active: 1, sessions: 14, failed: 1, runtimeSeconds: 15_120, runtimeShare: 1 },
  {
    name: "researcher",
    active: 1,
    sessions: 9,
    failed: 0,
    runtimeSeconds: 7_560,
    runtimeShare: 0.5,
  },
  { name: "ops", active: 0, sessions: 6, failed: 1, runtimeSeconds: 3_600, runtimeShare: 0.24 },
  {
    name: "publisher",
    active: 1,
    sessions: 3,
    failed: 0,
    runtimeSeconds: 1_440,
    runtimeShare: 0.1,
  },
];

function activityEvent(
  id: string,
  type: string,
  agent: string,
  summary: string,
  minutesAgo: number,
  outcome = "info"
): HomeActivityEvent {
  return {
    id,
    type,
    agent_name: agent,
    session_id: "sess-1",
    spawn_depth: 0,
    summary,
    outcome,
    timestamp: new Date(NOW - minutesAgo * 60_000).toISOString(),
  } as HomeActivityEvent;
}

const activityEvents: HomeActivityEvent[] = [
  activityEvent(
    "evt-1",
    "task.run_completed",
    "writer",
    "finished “Campaign brief v2”",
    2,
    "success"
  ),
  activityEvent("evt-2", "session.started", "researcher", "started a session", 12),
  activityEvent(
    "evt-3",
    "task.needs_attention",
    "",
    "Competitor research paused, waiting on input",
    45,
    "warning"
  ),
  activityEvent("evt-4", "task.approved", "", "You approved “Brand kit refresh”", 64),
  activityEvent(
    "evt-5",
    "task.run_failed",
    "publisher",
    "Publish run failed — retry queued",
    180,
    "failure"
  ),
  activityEvent("evt-6", "tool_call", "writer", "used Write twice on the brief draft", 300),
  activityEvent("evt-7", "config.read", "ops", "read workspace config", 360),
  activityEvent("evt-8", "memory.compaction_completed", "", "Memory compaction finished", 420),
];

const systemModel: HomeSystemModel = {
  allNormal: true,
  summary: "v0.4.2 · up 2d 4h · providers 3/3 · scheduler running",
  tiles: [
    {
      key: "daemon",
      label: "Daemon",
      value: "Running",
      detail: "v0.4.2 · up 2d 4h",
      tone: "success",
    },
    {
      key: "providers",
      label: "Providers",
      value: "3 of 3 ready",
      detail: "anthropic · openai · google",
      tone: "success",
    },
    {
      key: "scheduler",
      label: "Scheduler",
      value: "Running",
      detail: "next wake 09:57",
      tone: "success",
    },
    {
      key: "memory",
      label: "Memory",
      value: "Enabled",
      detail: "dream consolidation on",
      tone: "success",
    },
    { key: "hooks", label: "Hooks", value: "24 runs today", detail: "0 failed", tone: "success" },
    {
      key: "retention",
      label: "Retention",
      value: "60 days",
      detail: "events · usage · permissions",
    },
  ],
};

interface HomeDashboardStoryProps {
  systemOpen?: boolean;
  workingNowStatus?: HomeWorkingNowStatus;
  workingNowErrorMessage?: string;
}

function HomeDashboardStory({
  systemOpen = false,
  workingNowStatus = "ready",
  workingNowErrorMessage,
}: HomeDashboardStoryProps) {
  const workingNowIsError = workingNowStatus === "error";
  const visibleWorkingNowCards = workingNowIsError ? [] : workingNowCards;
  return (
    <SessionCreateProvider value={STORY_SESSION_CREATE}>
      <div className="mx-auto w-full max-w-[1240px] px-9 pt-6 pb-20">
        <HomePageMeta today={new Date(NOW)} workspaceName="launch-hq" />
        <div className="flex flex-col gap-6">
          <HomeAttentionZone attention={overview.attention} />
          <HomeKpiStrip
            overview={overview}
            workingNowDetail="2 sessions · 1 task run"
            workingNowTotal={3}
          />
          <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
            <HomeWorkingNow
              cards={visibleWorkingNowCards}
              errorMessage={workingNowErrorMessage}
              status={workingNowStatus}
              total={visibleWorkingNowCards.length}
            />
            <HomeNetworkPanel rows={networkRows} />
          </div>
          <HomePulseHeatmap pulse={overview.pulse} />
          <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
            <HomeOutcomesChart outcomes={overview.outcomes} />
            <HomeUsageChart onWindowChange={() => {}} usage={overview.usage} window={30} />
          </div>
          <div className="grid grid-cols-1 items-stretch gap-5 min-[1080px]:grid-cols-2">
            <HomeAgentsPanel rows={agentRows} />
            <HomeActivityFeed events={activityEvents} />
          </div>
          <HomeSystemPanel onOpenChange={() => {}} open={systemOpen} system={systemModel} />
        </div>
      </div>
    </SessionCreateProvider>
  );
}

const meta: Meta<typeof HomeDashboardStory> = {
  title: "systems/dashboard/components/HomeDashboard",
  component: HomeDashboardStory,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The 7-zone end-user home: needs-you, KPI strip, working-now | network, pulse heatmap, outcomes | usage, agents | activity, and the folded system row.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const FullPage: Story = {};

export const SystemExpanded: Story = {
  render: () => <HomeDashboardStory systemOpen />,
};

export const WorkingNowPartial: Story = {
  args: { workingNowStatus: "partial" },
};

export const WorkingNowError: Story = {
  args: {
    workingNowStatus: "error",
    workingNowErrorMessage: "The daemon did not return active work.",
  },
};
