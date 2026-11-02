import { act, render, screen } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it } from "vitest";

import { WindowScopeContext } from "@/hooks/use-window-scope";

import { useTopbarSlot } from "@compozy/ui";

import { OsWindowSurface } from "../../components/os-window-frame";
import { OS_APPS, matchSessionInstance, resolveAppForPath } from "../app-registry";
import type { OsAppId } from "../os-types";
import { defaultOsWindowRoute } from "../window-manager-view";
import { pickLastCreatedSession } from "../last-created-session";
import type { SessionPayload } from "@/systems/session";
import { sessionSidebarStore } from "@/systems/session/hooks/use-session-sidebar-state";
import { SHELL_WORKTREE_SCOPE } from "@/systems/workspace";

function catalogSession(id: string, createdAt: string, archivedAt: string | null = null) {
  return { id, created_at: createdAt, archived_at: archivedAt } as SessionPayload;
}

describe("app registry", () => {
  it("Should extract the session instance key and reject non-session agent paths (UT-050)", () => {
    expect(matchSessionInstance("/agents/webgen/sessions/s1")).toBe("s1");
    expect(matchSessionInstance("/agents/webgen/settings")).toBeNull();
    expect(OS_APPS.session.matchInstance?.("/agents/webgen/sessions/s1")).toBe("s1");
  });

  it("Should resolve pathname ownership per app prefix (UT-051)", () => {
    expect(resolveAppForPath("/loop-runs/r1")?.app.id).toBe("loops");
    expect(resolveAppForPath("/marketplace/skills")?.app.id).toBe("marketplace");
    expect(resolveAppForPath("/does-not-exist")).toBeNull();
  });

  it("Should route session paths to the session app ahead of the agents prefix", () => {
    const resolved = resolveAppForPath("/agents/webgen/sessions/s1");
    expect(resolved?.app.id).toBe("session");
    expect(resolved?.instanceKey).toBe("s1");
    expect(resolveAppForPath("/agents/webgen")?.app.id).toBe("agents");
  });

  it("Should resolve the session empty route without an instance key", () => {
    const resolved = resolveAppForPath("/sessions");
    expect(resolved?.app.id).toBe("session");
    expect(resolved?.instanceKey).toBeNull();
    expect(matchSessionInstance("/sessions")).toBeNull();
    expect(defaultOsWindowRoute("session")).toEqual({ pathname: "/sessions", search: {} });
  });

  it("Should pick the latest created live session and ignore archived rows", () => {
    const older = catalogSession("sess-a", "2026-08-01T00:00:00Z");
    const newer = catalogSession("sess-b", "2026-08-02T00:00:00Z");
    const archivedNewer = catalogSession("sess-c", "2026-08-03T00:00:00Z", "2026-08-04T00:00:00Z");
    const tiedHigherId = catalogSession("sess-z", "2026-08-02T00:00:00Z");

    expect(pickLastCreatedSession([older, archivedNewer])).toEqual(older);
    expect(pickLastCreatedSession([older, newer, tiedHigherId])).toEqual(tiedHigherId);
    expect(pickLastCreatedSession([archivedNewer])).toBeNull();
  });

  it("Should resolve each terminal path as its own lazy app instance", () => {
    const resolved = resolveAppForPath("/terminal/term-4aa01f22e6c3");
    expect(resolved?.app.id).toBe("terminal");
    expect(resolved?.instanceKey).toBe("term-4aa01f22e6c3");
    expect(OS_APPS.terminal.matchInstance?.("/terminal/term-other")).toBe("term-other");
    expect(resolveAppForPath("/terminal")?.app.id).toBe("terminal");
  });

  it("Should register one Automations app after Loops and no Jobs or Triggers apps", () => {
    const ids = Object.keys(OS_APPS);
    expect(ids).not.toContain("jobs");
    expect(ids).not.toContain("triggers");
    expect(ids.indexOf("automations")).toBe(ids.indexOf("loops") + 1);
    expect(OS_APPS.automations).toMatchObject({
      title: "Automations",
      paths: ["/automations"],
      dock: { group: 2 },
    });
    expect(resolveAppForPath("/automations/jobs/morning-digest")?.app.id).toBe("automations");
    expect(resolveAppForPath("/automations/triggers/rerun-delivery")?.app.id).toBe("automations");
  });

  it("Should own the desktop root exactly (no prefix bleed)", () => {
    expect(resolveAppForPath("/")?.app.id).toBe("dashboard");
    expect(resolveAppForPath("/tasks")?.app.id).toBe("tasks");
  });
});

