import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";

import { Topbar, TopbarSlotProvider, useTopbarSlotValue } from "@compozy/ui";

import { storyWorkspaceIds, storyWorkspaceNames } from "@/storybook/fintech-scenario";

import { filesAppPlaceStore } from "../../hooks/use-files-app-place";
import { FilesAppWindow } from "../files-app-window";

const WORKSPACE_ID = storyWorkspaceIds.hq;

function placeFile(path: string | null) {
  filesAppPlaceStore.trigger.placed({
    workspaceId: WORKSPACE_ID,
    place: { worktreeId: null, path },
  });
}

/** Stands in for the OS tab strip: the only place that names the open file. */
function FilesWindowTab() {
  const label = useTopbarSlotValue()?.tabLabel ?? "Files";
  return (
    <div
      data-testid="files-story-tab"
      className="truncate px-4 py-2 text-body font-medium text-muted-foreground"
    >
      {label}
    </div>
  );
}

/** The Files window as the OS frame renders it: its tab, then the shared head row over the app body. */
function FilesWindowFrame({ width }: { width: number }) {
  return (
    <TopbarSlotProvider>
      <FilesWindowTab />
      <div
        className="flex h-[560px] flex-col overflow-hidden rounded-lg border border-line bg-canvas"
        style={{ width }}
      >
        <Topbar title="Files" />
        <div className="min-h-0 flex-1">
          <FilesAppWindow workspaceId={WORKSPACE_ID} onSwitchProject={() => {}} />
        </div>
      </div>
    </TopbarSlotProvider>
  );
}

/**
 * The Files OS app window (`_uiux.md` S6). One head row: the Files identity, then the project and
 * root pickers. Only the tab names the open file, else the project.
 */
const meta = {
  title: "systems/files/components/FilesAppWindow",
  component: FilesWindowFrame,
  parameters: { layout: "centered" },
  args: { width: 960 },
} satisfies Meta<typeof FilesWindowFrame>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ProjectTitle: Story = {
  tags: ["play-fn"],
  beforeEach: () => placeFile(null),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() =>
      expect(canvas.getByTestId("files-story-tab")).toHaveTextContent(storyWorkspaceNames.hq)
    );
    expect(canvas.getByTestId("topbar-title-text")).toHaveTextContent("Files");
    const context = canvasElement.querySelector('[data-slot="topbar-context"]');
    expect(context?.querySelector('[data-testid="files-project-switcher"]')).not.toBeNull();
  },
};

export const OpenFileTitle: Story = {
  tags: ["play-fn"],
  beforeEach: () => placeFile("web/src/main.tsx"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() =>
      expect(canvas.getByTestId("files-story-tab")).toHaveTextContent("main.tsx")
    );
    expect(canvas.getByTestId("topbar-title-text")).toHaveTextContent("Files");
  },
};

export const OpenFileTitleLight: Story = {
  ...OpenFileTitle,
  globals: { theme: "light" },
};

export const Narrow: Story = {
  args: { width: 460 },
  beforeEach: () => placeFile("web/src/main.tsx"),
};

export const NarrowLight: Story = { ...Narrow, globals: { theme: "light" } };

/** #1605: Markdown opens rendered, with Source / Preview / Split in the viewer header. */
export const MarkdownPreview: Story = {
  tags: ["play-fn"],
  beforeEach: () => placeFile("docs/guide.md"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const preview = await canvas.findByTestId("file-viewer-markdown-preview");
    await waitFor(() =>
      expect(within(preview).getByRole("heading", { name: "File explorer" })).toBeVisible()
    );
    expect(preview.querySelector("table")).not.toBeNull();
    expect(canvas.getByTestId("file-viewer-mode-preview")).toHaveAttribute("aria-pressed", "true");
  },
};

export const MarkdownSplit: Story = {
  tags: ["play-fn"],
  beforeEach: () => placeFile("docs/guide.md"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    (await canvas.findByTestId("file-viewer-mode-split")).click();
    await waitFor(() => expect(canvas.getByTestId("file-viewer-split")).toBeVisible());
    expect(canvas.getByTestId("file-viewer-text")).toBeVisible();
    (await canvas.findByTestId("file-viewer-mode-preview")).click();
  },
};

export const CsvPreview: Story = {
  beforeEach: () => placeFile("docs/people.csv"),
};

export const LargeCsvPreview: Story = {
  beforeEach: () => placeFile("docs/metrics.csv"),
};

export const PdfPreview: Story = {
  tags: ["play-fn"],
  beforeEach: () => placeFile("docs/report.pdf"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const pdf = await canvas.findByTestId("file-viewer-pdf", undefined, { timeout: 10_000 });
    await waitFor(() => expect(pdf).toHaveAttribute("data-page-count", "2"), { timeout: 10_000 });
  },
};

export const HtmlPreview: Story = {
  beforeEach: () => placeFile("docs/page.html"),
};

export const NotebookPreview: Story = {
  beforeEach: () => placeFile("docs/analysis.ipynb"),
};

export const MarkdownPreviewLight: Story = {
  ...MarkdownPreview,
  globals: { theme: "light" },
};
