import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { memoryReadFixtures } from "@/systems/knowledge/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/knowledge/routes/Knowledge",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Route stories for the Memory v2 knowledge browser, covering scope tabs, agent inputs, server-backed recall, decision context, edit/delete dialogs, and truthful loading/empty/error states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default knowledge view with global memories from MSW fixtures.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Workspace scope selected from the route header.
 */
export const WorkspaceScope: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("tab-workspace"));
    await expect(canvas.findByTestId("knowledge-list-panel")).resolves.toBeDefined();
  },
};

/**
 * Agent scope after typing an agent name; exposes scope + tier inputs.
 */
export const AgentScope: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("tab-agent"));
    await expect(canvas.findByTestId("knowledge-guard")).resolves.toBeDefined();
    await userEvent.type(await canvas.findByTestId("agent-name-input"), "cto-agent");
    await expect(canvas.findByTestId("agent-tier-pills")).resolves.toBeDefined();
  },
};

/**
 * Empty list response from the daemon.
 */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/knowledge"),
    ...storybookMswParameters({
      knowledge: [
        aghApiMock.get("/api/memory", () =>
          HttpResponse.json({
            memories: [],
            page: { has_more: false, limit: 50, total: 0 },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Detail panel loading state while the selected memory content is still fetching.
 */
export const ContentLoading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/knowledge"),
    ...storybookMswParameters({
      knowledge: [
        aghApiMock.get("/api/memory/{filename}", async () => {
          await delay("infinite");
          return HttpResponse.json(memoryReadFixtures["operator-style.md"]!);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Detail panel error state when the memory content fetch fails.
 */
export const ContentError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/knowledge"),
    ...storybookMswParameters({
      knowledge: [
        aghApiMock.get("/api/memory/{filename}", () =>
          HttpResponse.json({ code: "memory.read_failed", message: "boom" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Search results from POST /api/memory/search after the user types a query.
 */
export const SearchResults: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const search = await canvas.findByTestId("knowledge-search-input");
    await userEvent.type(search, "launch");
    await expect(canvas.findByTestId("knowledge-search-info")).resolves.toBeDefined();
  },
};

/**
 * Delete confirmation dialog opened from the detail panel delete button.
 */
export const DeleteDialog: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const deleteBtn = await canvas.findByTestId("delete-memory-btn");
    await userEvent.click(deleteBtn);
    await expect(
      within(canvasElement.ownerDocument.body).findByTestId("knowledge-delete-dialog")
    ).resolves.toBeDefined();
  },
};

/**
 * Edit confirmation dialog opened from the detail panel edit button.
 */
export const EditDialog: Story = {
  args: {},
  parameters: appRouteParameters("/knowledge"),
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const editBtn = await canvas.findByTestId("edit-memory-btn");
    await userEvent.click(editBtn);
    await expect(
      within(canvasElement.ownerDocument.body).findByTestId("knowledge-edit-dialog")
    ).resolves.toBeDefined();
  },
};
