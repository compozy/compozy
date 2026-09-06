import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  quietWarningNoAutoStopSessionFixture,
  quietWarningSessionFixture,
} from "@/systems/session/mocks";

import { type SessionQuietWarning, sessionQuietWarning } from "../../lib/session-quiet-warning";
import { SessionQuietStatusRow } from "../session-quiet-status-row";

const NOW = Date.now();
const MINUTE = 60_000;

/** Re-anchors a fixture episode on the story's mount time so the live clock reads the intended values. */
function anchored(warning: SessionQuietWarning, quietForMinutes: number): SessionQuietWarning {
  const shift = NOW - quietForMinutes * MINUTE - warning.quietSinceMs;
  return {
    quietSinceMs: warning.quietSinceMs + shift,
    warnedAtMs: warning.warnedAtMs + shift,
    stopAtMs: warning.stopAtMs === null ? null : warning.stopAtMs + shift,
  };
}

const scheduledStop = sessionQuietWarning(quietWarningSessionFixture)!;
const autoStopOff = sessionQuietWarning(quietWarningNoAutoStopSessionFixture)!;

/**
 * The status row while a session is quiet: elapsed since the last work signal
 * and the time left before the scheduled inactivity stop, both computed from
 * the daemon's instants on the shared 1 Hz clock. Neutral ink with a clock
 * glyph; the notice above carries the warning tone.
 */
const meta: Meta<typeof SessionQuietStatusRow> = {
  title: "systems/session/components/SessionQuietStatusRow",
  component: SessionQuietStatusRow,
  parameters: { layout: "centered" },
  decorators: [
    Story => (
      <div className="flex w-80 flex-col gap-4 border border-line bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** One minute after the warning: "Quiet for 31m · stops in 9m", counting live. */
export const StopScheduled: Story = {
  args: { warning: anchored(scheduledStop, 31) },
};

/** `stop_grace = "0"`: only the elapsed quiet time, no countdown to promise. */
export const AutoStopOff: Story = {
  args: { warning: anchored(autoStopOff, 31) },
};

/** The deadline passed and the daemon has not reported the stop yet. */
export const StopDue: Story = {
  args: { warning: anchored(scheduledStop, 41) },
};

/** Background window: the clock is paused, the last computed read stays. */
export const Paused: Story = {
  args: { warning: anchored(scheduledStop, 31), liveDataEnabled: false },
};
