import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { settingsAutomationSectionFixture } from "@/systems/settings/mocks";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/automation/routes/Automations",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The Automations listing (`/automations`, board S1) with the real shell: the board's seven automations as sentence rows or cards, Start views, filters, and the empty and system states.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const emptyJobs = compozyApiMock.get("/api/automation/jobs", () =>
  HttpResponse.json({ jobs: [], page: { has_more: false, limit: 50, total: 0 } })
);
const emptyTriggers = compozyApiMock.get("/api/automation/triggers", () =>
  HttpResponse.json({ triggers: [], page: { has_more: false, limit: 50, total: 0 } })
);

/** list VC-01 — all seven automations as sentence rows. */
export const Default: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automations-list-rows")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automation-last-run-nightly-delivery")).toHaveTextContent(
      "Last run failed"
    );
  },
};

/** list VC-01 — the Scheduled Start view loads only the jobs list. */
export const ScheduledView: Story = {
  args: {},
  parameters: appRouteParameters("/automations?start=schedule"),
  render: () => <StorybookWorkspaceSetup />,
};

/** list VC-02 — cards display. */
export const Cards: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automations-list-card-grid")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** list VC-03 — Does = Start a Loop applied. */
export const FilterApplied: Story = {
  args: {},
  parameters: appRouteParameters("/automations?target=loop"),
  render: () => <StorybookWorkspaceSetup />,
};

/** list VC-04 — first run: two starts, suggestions, no toolbar. */
export const Empty: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations"),
    ...storybookMswParameters({ automation: [emptyJobs, emptyTriggers] }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automations-list-empty")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByTestId("automations-empty-start-schedule")).toHaveAttribute(
      "href",
      expect.stringContaining("start=schedule")
    );
    await expect(canvas.getByTestId("automations-empty-start-event")).toHaveAttribute(
      "href",
      expect.stringContaining("start=event")
    );
    await expect(canvas.queryByTestId("automation-search-input")).toBeNull();
  },
};

/** list VC-06 — runtime off: one warning with Open Settings; row actions disabled. */
export const Unavailable: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations"),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/automation", () =>
          HttpResponse.json({
            ...settingsAutomationSectionFixture,
            runtime: { ...settingsAutomationSectionFixture.runtime, available: false },
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automations-runtime-alert")).toBeVisible(), {
      timeout: 5000,
    });
    await expect(canvas.getByRole("link", { name: "Open Settings" })).toHaveAttribute(
      "href",
      "/settings/automation"
    );
  },
};

/** list VC-05 — filtered empty with Clear filters. */
export const FilteredEmpty: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations?start=event&q=zzzz"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(
      () => expect(canvas.getByTestId("automations-list-filtered-empty")).toBeVisible(),
      { timeout: 5000 }
    );
  },
};

/** list VC-06 — loading skeleton rows. */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/automations"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/jobs", async () => {
          await delay("infinite");
          return HttpResponse.json({ jobs: [], page: { has_more: false, limit: 50, total: 0 } });
        }),
        compozyApiMock.get("/api/automation/triggers", async () => {
          await delay("infinite");
          return HttpResponse.json({
            triggers: [],
            page: { has_more: false, limit: 50, total: 0 },
          });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** list VC-06 — both lists failed. */
export const LoadError: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/automations"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/jobs", () =>
          HttpResponse.json({ error: "jobs unavailable" }, { status: 500 })
        ),
        compozyApiMock.get("/api/automation/triggers", () =>
          HttpResponse.json({ error: "triggers unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** Business Rule 17 — event automations failed; scheduled rows stay under the alert. */
export const PartialFailure: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters("/automations"),
    ...storybookMswParameters({
      automation: [
        compozyApiMock.get("/api/automation/triggers", () =>
          HttpResponse.json({ error: "triggers unavailable" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await waitFor(() => expect(canvas.getByTestId("automations-partial-alert")).toBeVisible(), {
      timeout: 5000,
    });
  },
};

/** Row overflow for a dynamic schedule. */
export const RowOverflow: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/automations"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const more = await canvas.findByTestId("automation-more-morning-digest", undefined, {
      timeout: 5000,
    });
    await userEvent.click(more);
    await expect(
      within(document.body).findByTestId("automation-run-now-morning-digest")
    ).resolves.toBeDefined();
  },
};

/** Shim (removed in v0.5.0): `/jobs?…` replace-redirects to the Scheduled view. */
export const LegacyJobsRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/jobs?enabled=true"),
  render: () => <StorybookWorkspaceSetup />,
};

/** Shim (removed in v0.5.0): `/jobs/:id` replace-redirects to the automation detail. */
export const LegacyJobDetailRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/jobs/morning-digest"),
  render: () => <StorybookWorkspaceSetup />,
};

/** Shim (removed in v0.5.0): `/triggers?event=` becomes the On events view searched by event. */
export const LegacyTriggersRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/triggers?event=session.stopped"),
  render: () => <StorybookWorkspaceSetup />,
};

/** Shim (removed in v0.5.0): `/triggers/:id` replace-redirects to the automation detail. */
export const LegacyTriggerDetailRedirect: Story = {
  args: {},
  parameters: appRouteParameters("/triggers/rerun-delivery"),
  render: () => <StorybookWorkspaceSetup />,
};
