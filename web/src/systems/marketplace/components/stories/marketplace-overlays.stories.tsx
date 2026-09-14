import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  handlers,
  marketplaceSourcePreviewFixture,
  marketplaceSourceHandlers,
  marketplaceSourceFixtures,
} from "../../mocks";
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

/** Add ▾ → plugin marketplace opens the shared dialog empty: one field, primary disabled. */
export const AddMarketplaceEmpty: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await expect(body.findByTestId("add-marketplace-dialog")).resolves.toBeDefined();
    await expect(body.findByTestId("add-marketplace-submit")).resolves.toBeDisabled();
  },
};

/** Leaving the field runs the dry run; the success notice restates the identity and enables Add. */
export const AddMarketplaceFound: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...storybookMswParameters({
      marketplace: [...marketplaceSourceHandlers([marketplaceSourceFixtures.feed]), ...handlers],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(
      await body.findByTestId("add-marketplace-ref"),
      "anthropics/claude-plugins-official-mirror"
    );
    await userEvent.tab();
    await expect(body.findByTestId("add-marketplace-found")).resolves.toBeDefined();
    await expect(body.findByTestId("add-marketplace-submit")).resolves.toBeEnabled();
  },
};

/** A reference with no `marketplace.json`: the field is invalid and the notice names both paths. */
export const AddMarketplaceNotAMarketplace: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(await body.findByTestId("add-marketplace-ref"), "pedronauck/dotfiles");
    await userEvent.tab();
    const failure = await body.findByTestId("add-marketplace-failure");
    await expect(failure).toHaveAttribute("data-code", "marketplace_not_a_marketplace");
    await expect(body.findByTestId("add-marketplace-ref")).resolves.toHaveAttribute(
      "aria-invalid",
      "true"
    );
  },
};

/** The document's name is already registered: the notice proposes a name and one click re-checks. */
export const AddMarketplaceNameCollision: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(
      await body.findByTestId("add-marketplace-ref"),
      "anthropics/claude-plugins-official"
    );
    await userEvent.tab();
    const failure = await body.findByTestId("add-marketplace-failure");
    await expect(failure).toHaveAttribute("data-code", "marketplace_source_exists");
    await userEvent.click(await body.findByTestId("add-marketplace-use-suggested"));
    await expect(body.findByTestId("add-marketplace-found")).resolves.toBeDefined();
  },
};

export const AddMarketplaceCollisionNotice: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(
      await body.findByTestId("add-marketplace-ref"),
      "anthropics/claude-plugins-official"
    );
    await userEvent.tab();
    const failure = await body.findByTestId("add-marketplace-failure");
    await expect(failure).toHaveAttribute("data-code", "marketplace_source_exists");
  },
};

/** A name still carried by installed extensions is refused and the notice names them. */
export const AddMarketplaceNameRetained: Story = {
  parameters: appRouteParameters("/marketplace"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(await body.findByTestId("add-marketplace-ref"), "acme/retained-plugins");
    await userEvent.tab();
    const failure = await body.findByTestId("add-marketplace-failure");
    await expect(failure).toHaveAttribute("data-code", "marketplace_source_name_retained");
    await expect(body.findByTestId("add-marketplace-name")).resolves.toHaveAttribute(
      "aria-invalid",
      "true"
    );
  },
};

/** Hold the source dry run at its HTTP boundary so validation remains visible. */
export const AddMarketplaceValidating: Story = {
  parameters: {
    ...appRouteParameters("/marketplace"),
    ...storybookMswParameters({
      marketplace: [
        compozyApiMock.post("/api/marketplace/sources", async () => {
          await delay("infinite");
          return HttpResponse.json(marketplaceSourcePreviewFixture);
        }),
        ...handlers,
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    await userEvent.click(await within(canvasElement).findByTestId("marketplace-add"));
    const body = within(document.body);
    await userEvent.click(await body.findByTestId("marketplace-add-marketplace"));
    await userEvent.type(await body.findByTestId("add-marketplace-ref"), "acme/team-plugins");
    await userEvent.tab();
    await expect(body.findByTestId("add-marketplace-checking")).resolves.toBeVisible();
  },
};
