import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { TasksEmptyState } from "../tasks-empty-state";

const meta: Meta<typeof TasksEmptyState> = {
  title: "systems/tasks/components/TasksEmptyState",
  component: TasksEmptyState,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <PanelSurface>
      <TasksEmptyState onSelectTemplate={() => undefined} workspaceName="Polybot" />
    </PanelSurface>
  ),
};

export const WithCopyCli: Story = {
  render: () => (
    <PanelSurface>
      <TasksEmptyState
        onCopyCli={() => undefined}
        onSelectTemplate={() => undefined}
        workspaceName="Polybot"
      />
    </PanelSurface>
  ),
};

export const NoWorkspace: Story = {
  render: () => (
    <PanelSurface>
      <TasksEmptyState onSelectTemplate={() => undefined} />
    </PanelSurface>
  ),
};
