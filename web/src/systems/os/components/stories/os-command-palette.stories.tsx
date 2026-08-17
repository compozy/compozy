import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient } from "@tanstack/react-query";
import { HttpResponse } from "msw";
import { fn, userEvent, within } from "storybook/test";

import { createSessionCreateStore, SessionCreateProvider } from "@/systems/session";
import { sessionFixtures } from "@/systems/session/mocks";
import { rehydrateActiveWorkspaceStore, setActiveWorkspaceId } from "@/systems/workspace";
import { workspaceFixtures } from "@/systems/workspace/mocks";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";

import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import type { PaletteViewFrame } from "../../lib/palette-view-stack";
import { windowManagerStore } from "../../stores/window-manager-store";
import { OsCommandPalette } from "../os-command-palette";
import { DesktopShell } from "./_desktop";

const PALETTE_SESSIONS = sessionFixtures.filter(
  session => session.workspace_id === storyDefaultWorkspaceId
);

type SessionFixture = (typeof sessionFixtures)[number];
type SessionListScope = "workspace" | "all-workspaces";

function shellSettings(scope: SessionListScope) {
  return {
    available_scopes: ["global" as const],
    config: { sessions: { scope, sort: "last_activity" as const } },
    scope: "global" as const,
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
        const workspace = new URL(request.url).searchParams.get("workspace")?.trim();
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
      return {};
    },
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function renderPalette(args: React.ComponentProps<typeof OsCommandPalette>) {
  return (
    <OsShellContext.Provider value={STORY_SHELL}>
      <SessionCreateProvider store={STORY_SESSION_CREATE_STORE}>
        <DesktopShell wallpaper="ember" deskHint>
          <OsCommandPalette {...args} />
        </DesktopShell>
      </SessionCreateProvider>
    </OsShellContext.Provider>
  );
}

const OPEN_ARGS = { open: true, onOpenChange: fn() };

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
