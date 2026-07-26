import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { aghApiMock } from "@/storybook/openapi-msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { settingsWindowManagerSnapshotFixture } from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/SettingsLayouts",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The workspace layout as a desktop you manipulate directly, the layouts saved from it, and the global window-manager defaults — behind the daemon's validate-then-apply gate.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The stage reads the live snapshot query, which the OS handler group owns.
 * Overriding it there rather than adding a settings handler is deliberate: the
 * groups flatten in key order, `os` precedes `settings`, and MSW resolves the
 * first match — a settings-group handler for this route would never be reached.
 */
const denseSnapshot = aghApiMock.get(
  "/api/workspaces/{workspace_id}/window-manager",
  ({ params }) =>
    HttpResponse.json({
      ...settingsWindowManagerSnapshotFixture,
      workspace_id: String(params.workspace_id),
    })
);

const SEAM_TESTID = "layout-canvas-seam-split-main:0";

/** Three desktops, a nested split, a stack, a floating window and a minimized one. */
export const Default: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/layouts"),
    ...storybookMswParameters({ os: [denseSnapshot] }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** An edit the daemon has not seen yet: Review is armed, Apply is not. */
export const DirtyUnreviewed: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/layouts"),
    ...storybookMswParameters({ os: [denseSnapshot] }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const seam = await canvas.findByTestId(SEAM_TESTID);
    seam.focus();
    await userEvent.keyboard("{ArrowDown}");
    await expect(canvas.getByTestId("layout-review-check")).toBeEnabled();
    await expect(canvas.getByTestId("layout-review-apply")).toBeDisabled();
  },
};

/** The daemon refuses the document; the diagnostics say why and Apply stays shut. */
export const ValidationFailed: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/settings/layouts"),
    ...storybookMswParameters({
      os: [denseSnapshot],
      settings: [
        aghApiMock.post(
          "/api/workspaces/{workspace_id}/window-manager/layout/validate",
          ({ params }) =>
            HttpResponse.json({
              workspace_id: String(params.workspace_id),
              valid: false,
              diagnostics: [
                {
                  code: "topology.group_overlap",
                  path: "desktops.desktop-build.groups.group-main",
                  message: "group frames must not overlap inside a desktop",
                },
              ],
            })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const seam = await canvas.findByTestId(SEAM_TESTID);
    seam.focus();
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.click(canvas.getByTestId("layout-review-check"));
    await expect(await canvas.findByText("topology.group_overlap")).toBeInTheDocument();
    await expect(canvas.getByTestId("layout-review-apply")).toBeDisabled();
  },
};

/**
 * No workspace selected. The global defaults still apply — they are daemon-wide —
 * but there is no layout to show, and the page says so instead of rendering an
 * empty canvas that implies one exists.
 */
export const NoWorkspace: Story = {
  args: {},
  parameters: appRouteParameters("/settings/layouts"),
  render: () => <StorybookRouteCanvas />,
};
