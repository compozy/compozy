// Suite: Multi-computer session merge, offline retention, paging, and grouping (UT-085, UT-086, UT-087, UT-088)
// Invariant: Multi-computer lists merge by updated_at desc, keep offline rows marked offline,
//            page 50 per computer advancing only the oldest cursor, and separate same-repo groups by computer.
// Boundary IN: Pure merge/paging/grouping functions in web/src/systems/computers/lib/merge-sessions.ts.
// Boundary OUT: TanStack Query fan-out and HTTP transport.

import { describe, expect, it } from "vitest";

import {
  COMPUTER_SESSION_PAGE_LIMIT,
  groupSessionsByProjectAndComputer,
  mergeSessionPages,
  selectNextComputerToLoadMore,
} from "../merge-sessions";

describe("mergeSessionPages and multi-computer helpers", () => {
  it("UT-085: Should merge sessions from multiple computers ordered by updated_at desc with computer provenance", () => {
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

  it("UT-087: Should use limit 50 per computer and advance only the computer whose cursor is oldest", () => {
    expect(COMPUTER_SESSION_PAGE_LIMIT).toBe(50);

    const pages = Array.from({ length: 10 }, (_, index) => ({
      computer: {
        id: `cmp_${index + 1}`,
        name: `host-${index + 1}`,
        status: "connected",
      },
      sessions: [
        {
          id: `sess_${index + 1}_last`,
          // Host 7 has the oldest boundary timestamp; load-more must advance only host 7.
          updated_at:
            index === 6
              ? "2026-04-01T00:00:00.000Z"
              : `2026-04-${String(10 + index).padStart(2, "0")}T00:00:00.000Z`,
        },
      ],
      hasMore: true,
      nextCursor: `cur_${index + 1}`,
    }));

    const next = selectNextComputerToLoadMore(pages);
    expect(next).toEqual({
      computerId: "cmp_7",
      nextCursor: "cur_7",
      oldestTimestamp: Date.parse("2026-04-01T00:00:00.000Z"),
    });
  });

  it("UT-088: Should keep same-repository projects on two computers as separate labeled groups", () => {
    const merged = mergeSessionPages([
      {
        computer: { id: "cmp_devbox", name: "devbox", status: "connected" },
        sessions: [
          {
            id: "s_devbox",
            updated_at: "2026-04-10T00:00:05.000Z",
            workspace_id: "ws_api_a",
            workspace_name: "api",
          },
        ],
      },
      {
        computer: { id: "cmp_studio", name: "studio", status: "connected" },
        sessions: [
          {
            id: "s_studio",
            updated_at: "2026-04-10T00:00:04.000Z",
            workspace_id: "ws_api_b",
            workspace_name: "api",
          },
        ],
      },
    ]);

    const groups = groupSessionsByProjectAndComputer(merged);
    expect(groups).toHaveLength(2);
    expect(groups.map(group => group.label)).toEqual(["api · devbox", "api · studio"]);
    expect(groups[0]?.key).not.toBe(groups[1]?.key);
  });
});
