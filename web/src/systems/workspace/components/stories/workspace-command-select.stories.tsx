import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { StorybookUserHomeDirSetup } from "@/storybook/route-story";
import { workspaceFixtures } from "@/systems/workspace/mocks";

import { WorkspaceCommandSelect } from "../workspace-command-select";

const DEFAULT_WORKSPACE_ID = workspaceFixtures[0]?.id ?? null;

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface className="w-[280px] border border-line bg-canvas-soft p-0">
      {children}
    </CenteredSurface>
  );
}

interface HarnessProps {
  defaultWorkspaceId?: string | null;
  defaultOpen?: boolean;
  userHomeDir?: string;
  onAddWorkspace?: () => void;
}

function WorkspaceCommandSelectHarness({
  defaultWorkspaceId = DEFAULT_WORKSPACE_ID,
  defaultOpen = false,
  userHomeDir,
  onAddWorkspace = fn(),
}: HarnessProps) {
  const [workspaceId, setWorkspaceId] = useState<string | null>(defaultWorkspaceId);
  const [open, setOpen] = useState(defaultOpen);

  return (
    <Frame>
      <StorybookUserHomeDirSetup userHomeDir={userHomeDir ?? null} />
      <WorkspaceCommandSelect
        workspaces={workspaceFixtures}
        value={workspaceId}
        onChange={setWorkspaceId}
        onAddWorkspace={onAddWorkspace}
        open={open}
        onOpenChange={setOpen}
      />
    </Frame>
  );
}

const meta: Meta<typeof WorkspaceCommandSelectHarness> = {
  title: "systems/workspace/components/WorkspaceCommandSelect",
  component: WorkspaceCommandSelectHarness,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Searchable workspace switcher for the sidebar header. Uses CommandSelect with avatar initials, active checkmarks, and an optional Add workspace action.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Open: Story = {
  args: {
    defaultOpen: true,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("workspace-command-input")).toBeInTheDocument();
    await expect(canvas.getByTestId("workspace-command-group")).toBeInTheDocument();
  },
};

export const HomeWorkspaceOpen: Story = {
  args: {
    defaultWorkspaceId: workspaceFixtures[3]?.id ?? workspaceFixtures[0]?.id ?? null,
    defaultOpen: true,
    userHomeDir: workspaceFixtures[3]?.root_dir,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const homeWorkspace = workspaceFixtures[3] ?? workspaceFixtures[0];
    if (!homeWorkspace) {
      throw new Error("Expected a workspace fixture");
    }

    await expect(canvas.getByTestId("workspace-switcher-avatar")).toHaveAttribute(
      "data-home",
      "true"
    );
    await expect(canvas.getByTestId(`workspace-command-item-${homeWorkspace.id}`)).toHaveAttribute(
      "data-home",
      "true"
    );
    await expect(canvas.getByText("Home workspace")).toBeInTheDocument();
  },
};

export const SwitchesWorkspace: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const target = workspaceFixtures[1];
    if (!target) {
      throw new Error("Expected at least two workspace fixtures");
    }

    await userEvent.click(canvas.getByTestId("workspace-switcher"));
    await userEvent.click(canvas.getByTestId(`workspace-command-item-${target.id}`));

    await waitFor(() =>
      expect(canvas.getByTestId("workspace-switcher-name")).toHaveTextContent(target.name)
    );
  },
};

export const EmptyRegistry: Story = {
  render: () => (
    <Frame>
      <WorkspaceCommandSelect workspaces={[]} value={null} onChange={() => undefined} />
    </Frame>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("workspace-switcher-name")).toHaveTextContent("No workspace");
    await expect(canvas.getByTestId("workspace-switcher")).toBeDisabled();
  },
};

export const Compact: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Compact trigger for scope rows: pill-group md track height, form-label typography, and field border beside Global/Workspace toggles.",
      },
    },
  },
  render: () => (
    <CenteredSurface className="items-start justify-center p-6">
      <div className="w-[220px] border border-line bg-canvas-soft p-3">
        <WorkspaceCommandSelect
          workspaces={workspaceFixtures}
          value={workspaceFixtures[0]?.id ?? null}
          onChange={() => undefined}
          size="compact"
          triggerTestId="workspace-compact-switcher"
          testIdPrefix="workspace-compact-command"
        />
      </div>
    </CenteredSurface>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("workspace-compact-switcher")).toBeInTheDocument();
    await expect(canvas.getByTestId("workspace-switcher-name")).toHaveTextContent(
      workspaceFixtures[0]?.name ?? "No workspace"
    );
  },
};