// Invariant: an app's loading fallback is a static frame for the route it is about to render. It
// mounts with no providers (no query, router, store or window context), announces itself once as
// a single loading region, and leaves nothing in it that can take focus or be activated. A listing
// app's strip goes where the loaded app's will, in the frame's strip row above the body, so the
// body never changes place when the app arrives, and the app's own strip takes the row over.
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled]), [contenteditable="true"]';

interface FallbackCase {
  app: OsAppId;
  label: string;
  pathname: string;
  search?: Record<string, unknown>;
  /** An unbound window on the All projects desktop; the rest read one project. */
  aggregate?: boolean;
  /** The listing's strip, which the loaded app publishes above the body. */
  strip?: true;
}

const FALLBACK_CASES: readonly FallbackCase[] = [
  { app: "session", label: "a session", pathname: "/agents/webgen/sessions/s1" },
  { app: "session", label: "no session selected", pathname: "/sessions" },
  { app: "settings", label: "General", pathname: "/settings/general" },
  { app: "settings", label: "a lazy section", pathname: "/settings/providers" },
  { app: "settings", label: "a retired section", pathname: "/settings/defaults" },
  { app: "tasks", label: "the list", pathname: "/tasks", strip: true },
  {
    app: "tasks",
    label: "the kanban board",
    pathname: "/tasks",
    search: { mode: "kanban" },
    strip: true,
  },
  { app: "tasks", label: "the inbox", pathname: "/tasks", search: { mode: "inbox" }, strip: true },
  {
    app: "tasks",
    label: "the dashboard",
    pathname: "/tasks",
    search: { mode: "dashboard" },
    strip: true,
  },
  { app: "tasks", label: "the new-task dialog over the list", pathname: "/tasks/new", strip: true },
  { app: "tasks", label: "a task", pathname: "/tasks/t1" },
  { app: "tasks", label: "a task's runs tab", pathname: "/tasks/t1", search: { tab: "runs" } },
  { app: "tasks", label: "a task being edited", pathname: "/tasks/t1/edit" },
  { app: "tasks", label: "a run", pathname: "/tasks/t1/runs/r1" },
  { app: "loops", label: "the catalog", pathname: "/loops", strip: true },
  {
    app: "loops",
    label: "the catalog as cards",
    pathname: "/loops",
    search: { view: "cards" },
    strip: true,
  },
  { app: "loops", label: "a Loop", pathname: "/loops/ship-it" },
  { app: "loops", label: "a Loop's run form", pathname: "/loops/ship-it/run" },
  { app: "loops", label: "a Loop's editor", pathname: "/loops/ship-it/editor" },
  { app: "loops", label: "a Loop's configure dialog", pathname: "/loops/ship-it/configure" },
  { app: "loops", label: "the runs roster", pathname: "/loop-runs", strip: true },
  {
    app: "loops",
    label: "the step inventory",
    pathname: "/loop-runs",
    search: { nodes: "waiting" },
    strip: true,
  },
  { app: "loops", label: "a run", pathname: "/loop-runs/r1" },
  { app: "loops", label: "a run's comparison", pathname: "/loop-runs/r1/diff" },
  {
    app: "loops",
    label: "a run's comparison against another run",
    pathname: "/loop-runs/r1/diff",
    search: { against_run: "r0" },
  },
  { app: "agents", label: "the fleet", pathname: "/agents", strip: true },
  {
    app: "agents",
    label: "the fleet as cards",
    pathname: "/agents",
    search: { view: "cards" },
    strip: true,
  },
  { app: "agents", label: "an agent", pathname: "/agents/webgen" },
  {
    app: "agents",
    label: "an agent's instructions",
    pathname: "/agents/webgen",
    search: { tab: "instructions", file: "soul" },
  },
  {
    app: "agents",
    label: "an agent's heartbeat file (status card over the editor)",
    pathname: "/agents/webgen",
    search: { tab: "instructions", file: "heartbeat" },
  },
  {
    app: "agents",
    label: "an agent's sessions",
    pathname: "/agents/webgen",
    search: { tab: "sessions", filter: "failed" },
  },
  {
    app: "agents",
    label: "an agent's configuration",
    pathname: "/agents/webgen",
    search: { tab: "configuration" },
  },
  { app: "agents", label: "an agent's settings dialog", pathname: "/agents/webgen/settings" },
  { app: "automations", label: "the listing", pathname: "/automations", strip: true },
  {
    app: "automations",
    label: "the listing as cards",
    pathname: "/automations",
    search: { view: "cards" },
    strip: true,
  },
  { app: "automations", label: "a job", pathname: "/automations/jobs/morning-digest" },
  { app: "automations", label: "a trigger", pathname: "/automations/triggers/rerun-delivery" },
  { app: "marketplace", label: "Browse", pathname: "/marketplace", strip: true },
  { app: "marketplace", label: "Installed", pathname: "/marketplace/installed", strip: true },
  { app: "marketplace", label: "an entry", pathname: "/marketplace/compozy-catalog%2Fskills" },
  {
    app: "marketplace",
    label: "a path it does not own",
    pathname: "/marketplace/a/b",
    strip: true,
  },
  { app: "vault", label: "the secrets as rows", pathname: "/vault", strip: true },
  {
    app: "vault",
    label: "the secrets as cards",
    pathname: "/vault",
    search: { view: "cards" },
    strip: true,
  },
  { app: "dashboard", label: "Dashboard", pathname: "/" },
  { app: "new-tab", label: "an empty tab", pathname: "/new-tab" },
  { app: "terminal", label: "a project's terminal", pathname: "/terminal" },
  { app: "terminal", label: "a project's terminal by id", pathname: "/terminal/term-1" },
  { app: "terminal", label: "every project's terminals", pathname: "/terminal", aggregate: true },
  {
    app: "terminal",
    label: "a terminal named without its project",
    pathname: "/terminal/term-1",
    aggregate: true,
  },
  { app: "usage", label: "Provider usage", pathname: "/usage" },
];

