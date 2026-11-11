import type { Meta, StoryObj } from "@storybook/react-vite";

import { storyWorkspaceIds } from "@/storybook/fintech-scenario";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/files/routes/Files",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Real-shell stories for the Files OS app routes: the bare `/files` launch, which binds to the active project, and a window bound to one project.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** `/files` with no project in the path: the window binds to the active project. */
export const ActiveProject: Story = {
  args: {},
  parameters: appRouteParameters("/files"),
  render: () => <StorybookWorkspaceSetup />,
};

/** `/files/<workspace id>`: the window shows that project's root. */
export const Project: Story = {
  args: {},
  parameters: appRouteParameters(`/files/${storyWorkspaceIds.hq}`),
  render: () => <StorybookWorkspaceSetup />,
};
