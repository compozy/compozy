import { describe, expect, it } from "vitest";

import { OS_APPS, matchSessionInstance, resolveAppForPath } from "../app-registry";

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

  it("Should own the desktop root exactly (no prefix bleed)", () => {
    expect(resolveAppForPath("/")?.app.id).toBe("dashboard");
    expect(resolveAppForPath("/tasks")?.app.id).toBe("tasks");
  });
});
