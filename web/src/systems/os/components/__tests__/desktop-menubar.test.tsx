// Suite: DesktopMenubar scope-control wiring
// Invariant: while scope resolution is pending the globe control is
// aria-disabled, matching the runtime-workspace query lock at the root; once
// settled it is never locked, so zero projects can still turn All projects off.
// The command-palette control carries the live palette chord as
// aria-keyshortcuts once the keymap is known, and nothing before. The tray's
// All desktops control toggles the desktops overlay; Settings is not a tray
// control (it lives in the system menu). The Provider usage control sits
// right before the bell while the profile has usage targets, and holds that
// slot in its loading shape while the first usage read is pending; opening
// it asks for a background refresh of stale rows, and its footer opens the
// Usage app and the Provider usage settings (or their routes in a degraded
// shell).
// Owning layer: desktop-menubar.tsx. Canonical suite: this file.
// Boundary: daemon reads and refreshes run over a stubbed `fetch`.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { toast, UIProvider } from "@compozy/ui";
import { resetProfileViews } from "@/systems/profiles";
import type { ProviderUsageEnvelope } from "@/systems/provider-usage";
import type { WorkspacePayload, WorktreeListingByWorkspace } from "@/systems/workspace";
import { primaryWorkspaceFixture, worktreeListingFixture } from "@/systems/workspace/mocks";
import {
  codexOkTarget,
  statusTargets,
  usageEnvelope,
  usageTarget,
  usageTray,
} from "@/systems/provider-usage/mocks";

import type { OsAttentionModel } from "../../hooks/use-os-attention";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { paletteRegistryFixture, resolvedPaletteCommand } from "../../mocks/cmd-palette-fixtures";
import type { ResolvedPaletteCommand } from "../../lib/cmd-palette-types";
import type { DesktopOverlay } from "../../hooks/use-desktop-overlays";
import { DesktopMenubar } from "../desktop-menubar";

const shell = vi.hoisted(() => ({
  hydration: "live" as "live" | "loading",
  coordinator: { userOpen: vi.fn(), userNavigate: vi.fn() },
}));

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: object) => unknown) =>
    selector({
      hydration: shell.hydration,
      snapshot: {},
      client: {},
      windowManagerConfig: {},
    }),
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: shell.coordinator }),
}));

vi.mock("../../hooks/use-menubar-actions", () => ({
  useMenubarActions: () => ({
    menusVisible: false,
    canOpenApps: false,
    windowCommands: {},
    openApp: vi.fn(),
    openUpdates: vi.fn(),
    newAgent: vi.fn(),
  }),
}));

vi.mock("../../hooks/use-attention-jump", () => ({
  useAttentionJump: () => vi.fn(),
}));

vi.mock("../../hooks/use-open-target", () => ({
  useOpenTarget: () => vi.fn(),
}));

const ATTENTION: OsAttentionModel = {
  badges: {},
  notificationCount: 0,
  sections: { needsYou: [], finished: [] },
  sessions: [],
  attentionSessionsDisconnected: false,
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loopRequestsDisconnected: false,
  loading: false,
};

const NOW = Date.now();
const SECOND = 1_000;

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

/** The daemon over `fetch`: usage reads and refreshes answer `envelope`. */
function serveUsage(envelope: ProviderUsageEnvelope) {
  vi.mocked(globalThis.fetch).mockImplementation(input => {
    const url = input instanceof Request ? input.url : String(input);
    if (url.includes("/api/provider-usage")) return Promise.resolve(jsonResponse(envelope));
    return Promise.resolve(jsonResponse({ error: "not found", code: "not_found" }, 404));
  });
}

function usageRequests(): Request[] {
  return vi
    .mocked(globalThis.fetch)
    .mock.calls.map(([input]) => input)
    .filter((input): input is Request => input instanceof Request)
    .filter(request => request.url.includes("/api/provider-usage"));
}

const codexWeekly = usageEnvelope([codexOkTarget(NOW)], { tray: usageTray({}, NOW) }, NOW);

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
  serveUsage(usageEnvelope([], { enabled: false }, NOW));
  shell.hydration = "live";
  shell.coordinator.userOpen.mockReset();
  shell.coordinator.userNavigate.mockReset();
});

afterEach(() => {
  vi.unstubAllGlobals();
  resetProfileViews();
  vi.restoreAllMocks();
});

