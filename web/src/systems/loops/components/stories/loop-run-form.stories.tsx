import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { X } from "lucide-react";

import { Button } from "@compozy/ui";
import { expect, fireEvent, userEvent, within } from "storybook/test";

import { LoopRunForm } from "../run-form/loop-run-form";
import {
  loopDetailByName,
  loopEffectiveConfigFixture,
  loopRunFixtures,
} from "../../mocks/fixtures";
import { LoopsStoryShell } from "./loops-story-shell";

const meta: Meta<typeof LoopRunForm> = {
  title: "systems/loops/components/LoopRunForm",
  component: LoopRunForm,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const implementTasksLoop = loopDetailByName.get("implement-tasks")!;
const watchLoop = loopDetailByName.get("review-and-fix")!;
const liveRun = loopRunFixtures.find(
  run => run.loop_name === "implement-tasks" && run.status === "running"
)!;
const implementTasksConfig = {
  iteration_cap: 3,
  budget_on_exceeded: "escalate" as const,
  no_progress_window: 2,
  fan_out_width: 4,
  gate_max_revisions: 2,
  reattempt_strategy: "full_body" as const,
  human_gate_enabled: true,
};

/** The run form inside its product window chrome — the crumbs its route location publishes. */
function RunFormShell({ children }: { children: ReactNode }) {
  return (
    <LoopsStoryShell
      actions={
        <Button data-testid="loop-run-form-close" size="sm" type="button" variant="ghost">
          <X aria-hidden="true" className="size-3.5" />
          Close
        </Button>
      }
      crumb="Run"
      crumbs={[
        { id: "loops", label: "Loops", onSelect: () => {} },
        { id: "loop", label: "implement-tasks", onSelect: () => {} },
      ]}
      onBack={() => {}}
    >
      {children}
    </LoopsStoryShell>
  );
}

/** The hero run form: auto-generated typed inputs and per-run Limits overrides. */
export const ImplementTasks: Story = {
  render: () => (
    <RunFormShell>
      <LoopRunForm
        workspaceId="ws_default"
        loop={implementTasksLoop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...implementTasksConfig }}
      />
    </RunFormShell>
  ),
};

/** An explicit per-run cap marks overrides without changing the saved baseline. */
export const ImplementTasksWithRunOverride: Story = {
  ...ImplementTasks,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /Limits/ }));
    const capInput = canvas.getByTestId("loop-run-override-input-iteration_cap");
    await expect(capInput).toHaveAttribute("placeholder", "3");
    await fireEvent.change(capInput, { target: { value: "4" } });
    await expect(canvas.getByTestId("loop-run-overrides-badge")).toHaveTextContent("overrides set");
    await expect(capInput).toHaveAttribute("placeholder", "3");
  },
};

/**
 * The same loop, arrived at while one of its runs is already live: the open run is
 * stated with a link to its page, and no control is duplicated here (US-029 EC-6).
 */
export const AlreadyRunning: Story = {
  render: () => (
    <RunFormShell>
      <LoopRunForm
        activeRun={liveRun}
        workspaceId="ws_default"
        loop={implementTasksLoop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...implementTasksConfig }}
      />
    </RunFormShell>
  ),
};

/** A watch loop with no declared inputs — run it directly. */
export const NoInputs: Story = {
  render: () => (
    <RunFormShell>
      <LoopRunForm
        workspaceId="ws_default"
        loop={watchLoop}
        effectiveConfig={{
          ...loopEffectiveConfigFixture,
          fan_out_width: 2,
          gate_max_revisions: 0,
          iteration_cap: 0,
        }}
      />
    </RunFormShell>
  ),
};