describe("app loading fallbacks", () => {
  it("Should give every app a fallback", () => {
    for (const [app, definition] of Object.entries(OS_APPS)) {
      expect(definition.LoadingFallback, app).toBeTypeOf("function");
    }
  });

  it.each(FALLBACK_CASES)(
    "Should render $app on $label ($pathname) as one loading region with nothing focusable",
    ({ app, pathname, search = {}, aggregate = false }) => {
      const Fallback = OS_APPS[app].LoadingFallback;
      if (!Fallback) throw new Error(`${app} has no loading fallback`);

      const { container } = render(
        createElement(Fallback, { route: { pathname, search }, aggregate })
      );

      const regions = screen.getAllByRole("status");
      expect(regions).toHaveLength(1);
      expect(regions[0]).toHaveAttribute("aria-busy", "true");
      expect(regions[0]).toHaveTextContent(/^Loading/);
      const focusable = [...container.querySelectorAll(FOCUSABLE)].filter(
        element => !element.closest("[inert]")
      );
      expect(focusable).toEqual([]);
    }
  );

  it.each([
    { aggregate: true, pathname: "/terminal", shape: "terminal-aggregate-loading" },
    { aggregate: true, pathname: "/terminal/term-1", shape: "terminal-aggregate-loading" },
    { aggregate: false, pathname: "/terminal", shape: "terminal-window-loading" },
    { aggregate: false, pathname: "/terminal/term-1", shape: "terminal-window-loading" },
  ])(
    "Should load a terminal window at $pathname into the $shape shape its scope picks (aggregate: $aggregate)",
    ({ aggregate, pathname, shape }) => {
      const Fallback = OS_APPS.terminal.LoadingFallback;
      if (!Fallback) throw new Error("terminal has no loading fallback");

      render(createElement(Fallback, { route: { pathname, search: {} }, aggregate }));

      expect(screen.getByTestId(shape)).toBeInTheDocument();
    }
  );

  it.each(FALLBACK_CASES.filter(({ strip }) => strip))(
    "Should publish $app's strip on $label into the frame's strip row above the body and leave the head alone",
    ({ app, pathname, search = {}, aggregate = false }) => {
      const definition = OS_APPS[app];
      const Fallback = definition.LoadingFallback;
      if (!Fallback) throw new Error(`${app} has no loading fallback`);

      const { container } = render(
        createElement(
          OsWindowSurface,
          { title: definition.title },
          createElement(Fallback, { route: { pathname, search }, aggregate })
        )
      );

      const row = container.querySelector('[data-slot="os-window-toolbar"]');
      const body = container.querySelector('[data-slot="os-window-body"]');
      expect(row?.querySelector('[data-slot="toolbar-skeleton"]')).not.toBeNull();
      expect(body?.querySelector('[data-slot="toolbar-skeleton"]')).toBeNull();
      expect(row?.compareDocumentPosition(body as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
      // Only the strip is published: the head still names the window.
      expect(screen.getByTestId("topbar-title-text")).toHaveTextContent(definition.title);
      expect(screen.getAllByRole("status")).toHaveLength(1);
      expect([...(row?.querySelectorAll(FOCUSABLE) ?? [])]).toEqual([]);
    }
  );

  // The loading frame must honor each window's saved presentation before the lazy app loads.
  // Owner: OS app registry; exercise the real window-scoped store, without network or hook mocks.
  it("Should restore a Kanban loading frame without changing another window's List frame", () => {
    const Fallback = OS_APPS.session.LoadingFallback!;
    act(() => {
      sessionSidebarStore.trigger.viewChanged({ scopeId: "fallback-board", view: "kanban" });
      sessionSidebarStore.trigger.viewChanged({ scopeId: "fallback-list", view: "list" });
    });
    const { container, rerender } = render(
      createElement(
        WindowScopeContext,
        { value: "fallback-board" },
        createElement(Fallback, { route: { pathname: "/sessions", search: {} }, aggregate: false })
      )
    );
    expect(screen.getByTestId("sessions-board-skeleton")).toBeInTheDocument();
    expect(container.querySelector('[data-slot="session-sidebar-static-layout"]')).toBeNull();
    expect(screen.getAllByRole("status")).toHaveLength(1);
    rerender(
      createElement(
        WindowScopeContext,
        { value: "fallback-list" },
        createElement(Fallback, { route: { pathname: "/sessions", search: {} }, aggregate: false })
      )
    );
    expect(screen.queryByTestId("sessions-board-skeleton")).not.toBeInTheDocument();
    expect(container.querySelector('[data-slot="session-sidebar-static-layout"]')).not.toBeNull();
  });

  describe("the session rail", () => {
    afterEach(() => {
      act(() => {
        sessionSidebarStore.trigger.widthReset({ scopeId: SHELL_WORKTREE_SCOPE });
        sessionSidebarStore.trigger.sidebarVisibilityChanged({
          scopeId: SHELL_WORKTREE_SCOPE,
          open: true,
        });
      });
    });

    it("Should hold the rail at the width the window saved, in pixels, and collapse it with the rail", () => {
      const Fallback = OS_APPS.session.LoadingFallback;
      if (!Fallback) throw new Error("session has no loading fallback");
      act(() => {
        sessionSidebarStore.trigger.widthChanged({ scopeId: SHELL_WORKTREE_SCOPE, width: 320 });
      });

      const { container } = render(
        createElement(Fallback, { route: { pathname: "/sessions", search: {} }, aggregate: false })
      );
      const rail = container.querySelector('[data-slot="session-sidebar-static-layout"]')
        ?.firstElementChild as HTMLElement;

      expect(rail.style.width).toBe("320px");
      act(() => {
        sessionSidebarStore.trigger.sidebarVisibilityChanged({
          scopeId: SHELL_WORKTREE_SCOPE,
          open: false,
        });
      });
      expect(rail.style.width).toBe("0px");
    });
  });

  it("Should hand the strip row to the app's own strip, and clear it when the app publishes none", () => {
    const Fallback = OS_APPS.vault.LoadingFallback;
    if (!Fallback) throw new Error("vault has no loading fallback");
    function PublishingApp({ strip }: { strip: boolean }) {
      useTopbarSlot({
        crumb: "Vault",
        toolbar: strip ? createElement("span", { "data-testid": "app-strip" }) : undefined,
      });
      return null;
    }
    const frame = (child: React.ReactNode) =>
      createElement(OsWindowSurface, { title: "Vault" }, child);
    const { container, rerender } = render(
      frame(
        createElement(Fallback, { route: { pathname: "/vault", search: {} }, aggregate: false })
      )
    );
    const row = () => container.querySelector('[data-slot="os-window-toolbar"]');

    expect(row()?.querySelector('[data-slot="toolbar-skeleton"]')).not.toBeNull();

    rerender(frame(createElement(PublishingApp, { strip: true })));
    expect(row()).toContainElement(screen.getByTestId("app-strip"));
    expect(row()?.querySelector('[data-slot="toolbar-skeleton"]')).toBeNull();

    rerender(frame(createElement(PublishingApp, { strip: false })));
    expect(row()).toBeNull();

    // An app that never publishes leaves nothing of the fallback's strip behind either.
    rerender(
      frame(
        createElement(Fallback, { route: { pathname: "/vault", search: {} }, aggregate: false })
      )
    );
    expect(row()?.querySelector('[data-slot="toolbar-skeleton"]')).not.toBeNull();
    rerender(frame(createElement("div")));
    expect(row()).toBeNull();
  });
});
