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

/** The daemon refuses to read the typed folder, so the field explains why. */
const browseUnreadable = storybookMswParameters({
  workspace: [
    compozyApiMock.get("/api/fs/browse", ({ request }) =>
      new URL(request.url).searchParams.get("path")
        ? HttpResponse.json({ error: "permission denied" }, { status: 403 })
        : HttpResponse.json({
            entries: [],
            home: "/Users/pedronauck",
            path: "/Users/pedronauck",
            roots: ["/"],
          })
    ),
  ],
});

/** Types a root and waits until the daemon validates it. */
async function chooseRoot(canvasElement: HTMLElement, path = "~/Dev") {
  const canvas = within(canvasElement.ownerDocument.body);
  await userEvent.type(await canvas.findByTestId("workspace-setup-root-input"), path);
  await waitFor(() => expect(canvas.getByTestId("workspace-setup-submit")).toBeEnabled());
  return canvas;
}

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

/** A root typed with `~`, validated and ready for the single create on submit. */
export const SetupDialogRootSelected: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = await chooseRoot(canvasElement);
    await expect(canvas.getByTestId("workspace-setup-name-input")).toHaveValue("Dev");
  },
};

/** Typing completes folders from the daemon listing. */
export const SetupDialogRootAutocomplete: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(await canvas.findByTestId("workspace-setup-root-input"), "~/D");
    await expect(await canvas.findByRole("option", { name: "Dev" })).toBeVisible();
  },
};

/** An unreadable folder blocks submit with an inline reason. */
export const SetupDialogRootInvalid: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: browseUnreadable,
  render: () => dialogHarness("p-10"),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(await canvas.findByTestId("workspace-setup-root-input"), "~/locked");
    await expect(await canvas.findByTestId("workspace-setup-root-error")).toBeVisible();
    await expect(canvas.getByTestId("workspace-setup-submit")).toBeDisabled();
  },
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
    const canvas = await chooseRoot(canvasElement);
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
    const canvas = await chooseRoot(canvasElement);
    await userEvent.click(canvas.getByTestId("workspace-setup-submit"));
    await expect(await canvas.findByTestId("workspace-setup-error")).toBeVisible();
  },
};
