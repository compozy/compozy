import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { fireEvent, expect, userEvent, waitFor, within } from "storybook/test";

import { storyAgentNames, storyWorkspaceIds } from "@/storybook/fintech-scenario";
import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { agentFixtures } from "@/systems/agent/mocks";

const settingsRoute = `/agents/${storyAgentNames.fraud}/settings`;

function AgentWorkspaceSetup() {
  return <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />;
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/agent/routes/AgentSettings",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Agent settings modal overlay with section navigation, dirty footer Save/Cancel, and Danger zone.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Basics: Story = {
  args: {},
  parameters: appRouteParameters(settingsRoute),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-page")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-settings-basics")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-settings-name")).resolves.toHaveAttribute("readonly");
  },
};

export const Danger: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=danger`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-danger")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-settings-delete")).resolves.toBeDefined();
  },
};

export const Runtime: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=runtime`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-runtime")).resolves.toBeDefined();
    const reset = await canvas.findByTestId("agent-settings-runtime-use-project-defaults");
    reset.scrollIntoView({ block: "center" });
    await waitFor(() => expect(reset).toBeVisible());
  },
};

export const Instructions: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=instructions`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-instructions")).resolves.toBeDefined();
  },
};

export const Access: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=access`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-access")).resolves.toBeDefined();
  },
};

export const McpServers: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=mcp`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.findByTestId("agent-settings-mcp")).resolves.toBeDefined();
  },
};

export const DirtyInstructions: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=instructions`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const prompt = await canvas.findByTestId("agent-settings-prompt");
    await fireEvent.change(prompt, { target: { value: "Review every release gate before ship." } });
    await expect(canvas.findByTestId("agent-settings-unsaved")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-settings-save")).resolves.toBeDefined();
  },
};

export const DefinitionConflict: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=instructions`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.put("/api/agents/{name}", () =>
          HttpResponse.json({ error: "definition digest conflict" }, { status: 409 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const prompt = await canvas.findByTestId("agent-settings-prompt");
    await fireEvent.change(prompt, { target: { value: "A concurrent definition update." } });
    await userEvent.click(await canvas.findByRole("button", { name: "Save changes" }));
    await expect(canvas.findByTestId("agent-settings-conflict-banner")).resolves.toBeDefined();
  },
};

export const PermissionDenied: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=instructions`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.put("/api/agents/{name}", () =>
          HttpResponse.json({ error: "mutation forbidden" }, { status: 403 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const prompt = await canvas.findByTestId("agent-settings-prompt");
    await fireEvent.change(prompt, { target: { value: "An unauthorized definition update." } });
    await userEvent.click(await canvas.findByRole("button", { name: "Save changes" }));
    await expect(canvas.findByTestId("agent-settings-mutation-denied")).resolves.toBeDefined();
    await expect(canvas.findByTestId("agent-settings-save")).resolves.toBeDisabled();
  },
};

export const RuntimeSelectorOpen: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=runtime`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-settings-runtime")).toBeInTheDocument();
    const trigger = await body.findByTestId("agent-settings-runtime-select");
    const openButton =
      within(trigger).queryByRole("button", { name: /^Model:/i }) ??
      within(trigger).queryByRole("button", { name: /^Open runtime selector$/i }) ??
      within(trigger).getByRole("button", { name: /^Provider:/i });
    await waitFor(() => expect(openButton).toBeEnabled(), { timeout: 15_000 });
    await userEvent.click(openButton);
    await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  },
};

export const InstructionsLong: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=instructions`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", () =>
          HttpResponse.json({
            agent: {
              ...agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!,
              prompt: Array.from(
                { length: 24 },
                (_, index) =>
                  `Gate ${index + 1}: verify payout holds, reserve anomalies, sanctions evidence, and launch-room approval before any merchant action ships.`
              ).join("\n"),
            },
          })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-settings-instructions")).toBeInTheDocument();
    await expect(await body.findByTestId("agent-settings-prompt")).toBeInTheDocument();
  },
};

export const AccessPopulated: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=access`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", () =>
          HttpResponse.json({
            agent: {
              ...agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!,
              tools: ["agh__skill_view", "mcp__github__*"],
              toolsets: ["agh__catalog"],
              deny_tools: ["agh__task_delete"],
              skills: { disabled: ["code-review"] },
            },
          })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-settings-access")).toBeInTheDocument();
    await expect(await body.findByTestId("agent-settings-tools")).toHaveTextContent(
      "agh__skill_view"
    );
  },
};

export const McpServersPopulated: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=mcp`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", () =>
          HttpResponse.json({
            agent: {
              ...agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!,
              mcp_servers: [
                {
                  name: "github",
                  transport: "stdio",
                  command: "npx",
                  env: { GITHUB_API_URL: "redacted" },
                  secret_env: { GITHUB_TOKEN: "redacted" },
                },
              ],
            },
          })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-settings-mcp")).toBeInTheDocument();
  },
};

export const McpServersEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=mcp`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.get("/api/agents/{name}", () =>
          HttpResponse.json({
            agent: {
              ...agentFixtures.find(agent => agent.name === storyAgentNames.fraud)!,
              mcp_servers: [],
            },
          })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByTestId("agent-settings-mcp")).toBeInTheDocument();
  },
};

export const FieldValidationError: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=basics`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const category = await body.findByTestId("agent-settings-category");
    await fireEvent.change(category, { target: { value: "Engineering/" } });
    await expect(await body.findByTestId("agent-settings-category-error")).toBeInTheDocument();
  },
};

export const SaveFailure: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(`${settingsRoute}?section=instructions`),
    ...storybookMswParameters({
      agent: [
        aghApiMock.put("/api/agents/{name}", () =>
          HttpResponse.json({ error: "agent store unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const prompt = await body.findByTestId("agent-settings-prompt");
    await fireEvent.change(prompt, { target: { value: "A save that fails on the server." } });
    await userEvent.click(await body.findByRole("button", { name: "Save changes" }));
    await expect(await body.findByTestId("agent-settings-save-error")).toBeInTheDocument();
  },
};

export const DeleteConfirm: Story = {
  args: {},
  parameters: appRouteParameters(`${settingsRoute}?section=danger`),
  render: () => <AgentWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("agent-settings-delete"));
    await expect(await body.findByTestId("agent-delete-dialog")).toBeInTheDocument();
  },
};
