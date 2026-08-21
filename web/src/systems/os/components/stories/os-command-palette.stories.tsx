import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient } from "@tanstack/react-query";
import { HttpResponse } from "msw";
import { useState } from "react";
import { fireEvent, fn, userEvent, within } from "storybook/test";

import {
  closeProfileDialog,
  openProfileDialog,
  ProfileLifecycleDialogs,
  resetProfileViews,
  setProfileView,
} from "@/systems/profiles";
import { profileFixtures } from "@/systems/profiles/mocks";
import { createSessionCreateStore, SessionCreateProvider } from "@/systems/session";
import { sessionFixtures } from "@/systems/session/mocks";
import {
  enableGlobalScope,
  rehydrateActiveWorkspaceStore,
  setActiveWorkspaceId,
} from "@/systems/workspace";
import { workspaceFixtures } from "@/systems/workspace/mocks";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import type { CmdPaletteDispatch } from "../../hooks/use-cmd-palette-dispatch";
import type { CmdPaletteContextSnapshot } from "../../lib/cmd-palette-context";
import { buildPaletteRegistry } from "../../lib/cmd-palette-registry";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import type { PaletteViewFrame } from "../../lib/palette-view-stack";
import {
  cmdPaletteCatalogFixture,
  cmdPaletteProfileExtensionCommand,
} from "../../mocks/cmd-palette-fixtures";
import { windowManagerStore } from "../../stores/window-manager-store";
import { OsCommandPalette } from "../os-command-palette";
import { DesktopShell } from "./_desktop";

const PALETTE_SESSIONS = sessionFixtures.filter(
  session => session.workspace_id === storyDefaultWorkspaceId
);

const AGGREGATE_PALETTE_SESSIONS: SessionFixture[] = sessionFixtures
  .slice(0, 6)
  .map((session, index) => {
    if (index === 1 || index === 4) {
      return {
        ...session,
        profile_id: "01J9MARKETING00000000000000",
        profile_name: "marketing",
        profile_color: "#c26ad6",
        profile_icon: "megaphone",
        profile_archived: false,
      };
    }
    if (index === 5) {
      return {
        ...session,
        profile_id: "01J9OLDAGENCY0000000000000",
        profile_name: "old-agency",
        profile_color: "#b58e5f",
        profile_icon: "folder",
        profile_archived: true,
      };
    }
    return {
      ...session,
      profile_color: "#8a8f98",
      profile_icon: "user-round",
      profile_archived: false,
    };
  });

type SessionFixture = (typeof sessionFixtures)[number];
type SessionListScope = "workspace" | "all-workspaces";

function shellSettings(scope: SessionListScope) {
  return {
    available_scopes: ["user" as const],
    config: { sessions: { scope, sort: "last_activity" as const } },
    scope: "user" as const,
    section: "shell" as const,
  };
}

function paletteHandlers(sessions: SessionFixture[], scope: SessionListScope = "workspace") {
  return storybookMswParameters({
    workspace: [
      compozyApiMock.get("/api/workspaces", () =>
        HttpResponse.json({ workspaces: workspaceFixtures })
      ),
    ],
    settings: [
      compozyApiMock.get("/api/settings/shell", () => HttpResponse.json(shellSettings(scope))),
    ],
    session: [
      compozyApiMock.get("/api/sessions", ({ request }) => {
        const workspace = new URL(request.url).searchParams.get("workspace_id")?.trim();
        const scoped =
          workspace === undefined
            ? sessions
            : sessions.filter(
                session =>
                  session.workspace_id === workspace || session.workspace_path === workspace
              );
        return HttpResponse.json({
          sessions: scoped,
          page: { has_more: false, limit: 100, total: scoped.length },
        });
      }),
    ],
  });
}

/** Seeds the shell's view path so a story can open at a given level. */
function paletteViewLoader(stack: readonly PaletteViewFrame[]) {
  return async () => {
    windowManagerStore.trigger.paletteViewStackSet({ stack });
    return {};
  };
}

const SCALE_SESSIONS: SessionFixture[] = Array.from({ length: 240 }, (_, index) => ({
  ...PALETTE_SESSIONS[index % PALETTE_SESSIONS.length],
  id: `session:scale-${index}`,
  name: `Scale session ${index}`,
}));

function createStoryShell(): OsShellHandle {
  const manager = new WindowManagerRuntime(new QueryClient());
  const router: OsRouterPort = { navigate: () => {}, replace: () => {} };
  return {
    projection: manager.projectionAtom,
    manager,
    coordinator: new RoutingCoordinator(manager, router),
  };
}

