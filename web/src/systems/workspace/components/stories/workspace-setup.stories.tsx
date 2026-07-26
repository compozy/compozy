import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { aghApiMock } from "@/storybook/openapi-msw";

import { storybookMswParameters } from "@/storybook/msw";
import { StorySurface } from "@/storybook/story-layout";
import { statusFixture } from "@/systems/status/mocks/fixtures";
import { primaryWorkspaceFixture } from "@/systems/workspace/mocks/fixtures";

import { useWorkspaceSetupContent } from "../../hooks/use-workspace-setup-content";
import type { WorkspaceSetupDefaultsModel } from "../../lib/workspace-setup-defaults";
import { WorkspaceOnboarding, WorkspaceSetupDialog } from "../workspace-setup";

const storyDefaults: WorkspaceSetupDefaultsModel = {
  agents: { state: "ready", entries: [] },
  sandboxes: { state: "ready", entries: [] },
};

const meta: Meta<typeof WorkspaceOnboarding> = {
  title: "systems/workspace/components/WorkspaceSetup",
  component: WorkspaceOnboarding,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

type DialogStory = StoryObj<typeof WorkspaceSetupDialog>;

function WorkspaceOnboardingHarness({
  onWorkspaceResolved,
}: {
  onWorkspaceResolved: (workspaceId: string) => void;
}) {
  const setup = useWorkspaceSetupContent({ onWorkspaceResolved });
  return <WorkspaceOnboarding model={{ defaults: storyDefaults, setup }} />;
}

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

function onboardingHarness() {
  return (
    <StorySurface className="p-0">
      <WorkspaceOnboardingHarness onWorkspaceResolved={() => undefined} />
    </StorySurface>
  );
}

/** Stalls the directory browse so the reading state stays on screen. */
const browseLoading = storybookMswParameters({
  workspace: [
    aghApiMock.get("/api/fs/browse", async () => {
      await delay("infinite");
      return HttpResponse.json({ entries: [], home: "/Users/pedro", path: "/Users/pedro" });
    }),
  ],
});

const browseEmpty = storybookMswParameters({
  workspace: [
    aghApiMock.get("/api/fs/browse", () =>
      HttpResponse.json({
        entries: [],
        home: "/Users/pedro",
        parent: "/Users",
        path: "/Users/pedro",
      })
    ),
  ],
});

const browseError = storybookMswParameters({
  workspace: [
    aghApiMock.get("/api/fs/browse", () =>
      HttpResponse.json({ error: "permission denied" }, { status: 403 })
    ),
  ],
});

export const OnboardingDefault: Story = {
  args: {},
  render: onboardingHarness,
};

export const OnboardingGlobalUnavailable: Story = {
  args: {},
  parameters: {
    ...storybookMswParameters({
      daemon: [
        aghApiMock.get("/api/status", () =>
          HttpResponse.json({
            ...statusFixture,
            daemon: { ...statusFixture.daemon, user_home_dir: "" },
          })
        ),
      ],
    }),
  },
  render: onboardingHarness,
};

/** VC-03 capture target: the xl split shell at desktop width. */
export const SetupDialogOpen: DialogStory = {
  args: {},
  render: () => dialogHarness("p-10"),
};

/** Below 980px the split body collapses and session defaults stack underneath. */
export const SetupDialogStacked: DialogStory = {
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
export const SetupDialogRootSelected: DialogStory = {
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

export const SetupDialogBrowserLoading: DialogStory = {
  args: {},
  parameters: browseLoading,
  render: () => dialogHarness("p-10"),
};

export const SetupDialogBrowserEmpty: DialogStory = {
  args: {},
  parameters: browseEmpty,
  render: () => dialogHarness("p-10"),
};

export const SetupDialogBrowserError: DialogStory = {
  args: {},
  parameters: browseError,
  render: () => dialogHarness("p-10"),
};

export const OnboardingMobile: Story = {
  args: {},
  parameters: {
    viewport: { defaultViewport: "iphone14" },
    docs: {
      description: {
        story:
          "Below `lg` breakpoint the two-column hero collapses to a single rail so onboarding remains tappable on mobile.",
      },
    },
  },
  render: onboardingHarness,
};

export const OnboardingLoadingGlobal: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...storybookMswParameters({
      workspace: [
        aghApiMock.post("/api/workspaces/resolve", async () => {
          await delay("infinite");
          return HttpResponse.json({ workspace: primaryWorkspaceFixture });
        }),
      ],
    }),
    docs: {
      description: {
        story:
          "Drives the global-default CTA into its loading state by stalling the resolve call indefinitely.",
      },
    },
  },
  render: onboardingHarness,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("workspace-use-global"));
    await expect(canvas.getByTestId("workspace-use-global")).toBeDisabled();
  },
};

/** Registration spinner: stall `POST /api/workspaces` after picking a root. */
export const OnboardingLoadingCreate: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...storybookMswParameters({
      workspace: [
        aghApiMock.post("/api/workspaces", async () => {
          await delay("infinite");
          return HttpResponse.json({ workspace: primaryWorkspaceFixture }, { status: 201 });
        }),
      ],
    }),
  },
  render: onboardingHarness,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("workspace-setup-browser-use-current"));
    await userEvent.click(canvas.getByTestId("workspace-setup-onboarding-submit"));
    await expect(canvas.getByTestId("workspace-setup-onboarding-submit")).toBeDisabled();
  },
};

/** Registration failure keeps the draft and reports inline. */
export const OnboardingCreateError: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...storybookMswParameters({
      workspace: [
        aghApiMock.post("/api/workspaces", () =>
          HttpResponse.json({ error: "root is already registered" }, { status: 409 })
        ),
      ],
    }),
  },
  render: onboardingHarness,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("workspace-setup-browser-use-current"));
    await userEvent.click(canvas.getByTestId("workspace-setup-onboarding-submit"));
    await expect(await canvas.findByTestId("workspace-setup-error")).toBeVisible();
  },
};

export const UseGlobalWorkspace: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => {
    function Harness() {
      const [status, setStatus] = useState("");
      return (
        <StorySurface className="p-0">
          <WorkspaceOnboardingHarness onWorkspaceResolved={id => setStatus(`resolved:${id}`)} />
          <div data-testid="resolve-status" className="sr-only">
            {status}
          </div>
        </StorySurface>
      );
    }

    return <Harness />;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("workspace-use-global"));
  },
};
