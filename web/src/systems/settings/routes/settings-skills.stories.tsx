import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { StorybookFieldDirtySetup } from "@/storybook/settings-state-helpers";
import { settingsSkillsSectionFixture } from "@/systems/settings/mocks";
import {
  StorybookRestartNoticeSetup,
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsSkills",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Skills settings route stories covering policy editing, disabled-skill empty states, restart notices, and request failures.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default skills configuration surface with disabled-skill list and marketplace policy.
 */
export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/settings/skills"),
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Dirty shell state -- the marketplace registry has been edited so the policy
 * section's save controls enable.
 */
export const Dirty: Story = {
  args: {},
  parameters: appRouteParameters("/settings/skills"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookFieldDirtySetup
        testId="settings-page-skills-marketplace-registry-input"
        value="dirty-registry"
      />
    </>
  ),
};

/**
 * Disabled-skills empty branch when nothing has been opted out -- exercises
 * the @compozy/ui Empty primitive for the no-skills state.
 */
export const DisabledEmpty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/skills"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/skills", () =>
          HttpResponse.json({
            ...settingsSkillsSectionFixture,
            disabled_count: 0,
            config: {
              ...settingsSkillsSectionFixture.config,
              disabled_skills: [],
            },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Restart banner after saving marketplace policy that only applies after daemon restart.
 */
export const RestartNotice: Story = {
  args: {},
  parameters: appRouteParameters("/settings/skills"),
  render: () => (
    <>
      <StorybookWorkspaceSetup />
      <StorybookRestartNoticeSetup section="skills" />
    </>
  ),
};

/**
 * Loading state before the skills settings section resolves.
 */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/skills"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/skills", async () => {
          await delay("infinite");
          return HttpResponse.json(settingsSkillsSectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Runtime down: counts and folder states disappear because nothing was measured,
 * while the switches and the folder editor keep working — these are preferences,
 * not live state.
 */
export const SourcesRuntimeUnavailable: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/skills"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/skills", () =>
          HttpResponse.json({
            ...settingsSkillsSectionFixture,
            runtime_available: false,
            sources: settingsSkillsSectionFixture.sources.map(source => ({
              ...source,
              roots: source.roots.map(root => {
                const { scanned_count: _scanned, skill_count: _skills, ...rest } = root;
                return rest;
              }),
            })),
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Only CompozyOS's own folders are on, and no folder was added by hand.
 */
export const SourcesDefaultsOnly: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/skills"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/skills", () =>
          HttpResponse.json({
            ...settingsSkillsSectionFixture,
            config: {
              ...settingsSkillsSectionFixture.config,
              sources: [],
              custom_sources: [],
            },
            sources: settingsSkillsSectionFixture.sources
              .filter(source => source.kind !== "custom")
              .map(source => (source.always_on ? source : { ...source, enabled: false })),
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * Error branch when the skills settings request fails.
 */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/skills"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/skills", () =>
          HttpResponse.json({ error: "Failed to load skills settings" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
