import type { Meta, StoryObj } from "@storybook/react-vite";

import { RELEASE_TRAIN_LOOP_NAME, releaseTrainDetail, releaseTrainRun } from "../../mocks";
import { LoopForkDialog } from "../run-page/loop-fork-dialog";

const GENERATIONS = [3, 2, 1];
const SOURCE_INPUTS = releaseTrainRun.inputs ?? {};

function ForkStory(props: {
  fieldErrors?: Readonly<Record<string, string>>;
  blockedReason?: string;
  isPending?: boolean;
}) {
  return (
    <div className="min-h-dvh bg-canvas">
      <LoopForkDialog
        defaultGeneration={2}
        generations={GENERATIONS}
        inputSchema={releaseTrainDetail.definition.inputs}
        loopName={RELEASE_TRAIN_LOOP_NAME}
        onOpenChange={() => {}}
        onSubmit={() => {}}
        open
        sourceInputs={SOURCE_INPUTS}
        {...props}
      />
    </div>
  );
}

const meta: Meta<typeof ForkStory> = {
  title: "systems/loops/components/LoopForkDialog",
  component: ForkStory,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => <ForkStory />,
};

export const ValidationError: Story = {
  args: {},
  render: () => <ForkStory fieldErrors={{ services: "At least one service is required." }} />,
};

export const BlockedGeneration: Story = {
  args: {},
  render: () => <ForkStory blockedReason="Generation 5 does not exist on this run." />,
};

export const Starting: Story = {
  args: {},
  render: () => <ForkStory isPending />,
};
