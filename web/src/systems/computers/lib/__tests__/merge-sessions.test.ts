// Suite: Multi-computer session merge and offline retention (UT-085, UT-086)
// Invariant: Multi-computer lists merge by updated_at desc and keep offline rows marked offline.
// Boundary IN: Pure merge function in web/src/systems/computers/lib/merge-sessions.ts.
// Boundary OUT: TanStack Query fan-out and HTTP transport.

import { describe, expect, it } from "vitest";

import { COMPUTER_SESSION_PAGE_LIMIT, mergeSessionPages } from "../merge-sessions";

describe("mergeSessionPages", () => {
  it("UT-085: Should merge sessions from multiple computers ordered by updated_at desc with computer provenance", () => {
    expect(COMPUTER_SESSION_PAGE_LIMIT).toBe(50);

    const merged = mergeSessionPages([
      {
        computer: { id: "cmp_a", name: "devbox", status: "connected" },
        sessions: [{ id: "a1", updated_at: "2026-04-10T00:00:03.000Z" }],
      },
      {
        computer: { id: "cmp_b", name: "studio", status: "connected" },
        sessions: [{ id: "b1", updated_at: "2026-04-10T00:00:05.000Z" }],
      },
    ]);

    expect(merged.map(row => row.session.id)).toEqual(["b1", "a1"]);
    expect(merged[0]).toMatchObject({
      computerId: "cmp_b",
      computerName: "studio",
      offline: false,
    });
    expect(merged[1]).toMatchObject({
      computerId: "cmp_a",
      computerName: "devbox",
      offline: false,
    });
  });

  it("UT-086: Should retain cached rows with offline: true when a computer goes offline", () => {
    const merged = mergeSessionPages([
      {
        computer: { id: "local", name: "This computer", status: "connected" },
        sessions: [{ id: "local-1", updated_at: "2026-04-10T00:00:05.000Z" }],
      },
      {
        computer: { id: "cmp_devbox", name: "devbox", status: "offline" },
        sessions: [{ id: "remote-1", updated_at: "2026-04-10T00:00:04.000Z" }],
      },
    ]);

    expect(merged.map(row => row.session.id)).toEqual(["local-1", "remote-1"]);
    expect(merged[0]?.offline).toBe(false);
    expect(merged[1]).toMatchObject({
      computerId: "cmp_devbox",
      computerName: "devbox",
      offline: true,
    });
  });
});
