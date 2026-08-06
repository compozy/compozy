import type { Meta, StoryObj } from "@storybook/react-vite";

import { StorySurface } from "@/storybook/story-layout";

import { LoopConfigureDialog } from "../configure/loop-configure-dialog";
import {
  loopConfigFixture,
  loopDetailByName,
  loopEffectiveConfigFixture,
} from "../../mocks/fixtures";

const meta: Meta<typeof LoopConfigureDialog> = {
  title: "systems/loops/components/LoopConfigureDialog",
  component: LoopConfigureDialog,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const implementTasksLoop = loopDetailByName.get("implement-tasks")!;
const watchLoop = loopDetailByName.get("review-and-fix")!;

const noop = () => {};

/** The full configure modal for the bundled implement-tasks Loop. */
export const ImplementTasks: Story = {
  render: () => (
    <StorySurface className="h-[880px] p-0">
      <LoopConfigureDialog
        open
        workspaceId="ws_default"
        loop={implementTasksLoop}
        config={loopConfigFixture}
        effectiveConfig={loopEffectiveConfigFixture}
        onOpenChange={noop}
        onOpenEditor={noop}
      />
    </StorySurface>
  ),
};

/** No stored config — the sheet opens on the inherited loop defaults (all checks on, human
 *  gate off, failed-only, limits placeholder-only). */
export const InheritedDefaults: Story = {
  render: () => (
    <StorySurface className="h-[880px] p-0">
      <LoopConfigureDialog
        open
        workspaceId="ws_default"
        loop={implementTasksLoop}
        config={null}
        effectiveConfig={loopEffectiveConfigFixture}
        onOpenChange={noop}
        onOpenEditor={noop}
      />
    </StorySurface>
  ),
};

/** A watch loop that declares no verification checks — the Review gate group is empty. */
export const WatchNoChecks: Story = {
  render: () => (
    <StorySurface className="h-[880px] p-0">
      <LoopConfigureDialog
        open
        workspaceId="ws_default"
        loop={watchLoop}
        config={null}
        effectiveConfig={{
          ...loopEffectiveConfigFixture,
          fan_out_width: 2,
          gate_max_revisions: 0,
          iteration_cap: 0,
        }}
        onOpenChange={noop}
        onOpenEditor={noop}
      />
    </StorySurface>
  ),
};
