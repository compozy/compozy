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

import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { OsShellContext, type OsShellHandle } from "../../contexts/os-shell-context";
import type { CmdPaletteDispatch } from "../../hooks/use-cmd-palette-dispatch";
import { RoutingCoordinator, type OsRouterPort } from "../../lib/routing-coordinator";
import { buildPaletteRegistry } from "../../lib/cmd-palette-registry";
import type { PaletteRegistry } from "../../lib/cmd-palette-types";
import {
  cmdPaletteArgumentsCommand,
  cmdPaletteDestructiveCommand,
  cmdPaletteExecutionCatalog,
} from "../../mocks/cmd-palette-fixtures";
import { WindowManagerRuntime } from "../../runtime/window-manager-runtime";
import {
  cmdPaletteExecutionStore,
  requestPaletteArgs,
  requestPaletteConfirmation,
  resetPaletteExecutionEntry,
} from "../../stores/cmd-palette-execution-store";
import { OsCommandPalette } from "../os-command-palette";
import { DesktopShell } from "./_desktop";

/**
 * The execution surfaces the palette gains at P3: the ⌘K action panel, the
 * inline argument bar, the declared confirmation, and the in-palette pending
 * affordance. Each story renders the production component through the same
 * registry projection the shell builds, so what appears here is what the daemon
 * would actually serve.
 */
const PALETTE_SESSIONS = sessionFixtures.filter(
  session => session.workspace_id === storyDefaultWorkspaceId
);

const EXECUTION_REGISTRY: PaletteRegistry = buildPaletteRegistry({
  catalog: {
    commands: cmdPaletteExecutionCatalog().commands,
    sources: cmdPaletteExecutionCatalog().sources,
    catalogRevision: cmdPaletteExecutionCatalog().catalog_revision,
  },
  context: null,
  daemonReachable: true,
  stale: false,
  platform: "MacIntel",
});

/** Fails loudly rather than seeding a step with an id the fixture no longer has. */
function executionCommand(id: string) {
  const command = EXECUTION_REGISTRY.byId.get(id);
  if (command === undefined) throw new Error(`Execution fixture is missing ${id}`);
  return command;
}

/** A seam stub: the stories exercise the surfaces, not the daemon round-trip. */
const STORY_DISPATCH: CmdPaletteDispatch = {
  run: async () => ({ status: "ran" }),
  runById: async () => ({ status: "ran" }),
  executeClientOp: async () => undefined,
  setPinned: async () => undefined,
};

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

function paletteHandlers() {
  return storybookMswParameters({
    workspace: [
      compozyApiMock.get("/api/workspaces", () =>
        HttpResponse.json({ workspaces: workspaceFixtures })
      ),
    ],
    session: [
      compozyApiMock.get("/api/sessions", () =>
        HttpResponse.json({
          sessions: PALETTE_SESSIONS,
          page: { has_more: false, limit: 100, total: PALETTE_SESSIONS.length },
        })
      ),
    ],
  });
}

const meta: Meta<typeof OsCommandPalette> = {
  title: "systems/os/components/OsPaletteExecution",
  component: OsCommandPalette,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Action panel, inline arguments, declared confirmation and pending feedback, rendered from the one registry projection over the OS shell.",
      },
    },
  },
  loaders: [
    async () => {
      await rehydrateActiveWorkspaceStore();
      setActiveWorkspaceId(storyDefaultWorkspaceId);
      resetPaletteExecutionEntry();
      return {};
    },
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function renderPalette(args: React.ComponentProps<typeof OsCommandPalette>) {
  return (
    <OsShellContext.Provider value={STORY_SHELL}>
      <CmdPaletteRegistryProvider registry={EXECUTION_REGISTRY}>
        <SessionCreateProvider store={STORY_SESSION_CREATE_STORE}>
          <DesktopShell wallpaper="ember" deskHint>
            <OsCommandPalette {...args} />
          </DesktopShell>
        </SessionCreateProvider>
      </CmdPaletteRegistryProvider>
    </OsShellContext.Provider>
  );
}

const OPEN_ARGS = { open: true, onOpenChange: fn(), dispatch: STORY_DISPATCH };

/** ⌘K on the selected command row: sections, per-action chords, primary marked ↩. */
export const ActionPanel: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await canvas.findByTestId("os-command-palette");
    await userEvent.keyboard("{Meta>}k{/Meta}");
    await canvas.findByTestId("os-palette-action-panel");
  },
};