const STORY_SHELL = createStoryShell();
const STORY_SESSION_CREATE_STORE = createSessionCreateStore();
const STORY_DISPATCH: CmdPaletteDispatch = {
  run: async () => ({ status: "ran" }),
  runById: async () => ({ status: "ran" }),
  executeClientOp: async () => undefined,
  setPinned: async () => undefined,
};

const STORY_DESKTOP_CONTEXT: CmdPaletteContextSnapshot = {
  "window.focused": true,
  "window.floating": false,
  "window.stacked": false,
  "desktop.windowCount": 1,
  "scope.global": false,
  "shell.desktop": true,
  "session.focused.state": "idle",
  "workspace.trusted": true,
};

const STORY_REGISTRY = buildPaletteRegistry({
  catalog: {
    commands: cmdPaletteCatalogFixture.commands,
    sources: cmdPaletteCatalogFixture.sources,
    catalogRevision: cmdPaletteCatalogFixture.catalog_revision,
  },
  context: STORY_DESKTOP_CONTEXT,
  daemonReachable: true,
  stale: false,
  platform: "MacIntel",
});

const PROFILE_EXTENSION_REGISTRY = buildPaletteRegistry({
  catalog: {
    commands: [...cmdPaletteCatalogFixture.commands, cmdPaletteProfileExtensionCommand],
    sources: [
      ...cmdPaletteCatalogFixture.sources,
      { source: "ext.campaigns", status: "healthy" as const },
    ],
    catalogRevision: "sha256:story-profile-extension",
  },
  context: STORY_DESKTOP_CONTEXT,
  daemonReachable: true,
  stale: false,
  platform: "MacIntel",
});

const meta: Meta<typeof OsCommandPalette> = {
  title: "systems/os/components/OsCommandPalette",
  component: OsCommandPalette,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The real desktop command palette rendered over the OS shell with deterministic workspace and session responses. These stories are the canonical palette visual-contract surfaces.",
      },
    },
  },
  loaders: [
    async () => {
      await rehydrateActiveWorkspaceStore();
      setActiveWorkspaceId(storyDefaultWorkspaceId);
      resetProfileViews();
      closeProfileDialog();
      return {};
    },
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function PaletteStory({
  args,
  profileExtension = false,
}: {
  args: React.ComponentProps<typeof OsCommandPalette>;
  profileExtension?: boolean;
}) {
  return (
    <OsShellContext.Provider value={STORY_SHELL}>
      <CmdPaletteRegistryProvider
        registry={profileExtension ? PROFILE_EXTENSION_REGISTRY : STORY_REGISTRY}
      >
        <SessionCreateProvider store={STORY_SESSION_CREATE_STORE}>
          <DesktopShell wallpaper="ember" deskHint>
            <OsCommandPalette {...args} />
          </DesktopShell>
        </SessionCreateProvider>
      </CmdPaletteRegistryProvider>
    </OsShellContext.Provider>
  );
}

function renderPalette(args: React.ComponentProps<typeof OsCommandPalette>) {
  return <PaletteStory args={args} />;
}

function PaletteLifecycleHandoffStory(args: React.ComponentProps<typeof OsCommandPalette>) {
  const [open, setOpen] = useState(true);
  const dispatch: CmdPaletteDispatch = {
    ...STORY_DISPATCH,
    run: async command => {
      if (command.id === "profile.archive") {
        openProfileDialog({ flow: "archive", profile: "marketing" });
        setOpen(false);
      }
      return { status: "ran" };
    },
  };
  return (
    <OsShellContext.Provider value={STORY_SHELL}>
      <CmdPaletteRegistryProvider registry={STORY_REGISTRY}>
        <SessionCreateProvider store={STORY_SESSION_CREATE_STORE}>
          <DesktopShell wallpaper="ember" deskHint>
            <OsCommandPalette {...args} open={open} onOpenChange={setOpen} dispatch={dispatch} />
            <ProfileLifecycleDialogs
              profiles={profileFixtures}
              lens={{ scope: "workspace", workspaceId: storyDefaultWorkspaceId }}
              onSetAutomationEnabled={async () => undefined}
            />
          </DesktopShell>
        </SessionCreateProvider>
      </CmdPaletteRegistryProvider>
    </OsShellContext.Provider>
  );
}

const OPEN_ARGS = { open: true, onOpenChange: fn(), dispatch: STORY_DISPATCH };

/**
 * Open — production palette at its root, real app registry, live query hooks,
 * and a deterministic active workspace/session catalog over the resting
 * desktop. The Views group is where a nested view is entered.
 */
export const Open: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([])],
  render: renderPalette,
};

