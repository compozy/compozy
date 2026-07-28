import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
  storyWorkspaceIds,
  storyWorkspaceNames,
  storyWorkspacePaths,
} from "@/storybook/fintech-scenario";
import { CenteredSurface } from "@/storybook/story-layout";
import { StorybookUserHomeDirSetup } from "@/storybook/route-story";

import { ScopeSelector, type ScopeSelectorScope } from "../scope-selector";

const storyWorkspaces = [
  { id: storyWorkspaceIds.hq, name: storyWorkspaceNames.hq, root_dir: storyWorkspacePaths.hq },
  {
    id: storyWorkspaceIds.risk,
    name: storyWorkspaceNames.risk,
    root_dir: storyWorkspacePaths.risk,
  },
  {
    id: storyWorkspaceIds.growth,
    name: storyWorkspaceNames.growth,
    root_dir: storyWorkspacePaths.growth,
  },
];

interface ScopeSelectorHarnessProps {
  initialScope?: ScopeSelectorScope;
  initialWorkspaceId?: string | null;
  workspaceDisabled?: boolean;
  emptyRegistry?: boolean;
  userHomeDir?: string | null;
}

function ScopeSelectorHarness({
  initialScope = "workspace",
  initialWorkspaceId = storyWorkspaceIds.hq,
  workspaceDisabled = false,
  emptyRegistry = false,
  userHomeDir = null,
}: ScopeSelectorHarnessProps) {
  const [scope, setScope] = useState<ScopeSelectorScope>(initialScope);
  const [workspaceId, setWorkspaceId] = useState<string | null>(initialWorkspaceId);

  return (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-full max-w-[520px] border border-line bg-canvas-soft p-4">
        <StorybookUserHomeDirSetup userHomeDir={userHomeDir} />
        <ScopeSelector
          scope={scope}
          workspaceId={workspaceId}
          workspaces={emptyRegistry ? [] : storyWorkspaces}
          onScopeChange={setScope}
          onWorkspaceChange={setWorkspaceId}
          testIdPrefix="scope"
          workspaceDisabled={workspaceDisabled}
        />
      </div>
    </CenteredSurface>
  );
}

const meta: Meta<typeof ScopeSelectorHarness> = {
  title: "systems/workspace/components/ScopeSelector",
  component: ScopeSelectorHarness,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Shared global/workspace scope control for task, job, and trigger creation surfaces. Workspace mode uses the compact workspace command selector aligned to PillGroup md height.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Workspace: Story = {};

export const Global: Story = {
  args: {
    initialScope: "global",
    initialWorkspaceId: null,
  },
};

export const SwitchesWorkspace: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("scope-workspace-select"));
    await userEvent.click(canvas.getByTestId("scope-workspace-item-" + storyWorkspaceIds.risk));

    await waitFor(() =>
      expect(canvas.getByTestId("workspace-switcher-name")).toHaveTextContent(
        storyWorkspaceNames.risk
      )
    );
  },
};

export const EmptyRegistry: Story = {
  args: {
    emptyRegistry: true,
  },
};

export const WorkspaceDisabled: Story = {
  args: {
    initialScope: "global",
    initialWorkspaceId: null,
    workspaceDisabled: true,
  },
};

export const HomeWorkspace: Story = {
  args: {
    initialWorkspaceId: storyWorkspaceIds.risk,
    userHomeDir: storyWorkspacePaths.risk,
  },
  parameters: {
    docs: {
      description: {
        story:
          "Compact workspace selector inside scope rows pins the home workspace first and shows the home glyph when its root_dir matches the daemon user_home_dir.",
      },
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("scope-workspace-select"));
    await expect(
      canvas.getByTestId("scope-workspace-item-" + storyWorkspaceIds.risk)
    ).toHaveAttribute("data-home", "true");
    await expect(canvas.getByText("Home workspace")).toBeInTheDocument();
  },
};

export const HomeWorkspacePromotesGlobal: Story = {
  args: {
    initialScope: "workspace",
    initialWorkspaceId: storyWorkspaceIds.hq,
    userHomeDir: storyWorkspacePaths.risk,
  },
  parameters: {
    docs: {
      description: {
        story:
          "Selecting the home workspace while scope is Workspace automatically switches the toggle to Global.",
      },
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("scope-workspace-select"));
    await userEvent.click(canvas.getByTestId("scope-workspace-item-" + storyWorkspaceIds.risk));

    await waitFor(() =>
      expect(canvas.getByTestId("scope-scope-global")).toHaveAttribute("aria-pressed", "true")
    );
    expect(canvas.queryByTestId("scope-workspace-select")).not.toBeInTheDocument();
  },
};
