import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { useEffect } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsProvidersCollectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsProviders",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Providers settings route stories covering catalog layout, empty states, loading and the primary create/delete dialogs.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Provider catalog with status counts and the overlay table.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/providers"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the create-editor dialog is open with the name field
 * pre-filled, mirroring the in-progress editor state.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/providers"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookProvidersDirtySetup />
    </>
  ),
};

/**
 * Empty catalog branch before any provider overlays have been defined --
 * exercises the @agh/ui Empty primitive for the zero-providers state.
 */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/providers"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/providers", () =>
          HttpResponse.json({ ...settingsProvidersCollectionFixture, providers: [] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Create-provider editor opened from the page header and persisted through the real mutation flow.
 */
export const CreateProvider: Story = {
  args: {},
  parameters: appRouteParameters("/settings/providers"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("settings-page-providers-create"));
    await userEvent.type(
      await canvas.findByTestId("settings-providers-editor-name-input"),
      "openai"
    );
    await userEvent.type(
      await canvas.findByTestId("settings-providers-editor-command-input"),
      "npx openai"
    );
    await userEvent.click(await canvas.findByTestId("settings-providers-editor-save"));
    await expect(
      canvas.findByTestId("settings-page-providers-action-result")
    ).resolves.toBeDefined();
  },
};

/**
 * Delete dialog and fallback banner for removing an overlay provider that reveals the builtin definition.
 * The Delete action lives inside the card overflow menu now -- open it before clicking the item.
 */
export const DeleteOverlay: Story = {
  args: {},
  parameters: appRouteParameters("/settings/providers"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(
      await canvas.findByTestId("settings-page-providers-card-claude-overflow")
    );
    await userEvent.click(await screen.findByTestId("settings-page-providers-card-claude-delete"));
    await expect(canvas.findByTestId("settings-providers-delete-fallback")).resolves.toBeDefined();
  },
};

/**
 * Initial loading state while the providers collection is still fetching.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/providers"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/providers", async () => {
          await delay("infinite");
          return HttpResponse.json({ ...settingsProvidersCollectionFixture, providers: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch shown when the providers catalog cannot be loaded.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/providers"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/providers", () =>
          HttpResponse.json({ error: "Failed to load providers" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Reaches the dirty editor state by opening the create dialog and seeding the
 * name field via a native value setter. RAF-polls until the route mounts.
 */
function StorybookProvidersDirtySetup() {
  useEffect(() => {
    let cancelled = false;
    let stage: "open" | "fill" = "open";
    const setValue = (element: HTMLInputElement, next: string) => {
      const setter = Object.getOwnPropertyDescriptor(
        window.HTMLInputElement.prototype,
        "value"
      )?.set;
      setter?.call(element, next);
      element.dispatchEvent(new Event("input", { bubbles: true }));
      element.dispatchEvent(new Event("change", { bubbles: true }));
    };
    const advance = () => {
      if (cancelled) return;
      if (stage === "open") {
        const trigger = document.querySelector<HTMLButtonElement>(
          '[data-testid="settings-page-providers-create"]'
        );
        if (trigger) {
          trigger.click();
          stage = "fill";
        }
      } else if (stage === "fill") {
        const input = document.querySelector<HTMLInputElement>(
          '[data-testid="settings-providers-editor-name-input"]'
        );
        if (input) {
          setValue(input, "dirty-provider");
          return;
        }
      }
      requestAnimationFrame(advance);
    };
    requestAnimationFrame(advance);
    return () => {
      cancelled = true;
    };
  }, []);
  return null;
}