function renderMenubar({
  commands = [],
  scopePending = false,
  breadthMode = "focused",
  onToggleAllProjects = vi.fn(),
  activeOverlay = null,
  onOverlayOpenChange = vi.fn(),
  usage,
  workspaces = [],
  worktreesByWorkspace,
}: {
  commands?: readonly ResolvedPaletteCommand[];
  scopePending?: boolean;
  breadthMode?: "all" | "focused";
  onToggleAllProjects?: () => void;
  activeOverlay?: DesktopOverlay | null;
  onOverlayOpenChange?: (overlay: DesktopOverlay, open: boolean) => void;
  /** Cached and served usage envelope for the default profile. */
  usage?: ProviderUsageEnvelope;
  workspaces?: WorkspacePayload[];
  /** Gives git-backed project rows their worktree submenu. */
  worktreesByWorkspace?: WorktreeListingByWorkspace;
} = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { enabled: false, retry: false } },
  });
  if (usage) {
    serveUsage(usage);
    queryClient.setQueryData(["provider-usage", "envelope", "default"], usage);
  }
  const tree = (listed: WorkspacePayload[], overlay = activeOverlay) => (
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <CmdPaletteRegistryProvider registry={paletteRegistryFixture(commands)}>
          <DesktopMenubar
            workspaces={listed}
            activeWorkspace={undefined}
            breadthMode={breadthMode}
            focus={{ kind: "user" }}
            scopePending={scopePending}
            onToggleAllProjects={onToggleAllProjects}
            onSelectWorkspace={vi.fn()}
            onAddWorkspace={vi.fn()}
            onRunCommand={vi.fn()}
            activeOverlay={overlay}
            onOverlayOpenChange={onOverlayOpenChange}
            attention={ATTENTION}
            update={{ state: "hidden" }}
            worktreesByWorkspace={worktreesByWorkspace}
            onCreateWorktree={worktreesByWorkspace ? vi.fn() : undefined}
          />
        </CmdPaletteRegistryProvider>
      </UIProvider>
    </QueryClientProvider>
  );
  const view = render(tree(workspaces));
  return {
    ...view,
    /** The catalog answers a new project list (archived or removed elsewhere). */
    setWorkspaces: (next: WorkspacePayload[]) => view.rerender(tree(next)),
    setOverlay: (next: DesktopOverlay | null) => view.rerender(tree(workspaces, next)),
  };
}

