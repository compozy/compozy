import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { buildCreatedTaskFixture } from "@/systems/tasks/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/tasks/routes/TaskCreate",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Task create-route stories rendered inside the persistent tasks shell, including template search params and submit-pending state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default create route using the one-shot template and the shared editor surface.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/new"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Template-specific create route driven by the real search param contract.
 */
export const TemplatePreset: Story = {
  args: {},
  parameters: appRouteParameters("/tasks/new?template=human_in_loop"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Submit-pending state after the create mutation starts and the request remains in flight.
 */
export const Submitting: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/tasks/new"),
    ...storybookMswParameters({
      tasks: [
        aghApiMock.post("/api/tasks", async () => {
          await delay("infinite");
          return HttpResponse.json({ task: buildCreatedTaskFixture() }, { status: 201 });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(await canvas.findByTestId("task-title-input"), "Create API contract");
    await userEvent.click(await canvas.findByTestId("task-editor-modal-submit"));
    await expect(canvas.findByTestId("task-editor-modal-submit")).resolves.toBeDisabled();
  },
};
