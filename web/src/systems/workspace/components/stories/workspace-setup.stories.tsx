import { expect, userEvent, waitFor, within } from "storybook/test";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { StorySurface } from "@/storybook/story-layout";
import { primaryWorkspaceFixture } from "@/systems/workspace/mocks/fixtures";

import { useWorkspaceSetupContent } from "../../hooks/use-workspace-setup-content";
import type { WorkspaceSetupDefaultsModel } from "../../lib/workspace-setup-defaults";
import { WorkspaceSetupDialog } from "../workspace-setup";

const storyDefaults: WorkspaceSetupDefaultsModel = {
  agents: { state: "ready", entries: [] },
  sandboxes: { state: "ready", entries: [] },
};

const meta: Meta<typeof WorkspaceSetupDialog> = {
  title: "systems/workspace/components/WorkspaceSetup",
  component: WorkspaceSetupDialog,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function WorkspaceSetupDialogHarness({
  onOpenChange,
  onWorkspaceResolved,
}: {
  onOpenChange: (open: boolean) => void;
  onWorkspaceResolved: (workspaceId: string) => void;
}) {
  const setup = useWorkspaceSetupContent({
    onWorkspaceResolved,
    onSuccessClose: () => onOpenChange(false),
  });
  return (
    <WorkspaceSetupDialog
      model={{ defaults: storyDefaults, setup }}
      onOpenChange={onOpenChange}
      open
    />
  );
}

function dialogHarness(padding: string) {
  return (
    <StorySurface className={padding}>
      <WorkspaceSetupDialogHarness
        onOpenChange={() => undefined}
        onWorkspaceResolved={() => undefined}
      />
    </StorySurface>
  );
}

/** Stalls the directory browse so the reading state stays on screen. */
const browseLoading = storybookMswParameters({
  workspace: [
    compozyApiMock.get("/api/fs/browse", async () => {
      await delay("infinite");
      return HttpResponse.json({
        entries: [],
        home: "/Users/pedro",
        path: "/Users/pedro",
        roots: ["/"],
      });
    }),
  ],
});

const browseEmpty = storybookMswParameters({
  workspace: [
    compozyApiMock.get("/api/fs/browse", () =>
      HttpResponse.json({
        entries: [],
        home: "/Users/pedro",
        parent: "/Users",
        path: "/Users/pedro",
        roots: ["/"],
      })
    ),
  ],
});

const browseError = storybookMswParameters({
  workspace: [
    compozyApiMock.get("/api/fs/browse", () =>
      HttpResponse.json({ error: "permission denied" }, { status: 403 })
    ),
  ],
});

/** VC-03 capture target: the xl split shell at desktop width. */
export const SetupDialogOpen: Story = {
  args: {},
  render: () => dialogHarness("p-10"),
};

/** Below 980px the split body collapses and session defaults stack underneath. */
export const SetupDialogStacked: Story = {
  args: {},
  parameters: {
    viewport: { defaultViewport: "ipad" },
    docs: {
      description: {
        story:
          "Below 980px the two panes collapse to a single column with session defaults stacked beneath Location.",
      },
    },
  },
  render: () => dialogHarness("p-4"),
};

/** A root chosen in the browser, ready for the single create on submit. */
export const SetupDialogRootSelected: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const useCurrent = await canvas.findByTestId("workspace-setup-browser-use-current");
    // The control stays disabled until the browse resolves a current path; clicking
    // before then is a silent no-op.
    await waitFor(() => expect(useCurrent).toBeEnabled());
    await userEvent.click(useCurrent);
    await expect(await canvas.findByTestId("workspace-setup-selected-root")).toBeVisible();
  },
};

export const SetupDialogBrowserLoading: Story = {
  args: {},
  parameters: browseLoading,
  render: () => dialogHarness("p-10"),
};

export const SetupDialogBrowserEmpty: Story = {
  args: {},
  parameters: browseEmpty,
  render: () => dialogHarness("p-10"),
};

export const SetupDialogBrowserError: Story = {
  args: {},
  parameters: browseError,
  render: () => dialogHarness("p-10"),
};

/** Registration spinner: stall `POST /api/workspaces` after picking a root. */
export const SetupDialogLoadingCreate: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...storybookMswParameters({
      workspace: [
        compozyApiMock.post("/api/workspaces", async () => {
          await delay("infinite");
          return HttpResponse.json({ workspace: primaryWorkspaceFixture }, { status: 201 });
        }),
      ],
    }),
  },
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const useCurrent = await canvas.findByTestId("workspace-setup-browser-use-current");
    await waitFor(() => expect(useCurrent).toBeEnabled());
    await userEvent.click(useCurrent);
    await userEvent.click(canvas.getByTestId("workspace-setup-submit"));
    await expect(canvas.getByTestId("workspace-setup-submit")).toBeDisabled();
  },
};

/** Registration failure keeps the draft and reports inline. */
export const SetupDialogCreateError: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...storybookMswParameters({
      workspace: [
        compozyApiMock.post("/api/workspaces", () =>
          HttpResponse.json({ error: "root is already registered" }, { status: 409 })
        ),
      ],
    }),
  },
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const useCurrent = await canvas.findByTestId("workspace-setup-browser-use-current");
    await waitFor(() => expect(useCurrent).toBeEnabled());
    await userEvent.click(useCurrent);
    await userEvent.click(canvas.getByTestId("workspace-setup-submit"));
    await expect(await canvas.findByTestId("workspace-setup-error")).toBeVisible();
  },
};