// #1029: a menubar menu cannot nest a context menu, so a context click on a project row closes
// the workspace menu and the host opens that project's actions at the point; the confirm
// outlives the menu that opened it.
describe("DesktopMenubar project context menu", () => {
  it("Should restore reachable focus after the source menu closes and the project menu is dismissed", async () => {
    const user = userEvent.setup();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    const { setOverlay } = renderMenubar({
      workspaces: [project],
      activeOverlay: "workspace-menu",
    });
    const row = await screen.findByTestId(`os-workspace-option-${project.id}`);
    act(() => row.focus());
    fireEvent.keyDown(row, { key: "F10", shiftKey: true });
    setOverlay(null);
    expect(await screen.findByTestId(`workspace-project-menu-${project.id}`)).toBeVisible();
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByTestId(`workspace-project-menu-${project.id}`)).toBeNull()
    );
    expect(document.querySelector('[data-slot="os-menubar-workspace"]')).toHaveFocus();
  });

  it("Should relay a project row's context click to that project's actions", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    renderMenubar({ workspaces: [project], activeOverlay: "workspace-menu", onOverlayOpenChange });

    const row = await screen.findByTestId(`os-workspace-option-${project.id}`);
    // Shift+F10 on the focused row relays the same click from the keyboard.
    fireEvent.keyDown(row, { key: "F10", shiftKey: true });
    expect(onOverlayOpenChange).toHaveBeenCalledWith("workspace-menu", false);
    expect(await screen.findByTestId(`workspace-project-menu-${project.id}`)).toBeVisible();
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByTestId(`workspace-project-menu-${project.id}`)).toBeNull()
    );
    expect(fireEvent.contextMenu(row, { clientX: 40, clientY: 60 })).toBe(false);
    const menu = await screen.findByTestId(`workspace-project-menu-${project.id}`);
    await user.click(within(menu).getByRole("menuitem", { name: "Archive project" }));
    expect(await screen.findByRole("dialog", { name: `Archive ${project.name}?` })).toBeVisible();
    expect(screen.queryByTestId(`workspace-project-menu-${project.id}`)).toBeNull();
  });

  it("Should drop the relayed menu and its idle confirm once the project leaves the catalog", async () => {
    const user = userEvent.setup();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    const { setWorkspaces } = renderMenubar({
      workspaces: [project],
      activeOverlay: "workspace-menu",
    });

    const row = await screen.findByTestId(`os-workspace-option-${project.id}`);
    const menuId = `workspace-project-menu-${project.id}`;
    // An open menu closes when its project leaves and stays closed when it returns.
    fireEvent.contextMenu(row, { clientX: 40, clientY: 60 });
    expect(await screen.findByTestId(menuId)).toBeVisible();
    setWorkspaces([]);
    await waitFor(() => expect(screen.queryByTestId(menuId)).toBeNull());
    setWorkspaces([project]);
    expect(screen.queryByTestId(menuId)).toBeNull();

    fireEvent.contextMenu(await screen.findByTestId(`os-workspace-option-${project.id}`), {
      clientX: 40,
      clientY: 60,
    });
    const menu = await screen.findByTestId(menuId);
    await user.click(within(menu).getByRole("menuitem", { name: "Archive project" }));
    expect(await screen.findByRole("dialog", { name: `Archive ${project.name}?` })).toBeVisible();

    // Archived from another window: the idle confirm closes and does not reopen on return.
    setWorkspaces([]);
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    setWorkspaces([project]);
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByTestId(`workspace-project-menu-${project.id}`)).toBeNull();
  });

  it("Should keep a failed removal readable, but not retryable, once its project leaves", async () => {
    const user = userEvent.setup();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    const { setWorkspaces } = renderMenubar({
      workspaces: [project],
      activeOverlay: "workspace-menu",
    });

    fireEvent.contextMenu(await screen.findByTestId(`os-workspace-option-${project.id}`), {
      clientX: 40,
      clientY: 60,
    });
    const menu = await screen.findByTestId(`workspace-project-menu-${project.id}`);
    await user.click(within(menu).getByRole("menuitem", { name: "Remove project" }));
    const dialog = await screen.findByRole("dialog", { name: `Remove ${project.name}?` });
    // The stubbed daemon refuses the removal.
    await user.click(within(dialog).getByRole("button", { name: "Remove project" }));
    const error = await within(dialog).findByRole("alert");

    setWorkspaces([]);
    expect(screen.getByRole("dialog", { name: `Remove ${project.name}?` })).toBeVisible();
    expect(error).toBeVisible();
    // Neither the confirm nor its Archive instead can act on a project the catalog dropped.
    expect(within(dialog).getByRole("button", { name: "Remove project" })).toBeDisabled();
    expect(within(dialog).getByRole("button", { name: "Archive instead" })).toBeDisabled();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});

