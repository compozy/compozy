import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/marketplace/components/Overlays",
  component: StorybookRouteCanvas,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Add ▾ → local build opens the production install dialog on the local-path source. */
export const ExtensionUnionInstall: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-local"));
    await expect(body.findByTestId("extension-install-dialog")).resolves.toBeDefined();
    await expect(
      body.findByRole("button", { name: "About directory path" })
    ).resolves.toBeDefined();
  },
};

/** Add ▾ → GitHub preselects the release source with its version and asset selectors. */
export const ExtensionUnionInstallGitHub: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-github"));
    await expect(body.findByTestId("extension-install-asset")).resolves.toBeDefined();
  },
};

/** An unverified catalog entry installs through the daemon's explicit trust confirmation. */
export const ExtensionWarning: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install slack-notify" }));
    const dialog = within(document.body);
    await expect(dialog.findByText("Unsigned package")).resolves.toBeDefined();
    await expect(dialog.findByText("Network egress")).resolves.toBeDefined();
  },
};
