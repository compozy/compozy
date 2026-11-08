import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";

import { CheckInToolCallRows, CheckInToolCallsDisclosure } from "@/systems/agent";

import { REPORT_CHOICES, REPORT_TOOL_CALLS, REPORT_TRIAGE } from "../../mocks/report-fixtures";
import { Notice, ShoulderTap } from "../always-on/reports/report-entry";

const SUMMARY = { total: 4, writes_done: 1, writes_maybe: 1, denied: 1 };

function ToolCalls({ open = false }: { open?: boolean }) {
  return (
    <CheckInToolCallsDisclosure defaultOpen={open} summary={SUMMARY}>
      <CheckInToolCallRows calls={REPORT_TOOL_CALLS} />
    </CheckInToolCallsDisclosure>
  );
}

function Frame({ children }: { children: ReactNode }) {
  return <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">{children}</div>;
}

const meta: Meta<typeof ShoulderTap> = {
  title: "systems/session/components/CheckInReports",
  component: ShoulderTap,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "A check-in's report in the main conversation: picks go through the daemon, auto-accepted actions show as done, and the tool-call line lists what its tools did.",
      },
    },
  },
  decorators: [
    Story => (
      <Frame>
        <Story />
      </Frame>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const ask = "I found two things worth doing. Which one first?";

/** U1: waiting for a pick, with the tool-call line closed. */
export const Waiting: Story = {
  render: () => (
    <ShoulderTap
      agent={REPORT_TRIAGE}
      ask={ask}
      choices={REPORT_CHOICES}
      footer={<ToolCalls />}
      onChoose={() => undefined}
      when="23:30"
    />
  ),
};

/** U1: a pick on its way, then one that failed with its code. */
export const PickingAndFailed: Story = {
  render: () => (
    <>
      <ShoulderTap
        agent={REPORT_TRIAGE}
        ask={ask}
        choices={REPORT_CHOICES}
        onChoose={() => undefined}
        picking={0}
        when="23:30"
      />
      <ShoulderTap
        agent={REPORT_TRIAGE}
        ask={ask}
        choices={REPORT_CHOICES}
        error={
          <>
            The agent is busy; pick it when ready.{" "}
            <span className="text-subtle">(session_input_queue_full)</span>
          </>
        }
        onChoose={() => undefined}
        when="23:30"
      />
    </>
  ),
};

/** U1: picked by you; the ask folds to a record and keeps the tool-call line. */
export const PickedByYou: Story = {
  render: () => (
    <ShoulderTap
      agent={REPORT_TRIAGE}
      ask={ask}
      choices={REPORT_CHOICES}
      chosen="Close the stale design issue"
      footer={<ToolCalls />}
      when="23:30"
    />
  ),
};

/** U2: one action ran without a pick; the others say why they still wait. */
export const AutoAccepted: Story = {
  render: () => (
    <ShoulderTap
      agent={REPORT_TRIAGE}
      ask="I found four things worth doing. Which one first?"
      choices={[
        {
          title: "Label #1201",
          body: "A new bug report about the login page.",
          autoAccepted: { action: "Triage new issues" },
        },
        {
          title: "Label #1202",
          body: "A new feature request.",
          note: "Daily auto-accept limit reached",
        },
        {
          title: "Label #1203",
          body: "A new question.",
          note: "Waiting for your approval of check-in changes",
        },
        {
          title: "Label #1204",
          body: "A duplicate of #1180.",
          note: "The agent is busy; pick it when ready",
        },
      ]}
      onChoose={() => undefined}
      when="23:30"
    />
  ),
};

/** U3: the tool-call line opened: reads, a change, one that may have changed, one blocked. */
export const ToolCallsOpen: Story = {
  render: () => (
    <ShoulderTap
      agent={REPORT_TRIAGE}
      ask={ask}
      choices={REPORT_CHOICES}
      footer={<ToolCalls open />}
      onChoose={() => undefined}
      when="23:30"
    />
  ),
};

/** U4: a check-in that made changes and asks nothing; its list is open. */
export const Activity: Story = {
  render: () => (
    <Notice
      agent={REPORT_TRIAGE}
      body="1 call may have changed something."
      detail={<ToolCalls open />}
      title="The check-in made 1 change"
      when="23:30"
    />
  ),
};