// #1057: Archive project closes the submenu with the workspace menu, so the host opens the same
// confirm the context menu uses, blockers included, and focus returns to the project trigger.
describe("DesktopMenubar project submenu archive", () => {
  it("Should open the archive confirm from the project submenu and return focus on cancel", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    vi.mocked(globalThis.fetch).mockImplementation(input => {
      const url = input instanceof Request ? input.url : String(input);
      if (url.includes(`/api/workspaces/${project.id}/archive`)) {
        return Promise.resolve(
          jsonResponse(
            {
              error: "project has active work",
              code: "workspace_busy",
              blockers: [{ kind: "session", id: "sess-1", name: "Fix the build" }],
            },
            409
          )
        );
      }
      return Promise.resolve(jsonResponse({ error: "not found", code: "not_found" }, 404));
    });
    const { setOverlay } = renderMenubar({
      workspaces: [project],
      activeOverlay: "workspace-menu",
      onOverlayOpenChange,
      worktreesByWorkspace: { [project.id]: worktreeListingFixture },
    });

    const row = await screen.findByTestId(`os-workspace-option-${project.id}`);
    act(() => row.focus());
    await user.keyboard("{ArrowRight}");
    const submenu = await screen.findByTestId(`os-worktree-submenu-${project.id}`);
    // Archive alone: Remove project stays off the hover menu.
    expect(within(submenu).queryByRole("menuitem", { name: "Remove project" })).toBeNull();
    await user.click(within(submenu).getByRole("menuitem", { name: "Archive project" }));
    expect(onOverlayOpenChange).toHaveBeenCalledWith("workspace-menu", false);
    setOverlay(null);

    const dialog = await screen.findByRole("dialog", { name: `Archive ${project.name}?` });
    await user.click(within(dialog).getByRole("button", { name: "Archive project" }));
    expect(await within(dialog).findByText("session: Fix the build")).toBeVisible();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.querySelector('[data-slot="os-menubar-workspace"]')).toHaveFocus();
  });

  it("Should close the confirm once the archive succeeds and drop the project row", async () => {
    const user = userEvent.setup();
    const project = { ...primaryWorkspaceFixture, archived_at: null };
    vi.mocked(globalThis.fetch).mockImplementation(input => {
      const url = input instanceof Request ? input.url : String(input);
      if (url.includes(`/api/workspaces/${project.id}/archive`)) {
        return Promise.resolve(
          jsonResponse({ workspace: { ...project, archived_at: "2026-10-30T19:00:00Z" } })
        );
      }
      return Promise.resolve(jsonResponse({ error: "not found", code: "not_found" }, 404));
    });
    const { setOverlay, setWorkspaces } = renderMenubar({
      workspaces: [project],
      activeOverlay: "workspace-menu",
      worktreesByWorkspace: { [project.id]: worktreeListingFixture },
    });

    act(() => screen.getByTestId(`os-workspace-option-${project.id}`).focus());
    await user.keyboard("{ArrowRight}");
    const submenu = await screen.findByTestId(`os-worktree-submenu-${project.id}`);
    await user.click(within(submenu).getByRole("menuitem", { name: "Archive project" }));
    setOverlay(null);
    const dialog = await screen.findByRole("dialog", { name: `Archive ${project.name}?` });
    await user.click(within(dialog).getByRole("button", { name: "Archive project" }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(document.querySelector('[data-slot="os-menubar-workspace"]')).toHaveFocus();
    // The catalog refetch drops the archived project from the reopened top-bar list.
    setWorkspaces([]);
    await screen.findByTestId("os-workspace-menu");
    expect(screen.queryByTestId(`os-workspace-option-${project.id}`)).toBeNull();
  });
});

describe("DesktopMenubar scope control", () => {
  it("Should aria-disable the scope control while scope resolution is pending [RA0289]", () => {
    renderMenubar({ scopePending: true });

    expect(screen.getByTestId("os-global-scope-toggle")).toHaveAttribute("aria-disabled", "true");
  });

  it("Should leave the globe unlocked with zero projects so turning it off lands on User", async () => {
    const user = userEvent.setup();
    const onToggleAllProjects = vi.fn();
    renderMenubar({ breadthMode: "all", onToggleAllProjects });

    const globe = screen.getByTestId("os-global-scope-toggle");
    expect(globe).not.toHaveAttribute("aria-disabled");
    expect(globe).toHaveAttribute("aria-pressed", "true");
    expect(globe).toHaveAccessibleName("All projects");
    await user.click(globe);
    expect(onToggleAllProjects).toHaveBeenCalledOnce();
  });
});

describe("DesktopMenubar command palette control", () => {
  it("Should expose the live palette chord as aria-keyshortcuts", () => {
    const { container } = renderMenubar({
      commands: [
        resolvedPaletteCommand({
          id: "palette.open",
          title: "Command palette",
          bindings: ["meta+KeyK"],
          chords: ["⌘K"],
        }),
      ],
    });

    expect(container.querySelector('[data-slot="os-menubar-command"]')).toHaveAttribute(
      "aria-keyshortcuts",
      "Meta+K"
    );
  });

  it("Should omit aria-keyshortcuts until the palette keymap is known", () => {
    const { container } = renderMenubar();

    expect(container.querySelector('[data-slot="os-menubar-command"]')).not.toHaveAttribute(
      "aria-keyshortcuts"
    );
  });
});

