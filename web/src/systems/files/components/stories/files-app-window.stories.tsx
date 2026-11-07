import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";

import { Topbar, TopbarSlotProvider } from "@compozy/ui";

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

/** The Files window as the OS frame renders it: the shared head row over the app body. */
function FilesWindowFrame({ width }: { width: number }) {
  return (
    <TopbarSlotProvider>
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
 * The Files OS app window (`_uiux.md` S6). One head row: the project and root pickers, then the
 * window title, which is the open file's name, else the project's name.
 */
const meta = {
  title: "systems/files/FilesAppWindow",
  component: FilesWindowFrame,
  parameters: { layout: "centered" },
  args: { width: 960 },
} satisfies Meta<typeof FilesWindowFrame>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ProjectTitle: Story = {
  beforeEach: () => placeFile(null),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() =>
      expect(canvas.getByTestId("topbar-title-text")).toHaveTextContent(storyWorkspaceNames.hq)
    );
  },
};

export const OpenFileTitle: Story = {
  beforeEach: () => placeFile("web/src/main.tsx"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() =>
      expect(canvas.getByTestId("topbar-title-text")).toHaveTextContent("main.tsx")
    );
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
