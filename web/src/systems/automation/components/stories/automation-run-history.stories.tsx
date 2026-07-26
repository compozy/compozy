import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { automationRunFixtures, automationRunSkipFixtures } from "@/systems/automation/mocks";

import { AutomationRunHistory } from "../automation-run-history";

const meta: Meta<typeof AutomationRunHistory> = {
  title: "systems/automation/components/AutomationRunHistory",
  component: AutomationRunHistory,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory error={null} isLoading={false} runs={automationRunFixtures} />
      </div>
    </CenteredSurface>
  ),
};

export const WholeRowLinkAffordance: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory error={null} isLoading={false} runs={automationRunFixtures} />
      </div>
    </CenteredSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const firstRun = automationRunFixtures.find(run => run.session_id);
    if (!firstRun) throw new Error("fixture lacks a row with a session_id");
    const row = await canvas.findByTestId(`automation-run-${firstRun.id}`);
    await expect(row.tagName).toBe("A");
    await expect(row).toHaveAttribute("href", `/session/${firstRun.session_id}`);
    const chevron = row.querySelector("svg");
    await expect(chevron).not.toBeNull();
    await expect(chevron).toHaveAttribute("aria-hidden", "true");
  },
};

/** Canceled runs the scheduler recorded as durable skips, each explaining why the fire was dropped. */
export const SkipReasons: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory error={null} isLoading={false} runs={automationRunSkipFixtures} />
      </div>
    </CenteredSurface>
  ),
};

export const Empty: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory error={null} isLoading={false} runs={[]} />
      </div>
    </CenteredSurface>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory error={null} isLoading runs={[]} />
      </div>
    </CenteredSurface>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-3xl">
        <AutomationRunHistory
          error={new globalThis.Error("Failed to load automation runs")}
          isLoading={false}
          runs={[]}
        />
      </div>
    </CenteredSurface>
  ),
};
