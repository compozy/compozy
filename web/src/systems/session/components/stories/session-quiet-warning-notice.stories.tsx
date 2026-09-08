import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  quietWarningNoAutoStopSessionFixture,
  quietWarningSessionFixture,
} from "@/systems/session/mocks";

import { sessionQuietWarning } from "../../lib/session-quiet-warning";
import { SessionQuietWarningNotice } from "../session-quiet-warning-notice";

const scheduledStop = sessionQuietWarning(quietWarningSessionFixture)!;
const autoStopOff = sessionQuietWarning(quietWarningNoAutoStopSessionFixture)!;

const meta: Meta<typeof SessionQuietWarningNotice> = {
  title: "systems/session/components/SessionQuietWarningNotice",
  component: SessionQuietWarningNotice,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The single inactivity warning on the open session (US-014.EC-2). It rides the session window's notice slot as a warning `Alert` and states what the daemon stated when it warned: quiet for `quiet_after`, stopping after `stop_grace`. The only action is the public session stop; any work signal clears the notice from the daemon side, so there is no keep-alive control.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-3xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Quiet for 30 minutes with the inactivity stop scheduled 10 minutes after the warning. */
export const StopScheduled: Story = {
  args: {
    warning: scheduledStop,
    isStopping: false,
    onStop: fn(),
  },
};

/** `stop_grace = "0"`: the warning states that no automatic stop will follow. */
export const AutoStopOff: Story = {
  args: {
    warning: autoStopOff,
    isStopping: false,
    onStop: fn(),
  },
};

/** Stop now was activated: the action holds with a spinner until the daemon confirms. */
export const Stopping: Story = {
  args: {
    warning: scheduledStop,
    isStopping: true,
    onStop: fn(),
  },
};

/** A managed session the operator may not stop reads the warning without an action. */
export const WithoutAction: Story = {
  args: {
    warning: scheduledStop,
    isStopping: false,
  },
};
