import type { Meta, StoryObj } from "@storybook/react-vite";

import { createCodeHighlightWorker } from "../../../lib/code-highlight-worker";
import { CodeHighlightPoolProvider } from "../code-highlight-pool";
import { CodeViewer } from "../code-viewer";
import { longText, SAMPLE_GO } from "./files-fixtures";

const meta: Meta<typeof CodeViewer> = {
  title: "components/files/CodeViewer",
  component: CodeViewer,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Read-only, virtualized, highlighted file view over `@pierre/diffs` `CodeView`. `targetLine` scrolls into view and highlights; a target past the end clamps to the last line with a notice. The `header` and `notice` slots carry the caller's chrome and states.",
      },
    },
  },
  decorators: [
    Story => (
      <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
        <div className="flex h-[480px] flex-col">
          <Story />
        </div>
      </CodeHighlightPoolProvider>
    ),
  ],
  args: { className: "h-full" },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Header({ path, size }: { path: string; size: string }) {
  return (
    <div className="flex h-9 shrink-0 items-center gap-2 border-b border-line-soft px-3 text-small-body">
      <span className="truncate font-mono text-fg">{path}</span>
      <span className="text-muted-foreground">{size}</span>
    </div>
  );
}

export const Text: Story = {
  args: {
    path: "internal/projectfiles/reader.go",
    text: SAMPLE_GO,
    header: <Header path="internal/projectfiles/reader.go" size="1.9 KB" />,
  },
};

export const JumpedToLine: Story = {
  name: "Jumped to line",
  args: {
    path: "internal/projectfiles/reader.go",
    text: SAMPLE_GO,
    targetLine: 42,
    header: <Header path="internal/projectfiles/reader.go" size="1.9 KB" />,
  },
};

export const LinePastTheEnd: Story = {
  name: "Line past the end",
  args: {
    path: "internal/projectfiles/reader.go",
    text: SAMPLE_GO,
    targetLine: 420,
    header: <Header path="internal/projectfiles/reader.go" size="1.9 KB" />,
  },
};

export const Truncated: Story = {
  args: {
    path: "internal/generated/catalog.go",
    text: longText(SAMPLE_GO, 60),
    header: <Header path="internal/generated/catalog.go" size="3.4 MB" />,
    notice: (
      <p className="border-b border-line-soft px-3 py-1.5 text-xs text-muted-foreground">
        Showing the first 1 MB of 3.4 MB.
      </p>
    ),
  },
};
