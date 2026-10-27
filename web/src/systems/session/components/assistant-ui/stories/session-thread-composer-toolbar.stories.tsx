import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, screen, userEvent, within } from "storybook/test";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import type { SessionMonitorPayload } from "@/systems/session/adapters/session-monitor-api";
import { MonitorComposerControl } from "@/systems/session/components/monitor-composer-control";
import { SubagentComposerControl } from "@/systems/session/components/subagents/subagent-composer-control";
import { primarySessionFixture } from "@/systems/session/mocks";

import { baseArgs, composerToolbarArgs } from "./session-thread-story-runtime";
import { mixedStreamingTranscript, transcriptPayload } from "./session-thread-story-transcripts";

const storyWorkspaceId = primarySessionFixture.workspace_id ?? "ws_alpha";

/**
 * The session composer's control row (issue #826). Every control is 28 px tall:
 * icon-only tools are the `InputGroupButton` `tool-icon` square, tools with a
 * label or count are the padded `tool` pill, 4 px apart inside the tool and
 * action groups, which wrap in narrow windows.
 */
const meta: Meta<typeof SessionThread> = {
  title: "systems/session/components/assistant-ui/SessionThread/ComposerToolbar",
  component: SessionThread,
  parameters: {
    layout: "fullscreen",
    ...storybookMswParameters({
      session: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/sessions/{session_id}/transcript", () =>
          HttpResponse.json(transcriptPayload(mixedStreamingTranscript))
        ),
      ],
    }),
  },
  decorators: [
    Story => (
      <SessionChatRuntimeProvider
        sessionId={primarySessionFixture.id}
        workspaceId={storyWorkspaceId}
      >
        <div className="flex h-[640px] w-full flex-col bg-background border border-line">
          <Story />
        </div>
      </SessionChatRuntimeProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof meta>;

/** Idle — model picker, folder, context ring, subagents and attach, then the send disc. */
export const Idle: Story = {
  args: { ...baseArgs, ...composerToolbarArgs },
};

/**
 * Turn running — the live subagents count widens its tool into a pill, and
 * Queue, Steer, Interrupt and Stop share the 28 px row with no visible shortcut
 * badges (the Enter hint stays for screen readers).
 */
export const Busy: Story = {
  args: {
    ...baseArgs,
    ...composerToolbarArgs,
    subagentControl: (
      <SubagentComposerControl
        subagents={[]}
        counts={{ live: 2, total: 3, failed: 0, attention: 0 }}
      />
    ),
    isSessionRunning: true,
    allowBusyInput: true,
    onQueuePrompt: () => undefined,
    onInterruptPrompt: () => undefined,
    onSteerPrompt: () => undefined,
  },
};

/**
 * The subagents popover while its roster loads: placeholder rows in the roster's own frame, one
 * per row the session's summary reports, under the real heading and count.
 */
export const SubagentsLoading: Story = {
  tags: ["play-fn"],
  args: {
    ...baseArgs,
    ...composerToolbarArgs,
    subagentControl: (
      <SubagentComposerControl
        subagents={[]}
        counts={{ live: 0, total: 3, failed: 0, attention: 0 }}
        loading
      />
    ),
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(await within(canvasElement).findByTestId("composer-subagents-button"));
    await expect(await screen.findByText("Loading subagents")).toBeInTheDocument();
  },
};

const IN_TWELVE_MINUTES = new Date(Date.now() + 12 * 60_000).toISOString();

const STORY_MONITORS: readonly SessionMonitorPayload[] = [
  {
    monitor_id: "mon-1",
    session_id: primarySessionFixture.id,
    kind: "timer",
    description: "Check CI on PR #875",
    status: "active",
    created_at: new Date().toISOString(),
    deadline_at: IN_TWELVE_MINUTES,
  },
  {
    monitor_id: "mon-2",
    session_id: primarySessionFixture.id,
    kind: "terminal",
    description: "Wait for the release build",
    status: "active",
    until: "match",
    pattern: "published v\\d+",
    created_at: new Date().toISOString(),
    deadline_at: IN_TWELVE_MINUTES,
  },
];

/** The monitors popover with its rows: what each one watches, when it fires or gives up, Cancel. */
export const Monitors: Story = {
  tags: ["play-fn"],
  args: {
    ...baseArgs,
    ...composerToolbarArgs,
    monitorControl: (
      <MonitorComposerControl
        active={STORY_MONITORS.length}
        monitors={STORY_MONITORS}
        onCancel={() => undefined}
      />
    ),
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(await within(canvasElement).findByTestId("composer-monitors-button"));
    await expect(await screen.findByTestId("composer-monitor-mon-1")).toBeInTheDocument();
  },
};

/**
 * The monitors popover while its list loads: one placeholder row per active monitor the session's
 * summary reports, in the live row's own frame, under the real heading.
 */
export const MonitorsLoading: Story = {
  tags: ["play-fn"],
  args: {
    ...baseArgs,
    ...composerToolbarArgs,
    monitorControl: <MonitorComposerControl active={STORY_MONITORS.length} monitors={[]} loading />,
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(await within(canvasElement).findByTestId("composer-monitors-button"));
    await expect(await screen.findByText("Loading monitors")).toBeInTheDocument();
  },
};
