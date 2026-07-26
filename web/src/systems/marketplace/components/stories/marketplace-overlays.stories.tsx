import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
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

export const MCPStdioTyped: Story = {
  parameters: appRouteParameters("/marketplace/mcps"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install github" }));
    await expect(within(document.body).findByTestId("mcp-install-dialog")).resolves.toBeDefined();
  },
};

export const MCPVaultSelector: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/mcps"),
    ...storybookMswParameters({
      vault: [
        aghApiMock.get("/api/vault/secrets", () =>
          HttpResponse.json({
            secrets: [
              {
                created_at: "2026-07-14T11:00:00Z",
                kind: "mcp_env",
                namespace: "mcp",
                present: true,
                ref: "vault:mcp/ws/ws_story_fintech/github/env/GITHUB_TOKEN",
                updated_at: "2026-07-14T11:00:00Z",
              },
            ],
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install github" }));
    const dialog = within(document.body);
    await userEvent.click(await dialog.findByRole("button", { name: "Use Vault" }));
    const vaultRadio = await dialog.findByRole("radio", {
      name: /vault:mcp\/ws\/ws_story_fintech\/github\/env\/GITHUB_TOKEN/,
    });
    await userEvent.click(vaultRadio);
    await expect(vaultRadio).toHaveAttribute("aria-checked", "true");
  },
};

export const MCPVaultCreate: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/mcps"),
    ...storybookMswParameters({
      vault: [aghApiMock.get("/api/vault/secrets", () => HttpResponse.json({ secrets: [] }))],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install github" }));
    const dialog = within(document.body);
    await userEvent.click(await dialog.findByRole("button", { name: "Use Vault" }));
    await userEvent.click(await dialog.findByRole("button", { name: "Create Vault secret" }));
    await expect(dialog.findByText("Create inline")).resolves.toBeDefined();
  },
};

export const MCPRemote: Story = {
  parameters: appRouteParameters("/marketplace/mcps"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install linear" }));
    const dialog = within(document.body);
    await expect(dialog.findByText("Authorization · OAuth 2 PKCE")).resolves.toBeDefined();
    await expect(dialog.queryByText("Required configuration")).toBeNull();
  },
};

export const BundlePreview: Story = {
  parameters: appRouteParameters("/marketplace/bundles"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Activate dep-kit" }));
    await expect(within(document.body).findByText("What changes")).resolves.toBeDefined();
  },
};

export const BundleConflict: Story = {
  parameters: {
    ...appRouteParameters("/marketplace/bundles"),
    ...storybookMswParameters({
      marketplace: [
        aghApiMock.post("/api/bundles/preview", () =>
          HttpResponse.json(
            { error: "Activation conflicts with the existing dependency-review channel" },
            { status: 409 }
          )
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Activate dep-kit" }));
    const dialog = within(document.body);
    await expect(dialog.findByTestId("bundle-preview-error")).resolves.toBeDefined();
    await expect(dialog.findByTestId("bundle-activate-confirm")).resolves.toBeDisabled();
  },
};

export const ExtensionWarning: Story = {
  parameters: appRouteParameters("/marketplace/extensions"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Install slack-notify" }));
    const dialog = within(document.body);
    await expect(dialog.findByText("Unsigned package")).resolves.toBeDefined();
    await expect(dialog.findByText("Network egress")).resolves.toBeDefined();
  },
};
