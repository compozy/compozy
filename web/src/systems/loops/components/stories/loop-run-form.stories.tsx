import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fireEvent, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";

import { LoopRunForm } from "../run-form/loop-run-form";
import { loopDetailByName, loopEffectiveConfigFixture } from "../../mocks/fixtures";

const meta: Meta<typeof LoopRunForm> = {
  title: "systems/loops/components/LoopRunForm",
  component: LoopRunForm,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const deliveryLoop = loopDetailByName.get("software-delivery")!;
const watchLoop = loopDetailByName.get("reviews-watch")!;
const deliveryConfig = {
  iteration_cap: 3,
  budget_on_exceeded: "escalate" as const,
  no_progress_window: 2,
  fan_out_width: 4,
  gate_max_revisions: 2,
  reattempt_strategy: "full_body" as const,
  human_gate_enabled: true,
};

/** The hero run form: auto-generated typed inputs, Advanced overrides, live preview. */
export const Delivery: Story = {
  render: () => (
    <StorySurface className="flex h-[840px] flex-col p-0">
      <LoopRunForm
        workspaceId="ws_default"
        loop={deliveryLoop}
        effectiveConfig={{ ...loopEffectiveConfigFixture, ...deliveryConfig }}
      />
    </StorySurface>
  ),
};

/** The live preview reflects the explicit per-run cap without changing the saved baseline. */
export const DeliveryWithRunOverride: Story = {
  ...Delivery,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: /Advanced/ }));
    const capInput = canvas.getByTestId("loop-run-override-input-iteration_cap");
    await expect(capInput).toHaveAttribute("placeholder", "3");
    await fireEvent.change(capInput, { target: { value: "4" } });
    await expect(canvas.getByTestId("loop-run-overrides-badge")).toHaveTextContent("overrides set");
    await expect(canvas.getByTestId("loop-run-preview")).toHaveTextContent("4 generations");
    await expect(capInput).toHaveAttribute("placeholder", "3");
  },
};

/** A watch loop with no declared inputs — run it directly. */
export const NoInputs: Story = {
  render: () => (
    <StorySurface className="flex h-[840px] flex-col p-0">
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
    </StorySurface>
  ),
};
