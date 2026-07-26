import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/bridges/routes/Bridges",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Real-shell stories for bridges, including list-detail composition, empty states and dialog-driven bridge operations.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default bridges route with the selected bridge detail visible.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/bridges"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Empty-state branch used before the workspace has any configured bridges.
 */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/bridges"),
    ...storybookMswParameters({
      bridges: [
        aghApiMock.get("/api/bridges", () =>
          HttpResponse.json({
            bridge_health: {},
            bridges: [],
            facets: {
              platforms: {},
              statuses: {
                auth_required: 0,
                degraded: 0,
                disabled: 0,
                error: 0,
                ready: 0,
                starting: 0,
              },
            },
            page: { has_more: false, limit: 50, total: 0 },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Bridge creation dialog opened from the primary CTA.
 */
export const CreateDialog: Story = {
  args: {},
  parameters: appRouteParameters("/bridges"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("create-bridge-btn")).toBeEnabled(), {
      timeout: 5000,
    });
    const createBridgeButton = canvas.getByTestId("create-bridge-btn");
    await waitFor(() => expect(createBridgeButton).toBeEnabled());
    await userEvent.click(createBridgeButton);
    await expect(within(document.body).findByTestId("bridge-create-dialog")).resolves.toBeDefined();
  },
};

/**
 * Delivery test reached from the detail route — it opens the editor in Advanced
 * rather than a standalone dialog (D2).
 */
export const TestDelivery: Story = {
  args: {},
  parameters: appRouteParameters("/bridges/brg_launch_room"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("open-test-delivery-btn")).toBeEnabled(), {
      timeout: 5000,
    });
    const button = canvas.getByTestId("open-test-delivery-btn");
    await userEvent.click(button);
    await expect(
      within(document.body).findByTestId("bridge-delivery-test-panel")
    ).resolves.toBeDefined();
  },
};

/**
 * Initial loading state for the bridges list query.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/bridges"),
    ...storybookMswParameters({
      bridges: [
        aghApiMock.get("/api/bridges", async () => {
          await delay("infinite");
          return HttpResponse.json({
            bridge_health: {},
            bridges: [],
            facets: {
              platforms: {},
              statuses: {
                auth_required: 0,
                degraded: 0,
                disabled: 0,
                error: 0,
                ready: 0,
                starting: 0,
              },
            },
            page: { has_more: false, limit: 50, total: 0 },
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
