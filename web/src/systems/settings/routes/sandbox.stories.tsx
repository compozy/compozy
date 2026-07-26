import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { useEffect } from "react";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsSandboxesCollectionFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/Sandbox",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Sandbox profile route stories covering the listing inventory, detail sheet, empty state, editor flow, delete warnings, and request failures.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default sandbox catalog with workspace usage counts in ListingPage rows.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Cards view preserves inspect, edit, and delete controls for every profile.
 */
export const Cards: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Detail sheet opened on the daytona-eu profile (nested network/daytona/env).
 * RAF setup opens the sheet for static Storybook/iframe capture (play is interaction-only).
 */
export const ProfileSheet: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookSandboxSheetSetup />
    </>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("sandbox-profile-sheet")).resolves.toBeDefined();
    await expect(canvas.findByTestId("sandbox-profile-sheet-daytona")).resolves.toBeDefined();
  },
};

/**
 * Dirty editor state , the new-sandbox dialog is open with the name field
 * pre-filled so the Create button enables for the story.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookSandboxesDirtySetup />
    </>
  ),
};

/**
 * Empty-state branch shown before any sandbox profiles have been created.
 */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/sandbox"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/sandboxes", () =>
          HttpResponse.json({ ...settingsSandboxesCollectionFixture, sandboxes: [] })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Create-sandbox editor opened from the collection header.
 */
export const CreateSandbox: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("sandbox-page-create"));
    await userEvent.type(await canvas.findByTestId("sandbox-editor-name-input"), "preview");
    await userEvent.click(await canvas.findByTestId("settings-sandbox-editor-save"));
    await expect(canvas.findByTestId("sandbox-page-action-result")).resolves.toBeDefined();
  },
};

/**
 * Delete dialog with workspace usage warning for a sandbox still referenced by workspaces.
 */
export const DeleteProfile: Story = {
  args: {},
  parameters: appRouteParameters("/sandbox?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("sandbox-page-card-local-delete"));
    await expect(canvas.findByTestId("sandbox-delete-usage")).resolves.toBeDefined();
  },
};

/**
 * Loading state while the sandboxes collection is still resolving.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/sandbox"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/sandboxes", async () => {
          await delay("infinite");
          return HttpResponse.json({ ...settingsSandboxesCollectionFixture, sandboxes: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch when the sandboxes request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/sandbox"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/sandboxes", () =>
          HttpResponse.json({ error: "Failed to load sandboxes" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Opens the daytona-eu detail sheet once the listing mounts (static capture).
 */
function StorybookSandboxSheetSetup() {
  useEffect(() => {
    let cancelled = false;
    const advance = () => {
      if (cancelled) return;
      const sheet = document.querySelector('[data-testid="sandbox-profile-sheet"]');
      if (sheet && sheet.getAttribute("data-state") === "open") return;
      const trigger = document.querySelector<HTMLButtonElement>(
        '[data-testid="sandbox-page-select-daytona-eu"]'
      );
      if (trigger) {
        trigger.dispatchEvent(
          new MouseEvent("click", { bubbles: true, cancelable: true, view: window })
        );
        return;
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

/**
 * Reaches the dirty editor state by opening the new-sandbox dialog and
 * seeding the name field. RAF-polls until the route mounts.
 */
function StorybookSandboxesDirtySetup() {
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
          '[data-testid="sandbox-page-create"]'
        );
        if (trigger) {
          trigger.click();
          stage = "fill";
        }
      } else if (stage === "fill") {
        const input = document.querySelector<HTMLInputElement>(
          '[data-testid="sandbox-editor-name-input"]'
        );
        if (input) {
          setValue(input, "dirty-sandbox");
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
