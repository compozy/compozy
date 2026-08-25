import { describe, expect, it } from "vitest";

import { OS_APPS, matchSessionInstance, resolveAppForPath } from "../app-registry";
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

  it("Should own the desktop root exactly (no prefix bleed)", () => {
    expect(resolveAppForPath("/")?.app.id).toBe("dashboard");
    expect(resolveAppForPath("/tasks")?.app.id).toBe("tasks");
  });

  it("Should keep the Agents app owning its Activity and call locations", () => {
    // Both are new locations of the existing app, not a new app: the `/agents`
    // prefix already owns them, and nothing new is registered.
    expect(resolveAppForPath("/agents/activity")?.app.id).toBe("agents");
    expect(resolveAppForPath("/agents/calls/call_01JBD8G2K7Q9")?.app.id).toBe("agents");
    // A call is not a session, so it must not be claimed as a session instance.
    expect(matchSessionInstance("/agents/calls/call_01JBD8G2K7Q9")).toBeNull();
    expect(resolveAppForPath("/agents/activity")?.instanceKey).toBeNull();
  });

  it("Should light the Agents dock tile from the delegation badge", () => {
    // The union widened rather than a second tile appearing.
    expect(OS_APPS.agents.badge).toBe("calls");
  });
});
