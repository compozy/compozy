import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const deliveryLoopRoute = "/loops/software-delivery";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/loops/routes/Loops",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full app-shell route stories for the Loops catalog and the software-delivery detail, configure, editor, and run routes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Catalog: Story = {
  args: {},
  parameters: appRouteParameters("/loops"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Detail: Story = {
  args: {},
  parameters: appRouteParameters(deliveryLoopRoute),
  render: () => <StorybookWorkspaceSetup />,
};

export const Configure: Story = {
  args: {},
  parameters: appRouteParameters(`${deliveryLoopRoute}/configure`),
  render: () => <StorybookWorkspaceSetup />,
};

export const Editor: Story = {
  args: {},
  parameters: appRouteParameters(`${deliveryLoopRoute}/editor`),
  render: () => <StorybookWorkspaceSetup />,
};

export const RunForm: Story = {
  args: {},
  parameters: appRouteParameters(`${deliveryLoopRoute}/run`),
  render: () => <StorybookWorkspaceSetup />,
};

export const EmptyCatalog: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loops"),
    ...storybookMswParameters({
      loops: [
        aghApiMock.get("/api/workspaces/{workspace_id}/loops", () =>
          HttpResponse.json({
            facets: { categories: {}, kinds: {}, statuses: {} },
            loops: [],
            page: { has_more: false, limit: 50, total: 0 },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const LoadingCatalog: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/loops"),
    ...storybookMswParameters({
      loops: [
        aghApiMock.get("/api/workspaces/{workspace_id}/loops", async () => {
          await delay("infinite");
          return HttpResponse.json({
            facets: { categories: {}, kinds: {}, statuses: {} },
            loops: [],
            page: { has_more: false, limit: 50, total: 0 },
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