/** Typing narrows both sections; the primary marker stays on a visible row. */
export const ActionPanelFiltered: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await canvas.findByTestId("os-command-palette");
    await userEvent.keyboard("{Meta>}k{/Meta}");
    await userEvent.type(await canvas.findByPlaceholderText("Filter actions…"), "pin");
  },
};

/** A filter that matches nothing stays open and says so — it invents no fallback. */
export const ActionPanelFilteredEmpty: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await canvas.findByTestId("os-command-palette");
    await userEvent.keyboard("{Meta>}k{/Meta}");
    await userEvent.type(await canvas.findByPlaceholderText("Filter actions…"), "xyz");
  },
};

/** An unavailable row: meta-actions plus the runtime's reason, nothing runnable. */
export const ActionPanelUnavailableRow: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.hover(await canvas.findByTestId("os-palette-command-ext.notes.capture"));
    await userEvent.keyboard("{Meta>}k{/Meta}");
    await canvas.findByTestId("os-palette-action-reason");
  },
};

/** The input bar as typed fields, pristine — the state a bound hotkey opens into. */
export const ArgumentsPristine: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      requestPaletteArgs(executionCommand(cmdPaletteArgumentsCommand.id));
      return {};
    },
  ],
  render: renderPalette,
};

/** Filled fields: a typed title and a chosen tag. */
export const ArgumentsFilled: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      requestPaletteArgs(executionCommand(cmdPaletteArgumentsCommand.id));
      return {};
    },
  ],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(await canvas.findByTestId("os-palette-arg-title"), "Standup follow-ups");
    await userEvent.type(await canvas.findByTestId("os-palette-arg-tag"), "inbox");
  },
};

/** ⏎ with the required field empty: blocked, focused, and told why. */
export const ArgumentsRequiredMissing: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      requestPaletteArgs(executionCommand(cmdPaletteArgumentsCommand.id));
      return {};
    },
  ],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("os-palette-arg-title"));
    await userEvent.keyboard("{Enter}");
    await canvas.findByTestId("os-palette-arg-error-title");
  },
};

/** The dropdown open on a partial query, narrowed to what still matches. */
export const ArgumentsDropdownOpen: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      requestPaletteArgs(executionCommand(cmdPaletteArgumentsCommand.id));
      return {};
    },
  ],
  tags: ["play-fn"],
  render: renderPalette,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("os-palette-arg-tag"));
    await userEvent.keyboard("in");
    await canvas.findByTestId("os-palette-arg-options-tag");
  },
};

/** The declared confirmation: the command's own words, Cancel focused. */
export const ConfirmationDestructive: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      requestPaletteConfirmation(executionCommand(cmdPaletteDestructiveCommand.id), {});
      return {};
    },
  ],
  render: renderPalette,
};

/** The target moved between trigger and confirm: honest message, no confirm control. */
export const ConfirmationInvalidated: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      const live = executionCommand(cmdPaletteDestructiveCommand.id);
      requestPaletteConfirmation(
        {
          ...live,
          available: false,
          reason: "target changed — archived notes are no longer in this workspace",
        },
        {}
      );
      return {};
    },
  ],
  render: renderPalette,
};

/** A daemon invocation in flight: motion token and the literal word, no percentage. */
export const PendingInvocation: Story = {
  args: OPEN_ARGS,
  parameters: paletteHandlers(),
  loaders: [
    async () => {
      cmdPaletteExecutionStore.trigger.pendingStarted({
        pending: { commandId: "session.new", title: "New session" },
      });
      return {};
    },
  ],
  render: renderPalette,
};
