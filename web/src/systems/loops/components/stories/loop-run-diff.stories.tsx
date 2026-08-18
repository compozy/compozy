import type { Meta, StoryObj } from "@storybook/react-vite";

import { StoryTopbarHost } from "@/storybook/story-layout";

import { projectLoopDiff } from "../../lib/loop-run-diff-model";
import { emptyDiffFixture, generationDiffFixture, runDiffFixture } from "../../mocks";
import { LoopRunDiffPickers } from "../run-diff/loop-run-diff-pickers";
import { LoopRunDiffView } from "../run-diff/loop-run-diff-view";
import type { LoopDiff } from "../../types";

/** Comparison stories projected from daemon-shaped `LoopDiff` fixtures. */

const GENERATIONS = [3, 2, 1];
const RUNS = [
  { id: "looprun_release_train_fork", label: "release-train fork · gen 2" },
  { id: "looprun_release_train_done", label: "release-train · done" },
];

function DiffStory({
  diff,
  mode = "generation",
  error,
  isLoading,
}: {
  diff: LoopDiff | null;
  mode?: "generation" | "run";
  error?: string;
  isLoading?: boolean;
}) {
  return (
    <div className="flex h-dvh flex-col bg-canvas">
      <StoryTopbarHost title="Loops">
        <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
          <LoopRunDiffView
            error={error}
            isLoading={isLoading}
            pickers={
              <LoopRunDiffPickers
                againstGeneration={mode === "generation" ? 2 : null}
                againstRunId={mode === "run" ? RUNS[0].id : ""}
                baseGeneration={3}
                generations={GENERATIONS}
                mode={mode}
                onAgainstGenerationChange={() => {}}
                onAgainstRunChange={() => {}}
                onBaseGenerationChange={() => {}}
                onModeChange={() => {}}
                runs={RUNS}
              />
            }
            view={diff ? projectLoopDiff(diff) : null}
          />
        </div>
      </StoryTopbarHost>
    </div>
  );
}

const meta: Meta<typeof DiffStory> = {
  title: "systems/loops/components/LoopRunDiff",
  component: DiffStory,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Two generations of one run, grouped by change kind. */
export const GenerationCompare: Story = {
  args: {},
  render: () => <DiffStory diff={generationDiffFixture} />,
};

/** A run compare with inputs and a definition-divergence warning. */
export const RunCompare: Story = {
  args: {},
  render: () => <DiffStory diff={runDiffFixture} mode="run" />,
};

/** The honest empty state: the two sides read alike, so there is nothing to show. */
export const NoDifferences: Story = {
  args: {},
  render: () => <DiffStory diff={emptyDiffFixture} />,
};

/** A deterministic daemon refusal renders as information, not as a crash. */
export const CrossLoopRefusal: Story = {
  args: {},
  render: () => (
    <DiffStory diff={null} error="These runs belong to different Loops and cannot be compared." />
  ),
};

/** Nothing is chosen yet, so no comparison read has fired. */
export const AwaitingSelection: Story = {
  args: {},
  render: () => <DiffStory diff={null} />,
};

export const Loading: Story = {
  args: {},
  render: () => <DiffStory diff={null} isLoading />,
};
