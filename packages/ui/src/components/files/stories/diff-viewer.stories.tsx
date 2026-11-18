import type { Meta, StoryObj } from "@storybook/react-vite";

import { createCodeHighlightWorker } from "../../../lib/code-highlight-worker";
import { Pill } from "../../custom/pill";
import { CodeHighlightPoolProvider } from "../code-highlight-pool";
import { DiffViewer, type DiffViewerItem } from "../diff-viewer";
import { OLD_READER, PATCHES, SAMPLE_GO } from "./files-fixtures";

const PATCH_DELAY_MS = 300;

const items: DiffViewerItem[] = [
  { id: "internal/projectfiles/reader.go", version: "r1", path: "internal/projectfiles/reader.go" },
  { id: "internal/projectfiles/roots.go", version: "r1", path: "internal/projectfiles/roots.go" },
  {
    id: "web/src/main.tsx",
    version: "r1",
    path: "web/src/main.tsx",
    prevPath: "web/src/index.tsx",
    changeType: "rename-changed",
  },
  {
    id: "web/public/logo.png",
    version: "r1",
    path: "web/public/logo.png",
    changeType: "new",
    loadable: false,
  },
];

function loadPatch(item: DiffViewerItem): Promise<string | null> {
  return new Promise(resolve => {
    window.setTimeout(() => resolve(PATCHES[item.path] ?? null), PATCH_DELAY_MS);
  });
}

async function loadContents(item: DiffViewerItem) {
  if (item.path === "internal/projectfiles/reader.go") {
    return { oldText: OLD_READER, newText: SAMPLE_GO };
  }
  throw new Error("contents unavailable");
}

function fileHeader(item: DiffViewerItem, state: { status: string }) {
  const slash = item.path.lastIndexOf("/");
  return (
    <div className="flex h-9 items-center gap-2 border-b border-line-soft bg-canvas-soft px-3 text-small-body">
      <span className="truncate font-mono">
        <span className="text-muted-foreground">{item.path.slice(0, slash + 1)}</span>
        <span className="text-fg">{item.path.slice(slash + 1)}</span>
      </span>
      {state.status === "unavailable" ? <Pill size="sm">Binary</Pill> : null}
      {state.status === "pending" ? (
        <span className="text-xs text-muted-foreground">Loading…</span>
      ) : null}
    </div>
  );
}

const meta: Meta<typeof DiffViewer> = {
  title: "components/files/DiffViewer",
  component: DiffViewer,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Virtualized multi-file diff over `@pierre/diffs` `CodeView`. Patches load per file as it nears the viewport; `version` bumps refetch; `loadable: false` renders header-only; `loadContents` enables expanding unchanged context, and `expandable: false` turns it off for one item.",
      },
    },
  },
  decorators: [
    Story => (
      <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
        <div className="flex h-[560px] flex-col">
          <Story />
        </div>
      </CodeHighlightPoolProvider>
    ),
  ],
  args: { className: "h-full", items, loadPatch, loadContents, fileHeader },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Unified: Story = { args: { mode: "unified" } };

export const Split: Story = { args: { mode: "split" } };

export const Collapsed: Story = {
  name: "Collapse all",
  args: { mode: "unified", collapsed: true },
};

export const NotExpandable: Story = {
  name: "Context not expandable",
  args: {
    mode: "unified",
    items: [{ ...items[0]!, expandable: false }, items[1]!],
  },
  parameters: {
    docs: {
      description: {
        story:
          "`expandable: false` keeps one file's unchanged context collapsed (for example contents over the expand cap); the header slot says why.",
      },
    },
  },
};

export const OneFileAtATime: Story = {
  name: "One file at a time",
  args: { mode: "unified", items: items.slice(0, 1) },
  parameters: {
    docs: {
      description: {
        story: "Large-diff mode renders a single item; the caller steps between files.",
      },
    },
  },
};
