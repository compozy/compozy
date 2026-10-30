import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, screen, userEvent, within } from "storybook/test";

import { processesLeftRunningFixture } from "../../mocks/process-fixtures";
import { ProcessesLeftRunningDialog } from "../processes-left-running-dialog";
import { SessionPrimaryAction } from "../session-primary-action";
import { SessionProcessChoiceDialog } from "../session-process-choice-dialog";

/**
 * What CompozyOS asks and shows about the processes a session left running: the stop, archive,
 * and bulk choices (stop them too, keep them, or cancel) and "Processes left running".
 */
const meta: Meta = {
  title: "systems/session/components/SessionProcesses",
  parameters: { layout: "centered" },
};
export default meta;
type Story = StoryObj<typeof meta>;

/** The topbar stop asks while three processes run, even without a Goal to pause. */
export const StopWithProcesses: Story = {
  tags: ["play-fn"],
  render: () => (
    <SessionPrimaryAction
      showUnarchiveAction={false}
      showStopAction
      controlsBusy={false}
      isUnarchiving={false}
      isStopping={false}
      onUnarchive={fn()}
      onStop={fn()}
      hasLiveGoal={false}
      runningProcesses={3}
      processPorts={[5173, 8080, 9090]}
    />
  ),
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId("stop-button"));
    await expect(
      await screen.findByRole("button", { name: "Stop session and 3 processes" })
    ).toBeInTheDocument();
  },
};

/** Archive from a list row or the board: no dialog at zero, this choice above it. */
export const ArchiveWithProcesses: Story = {
  render: () => (
    <SessionProcessChoiceDialog
      open
      action="archive"
      sessionCount={1}
      processCount={2}
      ports={[5173]}
      onOpenChange={fn()}
      onChoose={fn()}
    />
  ),
};

/** One choice covers a bulk stop of the selection. */
export const BulkStopWithProcesses: Story = {
  render: () => (
    <SessionProcessChoiceDialog
      open
      action="stop"
      sessionCount={4}
      processCount={5}
      ports={[3000, 4321, 6006]}
      onOpenChange={fn()}
      onChoose={fn()}
    />
  ),
};

/** The palette's "Processes left running": grouped by session, Stop per row, per session, all. */
export const LeftRunning: Story = {
  render: () => (
    <ProcessesLeftRunningDialog
      open
      onOpenChange={fn()}
      groups={processesLeftRunningFixture}
      onStop={fn()}
    />
  ),
};

export const LeftRunningLoading: Story = {
  render: () => (
    <ProcessesLeftRunningDialog open onOpenChange={fn()} groups={[]} loading onStop={fn()} />
  ),
};

export const LeftRunningEmpty: Story = {
  render: () => <ProcessesLeftRunningDialog open onOpenChange={fn()} groups={[]} onStop={fn()} />,
};

export const LeftRunningError: Story = {
  render: () => (
    <ProcessesLeftRunningDialog
      open
      onOpenChange={fn()}
      groups={[]}
      failed
      onRetry={fn()}
      onStop={fn()}
    />
  ),
};
