import type { Meta, StoryObj } from "@storybook/react-vite";
import * as React from "react";
import { expect, fn, waitFor } from "storybook/test";

import { DropdownMenuItem, DropdownMenuSeparator } from "../../dropdown-menu";
import { Empty } from "../../empty";
import { FileTree, type FileTreeDecorations } from "../file-tree";
import { CHANGED_PATHS, CHILD_LISTINGS, ROOT_LISTING } from "./files-fixtures";

const meta: Meta<typeof FileTree> = {
  title: "components/files/FileTree",
  component: FileTree,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Virtualized, keyboard-driven file tree over `@pierre/trees`. Paths are flat (`dir/` for directories); a lazy explorer starts closed and feeds children through `paths` after `onExpand`. Decorations carry git status, `+N −M`, sensitive, and symlink marks; unknown status renders no badge.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="h-[420px] w-[300px] bg-sunken p-1">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const LISTING_DELAY_MS = 450;

function treeRow(canvasElement: HTMLElement, path: string): HTMLElement | null {
  const root = canvasElement.querySelector("file-tree-container")?.shadowRoot;
  return root?.querySelector<HTMLElement>(`[data-type="item"][data-item-path="${path}"]`) ?? null;
}

const explorerDecorations: FileTreeDecorations = {
  "node_modules/": { status: "ignored" },
  ".env": { sensitive: true },
  "go.mod": { status: "modified" },
  "internal/projectfiles/reader.go": { status: "modified" },
  "internal/projectfiles/status.go": { status: "untracked" },
  "internal/api/files.go": { status: "added" },
  "web/src/main.tsx": { symlink: true },
  // Unloaded until expanded; the daemon's listing says something below it changed.
  "web/": { containsChanges: true },
};

function LazyExplorer({
  onExpand,
  onSelect,
}: {
  onExpand?: (dir: string) => void;
  onSelect?: (path: string) => void;
}) {
  const [paths, setPaths] = React.useState<string[]>(ROOT_LISTING);
  const [loading, setLoading] = React.useState<string[]>([]);
  const [selected, setSelected] = React.useState<string | null>(null);
  return (
    <FileTree
      aria-label="Project files"
      className="h-full"
      paths={paths}
      loadingPaths={loading}
      decorations={explorerDecorations}
      selectedPath={selected}
      onSelect={path => {
        setSelected(path);
        onSelect?.(path);
      }}
      onExpand={dir => {
        onExpand?.(dir);
        setLoading(current => [...current, dir]);
        window.setTimeout(() => {
          setPaths(current => [...current, ...(CHILD_LISTINGS[dir] ?? [])]);
          setLoading(current => current.filter(path => path !== dir));
        }, LISTING_DELAY_MS);
      }}
    />
  );
}

export const LazyExplorerTree: Story = {
  name: "Lazy explorer",
  tags: ["play-fn"],
  args: { onExpand: fn(), onSelect: fn() },
  render: args => <LazyExplorer onExpand={args.onExpand} onSelect={args.onSelect} />,
  parameters: {
    docs: {
      description: {
        story:
          "Closed directories load on first expand: the row shows a spinner while its listing is in flight, ignored folders dim, `.env` carries the sensitive lock, and changed rows carry badges with a dot on their ancestors.",
      },
    },
  },
  play: async ({ args, canvasElement }) => {
    await waitFor(() => expect(treeRow(canvasElement, "internal/")).not.toBeNull());
    treeRow(canvasElement, "internal/")?.click();
    await waitFor(() => expect(args.onExpand).toHaveBeenCalledWith("internal/"));
    await waitFor(() => expect(treeRow(canvasElement, "internal/projectfiles/")).not.toBeNull());
    treeRow(canvasElement, "internal/")?.click();
    treeRow(canvasElement, "internal/")?.click();
    await expect(args.onExpand).toHaveBeenCalledTimes(1);
  },
};

const changeDecorations: FileTreeDecorations = {
  "internal/projectfiles/reader.go": { status: "modified", additions: 42, deletions: 7 },
  "internal/projectfiles/roots.go": { status: "conflicted", additions: 3, deletions: 3 },
  "internal/api/files.go": { status: "added", additions: 128, deletions: 0 },
  "web/src/main.tsx": { status: "renamed", additions: 1, deletions: 1 },
  "docs/old.md": { status: "deleted", additions: 0, deletions: 64 },
};

export const ChangedFiles: Story = {
  name: "Changed files",
  args: {
    "aria-label": "Changed files",
    paths: CHANGED_PATHS,
    initialExpansion: "open",
    decorations: changeDecorations,
    selectedPath: "internal/projectfiles/reader.go",
  },
  parameters: {
    docs: {
      description: {
        story:
          "The Changes sidebar shape: every changed path up front, expanded, with per-file `+N −M`, a destructive `C` for conflicts, and status badges.",
      },
    },
  },
};

export const FilterWithoutMatches: Story = {
  name: "Filter without matches",
  args: {
    "aria-label": "Project files",
    paths: CHANGED_PATHS,
    initialExpansion: "open",
    filter: "zzz",
    filterEmpty: (
      <p className="px-3 py-2 text-small-body text-muted-foreground">No loaded files match</p>
    ),
  },
};

export const EmptyFolder: Story = {
  name: "Empty folder",
  args: {
    "aria-label": "Project files",
    paths: [],
    empty: <Empty size="compact" title="This folder is empty" />,
  },
};

export const RowMenu: Story = {
  name: "Row menu",
  tags: ["play-fn"],
  args: {
    "aria-label": "Project files",
    paths: CHANGED_PATHS,
    initialExpansion: "open",
    rowMenu: (item, { close }) => (
      <>
        <DropdownMenuItem onClick={close}>Copy path</DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={close}>Open in Cursor</DropdownMenuItem>
        <DropdownMenuItem onClick={close}>
          {item.kind === "directory" ? "Reveal folder in Finder" : "Reveal in Finder"}
        </DropdownMenuItem>
      </>
    ),
  },
  parameters: {
    docs: {
      description: {
        story:
          "Right-click a row, or press Shift+F10 on a focused row, to open its menu. The consumer returns `DropdownMenu` items; the tree anchors them at the row.",
      },
    },
  },
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(treeRow(canvasElement, "docs/old.md")).not.toBeNull());
    const row = treeRow(canvasElement, "docs/old.md");
    const box = row?.getBoundingClientRect();
    row?.dispatchEvent(
      new MouseEvent("contextmenu", {
        bubbles: true,
        composed: true,
        cancelable: true,
        clientX: (box?.left ?? 0) + 24,
        clientY: (box?.top ?? 0) + (box?.height ?? 0) / 2,
      })
    );
    await waitFor(() =>
      expect(canvasElement.ownerDocument.querySelector('[role="menu"]')).not.toBeNull()
    );
  },
};
