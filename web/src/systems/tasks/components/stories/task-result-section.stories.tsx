import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { PanelSurface } from "@/storybook/story-layout";

import type { TaskResultPageController } from "../task-result-section";
import { TaskResultSection } from "../task-result-section";

const meta: Meta<typeof TaskResultSection> = {
  title: "systems/tasks/components/TaskResultSection",
  component: TaskResultSection,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Bounded task-result disclosure for inline and externally stored run output.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const externalPage = JSON.stringify(
  {
    completed: true,
    files: Array.from({ length: 24 }, (_, index) => `internal/loop/result-${index + 1}.go`),
    summary: "The first 16 KiB page stays inside a fixed result viewport.",
  },
  null,
  2
);

function ExternalResultStory() {
  const [open, setOpen] = useState(false);
  const controller: TaskResultPageController = {
    canGoNext: true,
    canGoPrevious: false,
    copyState: "idle",
    errorMessage: null,
    isLoading: false,
    onCopy: () => Promise.resolve(),
    onNextPage: () => undefined,
    onOpenChange: setOpen,
    onPreviousPage: () => undefined,
    onRetry: () => undefined,
    open,
    page: {
      run_id: "run_large_result",
      result_ref: "sha256:large-result",
      offset: 0,
      bytes: 16_384,
      total_bytes: 327_680,
      data_base64: "",
      next_offset: 16_384,
      eof: false,
    },
    pageText: externalPage,
  };
  return (
    <PanelSurface className="w-[min(46rem,calc(100vw-2rem))] p-6">
      <TaskResultSection
        emptyMessage="No result recorded."
        external={controller}
        result={null}
        resultBytes={327_680}
        resultRef="sha256:large-result"
      />
    </PanelSurface>
  );
}

/** Externally stored result stays closed until requested and renders one byte page at a time. */
export const ExternalLargeResult: Story = {
  args: {
    emptyMessage: "No result recorded.",
    result: null,
  },
  render: () => <ExternalResultStory />,
};

/** Inline result keeps a clamped first read with an explicit full-view disclosure. */
export const InlineResult: Story = {
  args: {
    emptyMessage: "No result recorded.",
    result: {
      completed: true,
      summary: "Inline output remains bounded without another network read.",
      validations: ["Go tests passed", "Web typecheck passed", "Result stored"],
    },
  },
  render: args => (
    <PanelSurface className="w-[min(46rem,calc(100vw-2rem))] p-6">
      <TaskResultSection {...args} />
    </PanelSurface>
  ),
};