describe("DesktopMenubar tray", () => {
  it("Should toggle the desktops overview from the All desktops control", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    const view = renderMenubar({ onOverlayOpenChange });

    const allDesktops = screen.getByRole("button", { name: "All desktops" });
    expect(allDesktops).toHaveAttribute("aria-expanded", "false");
    await user.click(allDesktops);
    expect(onOverlayOpenChange).toHaveBeenLastCalledWith("desktops", true);

    view.unmount();
    renderMenubar({ activeOverlay: "desktops", onOverlayOpenChange });
    const expanded = screen.getByRole("button", { name: "All desktops" });
    expect(expanded).toHaveAttribute("aria-expanded", "true");
    await user.click(expanded);
    expect(onOverlayOpenChange).toHaveBeenLastCalledWith("desktops", false);
  });

  it("Should leave Settings out of the tray and every tray control outside the menubars", () => {
    renderMenubar();

    expect(screen.queryByRole("button", { name: "Settings" })).not.toBeInTheDocument();
    for (const name of ["All desktops", "Attention", "Command palette"]) {
      expect(screen.getByRole("button", { name }).closest('[role="menubar"]')).toBeNull();
    }
  });
});

describe("DesktopMenubar provider usage control", () => {
  function trayOrder(container: HTMLElement): string[] {
    return Array.from(
      container.querySelectorAll('[data-slot="os-menubar-tray"] [data-slot^="os-menubar-"]')
    ).map(element => element.getAttribute("data-slot") ?? "");
  }

  // The control sits right before the bell; the pager and All desktops lead the tray. DOM order is
  // the visual and tab order.
  it("Should sit right before the bell while the profile has targets", () => {
    const { container } = renderMenubar({ usage: codexWeekly });

    expect(
      screen.getByRole("button", { name: /^Codex · Weekly 72% used · resets in/ })
    ).toHaveAttribute("aria-haspopup", "dialog");
    // The menubar hands the control its icon-button footprint: same as All desktops.
    const footprint = screen.getByRole("button", { name: "All desktops" }).classList;
    const usageButton = screen.getByRole("button", { name: /^Codex · Weekly/ });
    expect([...footprint].filter(token => !usageButton.classList.contains(token))).toEqual([]);
    expect(trayOrder(container)).toEqual([
      "os-menubar-desktops",
      "os-menubar-usage",
      "os-menubar-bell",
      "os-menubar-command",
    ]);
  });

  function holdUsageRead(): () => ((response: Response) => void) | undefined {
    let answer: ((response: Response) => void) | undefined;
    vi.mocked(globalThis.fetch).mockImplementation(
      input =>
        new Promise<Response>(resolve => {
          const url = input instanceof Request ? input.url : String(input);
          if (url.includes("/api/provider-usage") && !url.includes("/stream")) answer = resolve;
          else resolve(jsonResponse({ error: "not found", code: "not_found" }, 404));
        })
    );
    return () => answer;
  }

  // Invariant: the slot before the bell is held while the first read is pending, so a target
  // answer lands in place and the controls already on screen do not move.
  it("Should hold the usage slot while the first read is pending", async () => {
    const answer = holdUsageRead();
    const { container } = renderMenubar();

    await waitFor(() => expect(answer()).toBeTypeOf("function"));
    const pending = screen.getByRole("button", { name: "Provider usage" });
    expect(pending).toHaveAttribute("data-loading", "true");
    expect(trayOrder(container)).toEqual([
      "os-menubar-desktops",
      "os-menubar-usage",
      "os-menubar-bell",
      "os-menubar-command",
    ]);

    answer()?.(jsonResponse(codexWeekly));
    const settled = await screen.findByRole("button", { name: /^Codex · Weekly/ });
    expect(settled).not.toHaveAttribute("data-loading");
    expect(trayOrder(container)).toEqual([
      "os-menubar-desktops",
      "os-menubar-usage",
      "os-menubar-bell",
      "os-menubar-command",
    ]);
  });

  it("Should release the held slot when the first read answers without a target", async () => {
    const answer = holdUsageRead();
    const { container } = renderMenubar();

    await waitFor(() => expect(answer()).toBeTypeOf("function"));
    expect(container.querySelector('[data-slot="os-menubar-usage"]')).not.toBeNull();

    answer()?.(jsonResponse(usageEnvelope([], { enabled: false }, NOW)));
    await waitFor(() =>
      expect(container.querySelector('[data-slot="os-menubar-usage"]')).toBeNull()
    );
    expect(trayOrder(container)).toEqual([
      "os-menubar-desktops",
      "os-menubar-bell",
      "os-menubar-command",
    ]);
  });

  it.each([
    ["the feature is off", usageEnvelope([], { enabled: false }, NOW)],
    ["the profile has no target", usageEnvelope([], {}, NOW)],
  ])("Should leave the control out when %s (UT-052)", (_case, usage) => {
    const { container } = renderMenubar({ usage });

    expect(container.querySelector('[data-slot="os-menubar-usage"]')).toBeNull();
    expect(trayOrder(container)).toEqual([
      "os-menubar-desktops",
      "os-menubar-bell",
      "os-menubar-command",
    ]);
  });

  it("Should show the glyph without a meter when no window is known (UT-051)", () => {
    const { container } = renderMenubar({
      usage: usageEnvelope([statusTargets(NOW).unsupported], {}, NOW),
    });

    const control = screen.getByRole("button", { name: "Provider usage" });
    expect(control.closest('[data-slot="os-menubar-usage"]')).not.toBeNull();
    expect(within(control).queryByRole("progressbar")).toBeNull();
    expect(container.querySelector('[data-slot="os-menubar-usage"] [role="menubar"]')).toBeNull();
  });

  it("Should ask for a background refresh of rows older than 60 s on open (UT-054)", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    const fresh = usageTarget({
      provider_id: "claude",
      provider_name: "Claude",
      measured_at: new Date(NOW - 30 * SECOND).toISOString(),
    });
    const old = { ...codexOkTarget(NOW), measured_at: new Date(NOW - 90 * SECOND).toISOString() };
    renderMenubar({
      onOverlayOpenChange,
      usage: usageEnvelope([fresh, old], { tray: usageTray({}, NOW) }, NOW),
    });

    await user.click(screen.getByRole("button", { name: /^Codex · Weekly/ }));

    expect(onOverlayOpenChange).toHaveBeenLastCalledWith("usage", true);
    await waitFor(() =>
      expect(usageRequests().filter(request => request.method === "POST")).toHaveLength(1)
    );
    const refresh = usageRequests().find(request => request.method === "POST");
    expect(await refresh?.clone().json()).toEqual({
      profile: "default",
      intent: "background",
      provider_ids: ["codex"],
      max_age_seconds: 60,
    });
  });

  it("Should open the Usage app from the popover footer and close the popover (UT-064)", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    renderMenubar({ activeOverlay: "usage", onOverlayOpenChange, usage: codexWeekly });

    const popover = await screen.findByTestId("os-usage-popover");
    await user.click(within(popover).getByRole("button", { name: "Open Usage" }));

    expect(onOverlayOpenChange).toHaveBeenCalledWith("usage", false);
    expect(shell.coordinator.userOpen).toHaveBeenCalledWith({ app: "usage" });
  });

  it("Should navigate to the Usage route while window commands are unavailable (UT-064)", async () => {
    const user = userEvent.setup();
    shell.hydration = "loading";
    renderMenubar({ activeOverlay: "usage", usage: codexWeekly });

    const popover = await screen.findByTestId("os-usage-popover");
    await user.click(within(popover).getByRole("button", { name: "Open Usage" }));

    expect(shell.coordinator.userOpen).not.toHaveBeenCalled();
    expect(shell.coordinator.userNavigate).toHaveBeenCalledWith({ pathname: "/usage", search: {} });
  });

  it("Should open Settings at Provider usage from the footer and from a configure row", async () => {
    const user = userEvent.setup();
    const onOverlayOpenChange = vi.fn();
    renderMenubar({
      activeOverlay: "usage",
      onOverlayOpenChange,
      usage: usageEnvelope([codexOkTarget(NOW), statusTargets(NOW).noPlan], {}, NOW),
    });
    const settings = {
      app: "settings",
      route: { pathname: "/settings/provider-usage", search: {} },
    };

    const popover = await screen.findByTestId("os-usage-popover");
    await user.click(within(popover).getByRole("button", { name: "Settings" }));
    expect(shell.coordinator.userOpen).toHaveBeenLastCalledWith(settings);
    expect(onOverlayOpenChange).toHaveBeenLastCalledWith("usage", false);

    shell.coordinator.userOpen.mockClear();
    onOverlayOpenChange.mockClear();
    await user.click(
      within(popover).getByRole("button", { name: "1 provider without usage data" })
    );
    await user.click(within(popover).getByRole("button", { name: "Open settings" }));
    expect(shell.coordinator.userOpen).toHaveBeenLastCalledWith(settings);
    expect(onOverlayOpenChange).toHaveBeenLastCalledWith("usage", false);
    expect(usageRequests().filter(request => request.method === "POST")).toHaveLength(0);
  });

  it.each([
    [true, 1],
    [false, 0],
  ])(
    "Should offer Allow only when the daemon supports Keychain consent (%s)",
    async (keychain_supported, allowCount) => {
      const user = userEvent.setup();
      renderMenubar({
        activeOverlay: "usage",
        usage: usageEnvelope(
          [codexOkTarget(NOW), statusTargets(NOW).needsConsent],
          { keychain_supported },
          NOW
        ),
      });

      const popover = await screen.findByTestId("os-usage-popover");
      await user.click(within(popover).getByRole("button", { name: "Claude, Needs access" }));
      expect(within(popover).getByText("Needs Keychain access")).toBeInTheDocument();
      expect(within(popover).queryAllByRole("button", { name: "Allow" })).toHaveLength(allowCount);
    }
  );

  it.each([false, true])("Should report consent save failures once (%s)", async failed => {
    const notify = vi.spyOn(toast, "error").mockReturnValue("consent-error");
    const user = userEvent.setup();
    const consent = usageEnvelope(
      [codexOkTarget(NOW), statusTargets(NOW).needsConsent],
      { profile_id: "work", keychain_supported: true },
      NOW
    );
    vi.mocked(globalThis.fetch).mockImplementation(input => {
      const request = input as Request;
      const { pathname } = new URL(request.url);
      if (pathname === "/api/profiles/selection") {
        return Promise.resolve(jsonResponse([{ scope: "global", profile: "work" }]));
      }
      if (pathname === "/api/settings/provider-usage") {
        if (failed && request.method === "PATCH") {
          return Promise.resolve(
            jsonResponse({ error: "Consent save refused", code: "validation_error" }, 400)
          );
        }
        return Promise.resolve(
          jsonResponse(
            request.method === "PATCH"
              ? {
                  section: "provider-usage",
                  scope: "user",
                  restart_required: false,
                  warnings: [],
                }
              : {
                  section: "provider-usage",
                  scope: "user",
                  available_scopes: ["user"],
                  keychain_supported: true,
                  config: { enabled: true, refresh_interval: "5m0s", providers: {} },
                }
          )
        );
      }
      if (pathname.startsWith("/api/provider-usage")) return Promise.resolve(jsonResponse(consent));
      return Promise.resolve(jsonResponse({ error: "not found", code: "not_found" }, 404));
    });
    const queryClient = new QueryClient({
      defaultOptions: { queries: { enabled: false, retry: false } },
    });
    queryClient.setQueryData(["provider-usage", "envelope", "work"], consent);
    render(
      <QueryClientProvider client={queryClient}>
        <UIProvider reducedMotion="never" skipAnimations>
          <CmdPaletteRegistryProvider registry={paletteRegistryFixture([])}>
            <DesktopMenubar
              workspaces={[]}
              activeWorkspace={undefined}
              breadthMode="focused"
              focus={{ kind: "user" }}
              onSelectWorkspace={vi.fn()}
              onAddWorkspace={vi.fn()}
              onRunCommand={vi.fn()}
              activeOverlay="usage"
              onOverlayOpenChange={vi.fn()}
              attention={ATTENTION}
              update={{ state: "hidden" }}
            />
          </CmdPaletteRegistryProvider>
        </UIProvider>
      </QueryClientProvider>
    );

    const popover = await screen.findByTestId("os-usage-popover");
    await user.click(await within(popover).findByRole("button", { name: "Claude, Needs access" }));
    await user.click(await within(popover).findByRole("button", { name: "Allow" }));

    const requests = () =>
      vi
        .mocked(globalThis.fetch)
        .mock.calls.map(([input]) => input as Request)
        .filter(request => request.method !== "GET");
    await waitFor(() => expect(requests()).toHaveLength(1));
    const [save] = requests();
    expect(save?.method).toBe("PATCH");
    expect(new URL(save?.url ?? "").searchParams.get("collection_profile")).toBe("work");
    expect(usageRequests().filter(request => request.method === "POST")).toHaveLength(0);
    if (failed) {
      await waitFor(() =>
        expect(notify).toHaveBeenCalledWith("Couldn't allow Keychain access: Consent save refused")
      );
      expect(notify).toHaveBeenCalledTimes(1);
    } else {
      expect(notify).not.toHaveBeenCalled();
    }
  });
});
