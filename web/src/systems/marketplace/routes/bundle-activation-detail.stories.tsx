import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/routes/BundleActivationDetail",
  component: StorybookRouteCanvas,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Current: Story = {
  parameters: appRouteParameters("/marketplace/bundles/activations/activation-ops-starter"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Drifted: Story = {
  parameters: appRouteParameters("/marketplace/bundles/activations/activation-dep-kit"),
  render: () => <StorybookWorkspaceSetup />,
};

export const DeactivateOpen: Story = {
  parameters: appRouteParameters("/marketplace/bundles/activations/activation-ops-starter"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Actions for ops-starter" }));
    await userEvent.click(
      await within(document.body).findByRole("menuitem", { name: "Deactivate…" })
    );
    await expect(
      within(document.body).findByTestId("deactivate-bundle-dialog")
    ).resolves.toBeDefined();
  },
};
