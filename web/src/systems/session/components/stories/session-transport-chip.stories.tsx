import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import {
  SessionTransportContext,
  type SessionTransportState,
} from "../../lib/session-transcript-thread-context-value";
import { SESSION_TRANSPORT_LIVE } from "../../lib/session-transport";
import { SessionTransportChip } from "../session-transport-chip";
import {
  SessionTransportFailureNotice,
  SessionTransportHistoryResetNotice,
} from "../session-transport-notices";

const LOST_AT = Date.parse("2026-09-06T14:02:00Z");

function transport(overrides: Partial<SessionTransportState>): SessionTransportState {
  return { ...SESSION_TRANSPORT_LIVE, lastLiveAt: LOST_AT, retry: fn(), ...overrides };
}

interface TransportStoryArgs {
  transport: SessionTransportState;
  windowLive: boolean;
}

/**
 * The S4 connection chip and its two end-of-transcript notices (task_06 VC-01..05).
 * Every state is the store's own phase; the 2s grace is already elapsed here.
 */
const meta: Meta<TransportStoryArgs> = {
  title: "systems/session/components/SessionTransportChip",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Connection chip at the window head's trailing edge: absent while the stream is healthy, info-toned while it comes back (Reconnecting · N, Catching up), neutral when a background window is paused, danger only once retries ran out. The notices below it state a dead stream (Try again) and a history reset.",
      },
    },
  },
  decorators: [
    (Story, context) => (
      <CenteredSurface>
        <SessionTransportContext.Provider value={context.args.transport}>
          <div className="flex w-full max-w-3xl flex-col gap-3">
            <div className="flex justify-end">
              <Story />
            </div>
            <SessionTransportHistoryResetNotice />
            <SessionTransportFailureNotice />
          </div>
        </SessionTransportContext.Provider>
      </CenteredSurface>
    ),
  ],
  render: args => <SessionTransportChip windowLive={args.windowLive} />,
  args: { windowLive: true },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Healthy stream: the chip slot stays empty. */
export const Live: Story = {
  args: { transport: transport({}) },
};

/** VC-01 — the stream dropped and the store is on its third retry. */
export const Reconnecting: Story = {
  args: {
    transport: transport({
      degradedAt: LOST_AT,
      phase: "waiting-reconnect",
      reconnectAttempt: 3,
    }),
  },
};

/** VC-02 — back, replaying the gap by sequence, plus the history-reset note. */
export const CatchingUpAfterReset: Story = {
  args: {
    transport: transport({
      catchingUp: true,
      historyReset: { at: LOST_AT + 60_000, generation: 4, reason: "generation_mismatch" },
      phase: "live",
    }),
  },
};

/** VC-03 — retries ran out: the only danger chip, with the stopped-updates marker and Try again. */
export const Disconnected: Story = {
  args: {
    transport: transport({
      degradedAt: LOST_AT,
      failure: { at: LOST_AT + 30_000, attempts: 6 },
      phase: "failed",
    }),
  },
};

/** VC-05 — an unfocused window keeps its last frame and reads paused. */
export const BackgroundPaused: Story = {
  args: {
    transport: transport({ lastLiveAt: Date.now() - 4 * 60_000, phase: "disabled" }),
    windowLive: false,
  },
};
