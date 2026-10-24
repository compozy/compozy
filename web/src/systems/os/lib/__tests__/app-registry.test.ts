import { render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import { OS_APPS, matchSessionInstance, resolveAppForPath } from "../app-registry";
import type { OsAppId } from "../os-types";
import { defaultOsWindowRoute } from "../window-manager-view";
import { pickLastCreatedSession } from "../last-created-session";
import type { SessionPayload } from "@/systems/session";

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
// a single loading region, and leaves nothing in it that can take focus or be activated.
const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled]), [contenteditable="true"]';

interface FallbackCase {
  app: OsAppId;
  label: string;
  pathname: string;
  search?: Record<string, unknown>;
}

const FALLBACK_CASES: readonly FallbackCase[] = [
  { app: "session", label: "a session", pathname: "/agents/webgen/sessions/s1" },
  { app: "session", label: "no session selected", pathname: "/sessions" },
  { app: "settings", label: "General", pathname: "/settings/general" },
  { app: "settings", label: "a lazy section", pathname: "/settings/providers" },
  { app: "settings", label: "a retired section", pathname: "/settings/defaults" },
  { app: "tasks", label: "the list", pathname: "/tasks" },
  { app: "tasks", label: "the kanban board", pathname: "/tasks", search: { mode: "kanban" } },
  { app: "tasks", label: "the inbox", pathname: "/tasks", search: { mode: "inbox" } },
  { app: "tasks", label: "the dashboard", pathname: "/tasks", search: { mode: "dashboard" } },
  { app: "tasks", label: "the new-task dialog over the list", pathname: "/tasks/new" },
  { app: "tasks", label: "a task", pathname: "/tasks/t1" },
  { app: "tasks", label: "a task's runs tab", pathname: "/tasks/t1", search: { tab: "runs" } },
  { app: "tasks", label: "a task being edited", pathname: "/tasks/t1/edit" },
  { app: "tasks", label: "a run", pathname: "/tasks/t1/runs/r1" },
  { app: "loops", label: "the catalog", pathname: "/loops" },
  { app: "loops", label: "the catalog as cards", pathname: "/loops", search: { view: "cards" } },
  { app: "loops", label: "a Loop", pathname: "/loops/ship-it" },
  { app: "loops", label: "a Loop's run form", pathname: "/loops/ship-it/run" },
  { app: "loops", label: "a Loop's editor", pathname: "/loops/ship-it/editor" },
  { app: "loops", label: "a Loop's configure dialog", pathname: "/loops/ship-it/configure" },
  { app: "loops", label: "the runs roster", pathname: "/loop-runs" },
  {
    app: "loops",
    label: "the step inventory",
    pathname: "/loop-runs",
    search: { nodes: "waiting" },
  },
  { app: "loops", label: "a run", pathname: "/loop-runs/r1" },
  { app: "loops", label: "a run's comparison", pathname: "/loop-runs/r1/diff" },
  {
    app: "loops",
    label: "a run's comparison against another run",
    pathname: "/loop-runs/r1/diff",
    search: { against_run: "r0" },
  },
  { app: "agents", label: "the fleet", pathname: "/agents" },
  { app: "agents", label: "the fleet as cards", pathname: "/agents", search: { view: "cards" } },
  { app: "agents", label: "an agent", pathname: "/agents/webgen" },
  {
    app: "agents",
    label: "an agent's instructions",
    pathname: "/agents/webgen",
    search: { tab: "instructions", file: "soul" },
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
  { app: "automations", label: "the listing", pathname: "/automations" },
  {
    app: "automations",
    label: "the listing as cards",
    pathname: "/automations",
    search: { view: "cards" },
  },
  { app: "automations", label: "a job", pathname: "/automations/jobs/morning-digest" },
  { app: "automations", label: "a trigger", pathname: "/automations/triggers/rerun-delivery" },
  { app: "marketplace", label: "Browse", pathname: "/marketplace" },
  { app: "marketplace", label: "Installed", pathname: "/marketplace/installed" },
  { app: "marketplace", label: "an entry", pathname: "/marketplace/compozy-catalog%2Fskills" },
  { app: "marketplace", label: "a path it does not own", pathname: "/marketplace/a/b" },
  { app: "vault", label: "the secrets as rows", pathname: "/vault" },
  { app: "vault", label: "the secrets as cards", pathname: "/vault", search: { view: "cards" } },
];

describe("app loading fallbacks", () => {
  it("Should give every app in the content-shaped rollout a fallback", () => {
    for (const app of [
      "session",
      "settings",
      "tasks",
      "loops",
      "agents",
      "automations",
      "marketplace",
      "vault",
    ] as const) {
      expect(OS_APPS[app].LoadingFallback, app).toBeTypeOf("function");
    }
  });

  it.each(FALLBACK_CASES)(
    "Should render $app on $label ($pathname) as one loading region with nothing focusable",
    ({ app, pathname, search = {} }) => {
      const Fallback = OS_APPS[app].LoadingFallback;
      if (!Fallback) throw new Error(`${app} has no loading fallback`);

      const { container } = render(createElement(Fallback, { route: { pathname, search } }));

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
});
