import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

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
          "Roles settings route stories covering the truthful projection: built-in/default-agent resolution lines, decided-when-it-runs affordances, resolution diagnostics, and the editable fallback chain.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Populated surface — both roles in product order. Coordinator is OFF
 * (disabled) and built in; auto_title uses the default agent ("Decided when
 * the role runs.").
 */
export const Populated: Story = {
  args: {},
  parameters: appRouteParameters("/settings/roles"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Auto title routed to a missing catalog agent — the row shows an inline warning
 * (`role_agent_not_found`) with the agent as mono metadata.
 */
export const Diagnostics: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/roles", () => HttpResponse.json(rolesStatusWithDiagnosticFixture)),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Editable fallback chain: auto_title carries two routes, the second missing its
 * provider so the advanced fold opens with an inline validation error.
 */
export const FallbackEditor: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/roles", () =>
          HttpResponse.json({
            ...settingsRolesSectionFixture,
            config: {
              ...settingsRolesConfigFixture,
              auto_title: {
                ...settingsRolesConfigFixture.auto_title,
                fallback_chain: [
                  {
                    provider: "anthropic",
                    model: "claude-sonnet-5",
                    reasoning_effort: "",
                    acp_options: [],
                    command: "",
                  },
                  {
                    provider: "",
                    model: "gpt-5",
                    reasoning_effort: "high",
                    acp_options: [],
                    command: "",
                  },
                ],
              },
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Loading state before both role reads resolve. */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/roles"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/roles", async () => {
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
        compozyApiMock.get("/api/roles", () =>
          HttpResponse.json({ error: "Failed to load roles" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