/** The Sessions view pushed: state chips, attention-first rows, scope. */
export const SessionsView: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([{ viewId: "sessions" }])],
  render: renderPalette,
};

/** A view with nothing to list still keeps its breadcrumb and keyboard contract. */
export const SessionsViewEmpty: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers([]),
  loaders: [paletteViewLoader([{ viewId: "sessions" }])],
  render: renderPalette,
};

/** A state chip with no matches names the filter and remains one Backspace from the full list. */
export const SessionsViewZeroMatch: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([{ viewId: "sessions" }])],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("os-palette-session-filter-working"));
  },
};

/** Show all: the operator's persisted scope widened to every workspace. */
export const SessionsViewAllWorkspaces: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(sessionFixtures, "all-workspaces"),
  loaders: [paletteViewLoader([{ viewId: "sessions" }])],
  render: renderPalette,
};

/** Hundreds of sessions: bounded rows, truthful counts, honest overflow note. */
export const SessionsViewAtScale: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(SCALE_SESSIONS),
  loaders: [paletteViewLoader([{ viewId: "sessions" }])],
  render: renderPalette,
};

/** The profile catalog view: current, needs-setup and archived rows plus lifecycle entry points. */
export const ProfilesView: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([{ viewId: "profiles" }])],
  render: renderPalette,
};

/** The Profiles view keeps an unavailable row visible with its typed runtime reason. */
export const ProfilesViewUnavailable: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([{ viewId: "profiles" }])],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    fireEvent.change(await canvas.findByPlaceholderText("Switch profile…"), {
      target: { value: "growth" },
    });
    await canvas.findByText("needs setup");
  },
};

/** Root search exposes the stable profile command ids. */
export const RootProfileSearch: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([])],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    fireEvent.change(await canvas.findByPlaceholderText("Search apps, sessions, and actions…"), {
      target: { value: "profile" },
    });
    await canvas.findByTestId("os-palette-command-profile.create");
  },
};

/** A lifecycle command leaves the palette and raises the shell's one canonical profile dialog. */
export const ProfileLifecycleHandoff: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([])],
  tags: ["play-fn"],
  render: args => <PaletteLifecycleHandoffStory {...args} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(
      await canvas.findByPlaceholderText("Search apps, sessions, and actions…"),
      "archive profile"
    );
    await userEvent.click(await canvas.findByTestId("os-palette-command-profile.archive"));
    await canvas.findByTestId("profile-archive-dialog");
  },
};

/** Global and All profiles compose in the real Sessions palette view with owner-labeled rows. */
export const AllProfilesGlobal: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(AGGREGATE_PALETTE_SESSIONS, "all-workspaces"),
  loaders: [
    async () => {
      enableGlobalScope();
      setProfileView({ scope: "global" }, { kind: "aggregate" });
      windowManagerStore.trigger.paletteViewStackSet({ stack: [{ viewId: "sessions" }] });
      return {};
    },
  ],
  render: renderPalette,
};

/** A profile-bound extension contribution appears in the profile catalog that owns it. */
export const ProfileExtensionContribution: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([])],
  tags: ["play-fn"],
  render: args => <PaletteStory args={args} profileExtension />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    fireEvent.change(await canvas.findByPlaceholderText("Search apps, sessions, and actions…"), {
      target: { value: "campaign" },
    });
    await canvas.findByTestId("os-palette-command-ext.campaigns.draft");
  },
};

/** The same query in a profile without the contribution returns no command row. */
export const ProfileExtensionContributionAbsent: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [paletteViewLoader([])],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    fireEvent.change(await canvas.findByPlaceholderText("Search apps, sessions, and actions…"), {
      target: { value: "campaign" },
    });
    await canvas.findByTestId("os-palette-empty");
  },
};

/**
 * Three levels deep. Only one view is registered in v1, so the depth is seeded
 * through the store rather than reachable by clicking — the breadcrumb's
 * left-truncation is the thing on display.
 */
export const ViewStackDepth: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(PALETTE_SESSIONS),
  loaders: [
    paletteViewLoader([{ viewId: "sessions" }, { viewId: "sessions" }, { viewId: "sessions" }]),
  ],
  render: renderPalette,
};
