import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { workspaceFixtures } from "@/systems/workspace/mocks";

import { WorkspaceCommandSelect } from "../workspace-command-select";

const DEFAULT_WORKSPACE_ID = workspaceFixtures[0]?.id ?? null;

function Frame({ children, open = false }: { children: React.ReactNode; open?: boolean }) {
  return (
    <CenteredSurface className={open ? "items-start" : undefined}>
      <div className="w-[280px] border border-line bg-canvas-soft">{children}</div>
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
    <Frame open={defaultOpen}>
      <WorkspaceCommandSelect
        workspaces={workspaceFixtures}
        value={workspaceId}
        onChange={setWorkspaceId}
        onAddWorkspace={onAddWorkspace}
        open={open}
        onOpenChange={setOpen}
        userHomeDir={userHomeDir}
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

/** Default closed workspace switcher. */
export const Default: Story = { args: {} };

/** Open switcher with searchable project workspaces. */
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

/** Operator-home registrations never appear as project choices. */
export const HidesHomeRegistration: Story = {
  args: {
    defaultOpen: true,
    userHomeDir: workspaceFixtures[3]?.root_dir,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const homeWorkspace = workspaceFixtures[3];
    if (!homeWorkspace) {
      throw new Error("Expected a home workspace fixture");
    }

    await expect(
      canvas.queryByTestId(`workspace-command-item-${homeWorkspace.id}`)
    ).not.toBeInTheDocument();
    await expect(canvas.queryByText("Home workspace")).not.toBeInTheDocument();
  },
};

/** Selecting a row updates the trigger and closes the list. */
export const SwitchesWorkspace: Story = {
  args: {},
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

/** Empty registries expose a disabled, truthful trigger. */
export const EmptyRegistry: Story = {
  args: {},
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

/** Compact trigger for dense form rows. */
export const Compact: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story: "Compact trigger for dense rows: pill-group md track height and form-label type.",
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
