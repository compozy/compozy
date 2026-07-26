import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import {
  rolesStatusFixture,
  rolesStatusWithDiagnosticFixture,
  settingsRolesConfigFixture,
  settingsRolesSectionFixture,
} from "@/systems/settings/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsRoles",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Roles settings route stories covering the truthful projection: builtin/inherit/off badges, resolves-at-invocation affordances, the memory-controller timeout, resolution diagnostics, and the editable fallback chain.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Reveals the Dream panel (second in product order) so below-fold states capture cleanly. */
async function scrollDreamIntoView(canvasElement: HTMLElement) {
  const findDream = () =>
    canvasElement.querySelector('[data-testid="settings-page-roles-group-dream"]');
  for (let attempt = 0; attempt < 40 && !findDream(); attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  findDream()?.scrollIntoView({ block: "start" });
}

/**
 * Populated surface — all six roles in product order. Coordinator is OFF
 * (disabled); auto_title, memory_extractor, and memory_controller are INHERIT
 * with "Resolves at invocation." affordances; dream and checkpoint are BUILTIN.
 */
export const Populated: Story = {
  args: {},
  parameters: appRouteParameters("/settings/roles"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dream routed to a missing catalog agent — the row shows an inline warning
 * (`role_agent_not_found`) with the agent as mono metadata.
 */
export const Diagnostics: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/roles", () => HttpResponse.json(rolesStatusWithDiagnosticFixture)),
      ],
    }),
  },
  play: ({ canvasElement }) => scrollDreamIntoView(canvasElement),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Editable fallback chain: dream carries two routes, the second missing its
 * provider so the advanced fold opens with an inline validation error.
 */
export const FallbackEditor: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/settings/roles", () =>
          HttpResponse.json({
            ...settingsRolesSectionFixture,
            config: {
              ...settingsRolesConfigFixture,
              dream: {
                ...settingsRolesConfigFixture.dream,
                fallback_chain: [
                  { provider: "anthropic", model: "claude-sonnet-5", reasoning_effort: "" },
                  { provider: "", model: "gpt-5", reasoning_effort: "high" },
                ],
              },
            },
          })
        ),
      ],
    }),
  },
  play: ({ canvasElement }) => scrollDreamIntoView(canvasElement),
  render: () => <StorybookWorkspaceSetup />,
};

/** Loading state before both role reads resolve. */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/roles", async () => {
          await delay("infinite");
          return HttpResponse.json(rolesStatusFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Error branch when the projection read fails — Retry only. */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        aghApiMock.get("/api/roles", () =>
          HttpResponse.json({ error: "Failed to load roles" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
