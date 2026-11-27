// Suite: Together layout grouping, sorting, computer provenance and Load more selection (T-F6)
// Invariant: Together + Project builds `project · computer` groups from per-computer facets
//            without merging computers; Together + None honors the window sort (O1, O2) and
//            selects the oldest-cursor connected computer for Load more (S10).
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { LOCAL_COMPUTER_FIXTURE } from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionPayload } from "../../types";
import {
  buildTogetherComputerMap,
  buildTogetherProjectGroups,
  mergeTogetherFlatRows,
  selectNextTogetherComputer,
} from "../session-list-together";

function makeSession(overrides: Partial<SessionPayload> & { id: string }): SessionPayload {
  return {
    name: overrides.id,
    agent_name: "claude",
    state: "idle",
    badge: "idle",
    workspace_id: "ws_1",
    created_at: "2026-04-10T00:00:00.000Z",
    updated_at: "2026-04-10T00:00:00.000Z",
    pending_interactions: [],
    runtime: {},
    ...overrides,
  } as SessionPayload;
}

function makeSource(
  overrides: Partial<ComputerSessionsSource> & { key: string; name: string }
): ComputerSessionsSource {
  const isLocal = overrides.key === LOCAL_COMPUTER_ID;
  return {
    isLocal,
    computer: LOCAL_COMPUTER_FIXTURE,
    read: "ok",
    detail: isLocal ? "This computer" : "via LAN",
    canOperate: true,
    canCreate: true,
    facets: undefined,
    count: 0,
    needsYou: 0,
    working: 0,
    laterCount: 0,
    workspaces: [],
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: null,
    retry: () => undefined,
    ...overrides,
  };
}

describe("Together layout helpers (T-F6)", () => {
  it("Should build project · computer groups from per-computer facets without merging computers", () => {
    const local = makeSource({
      key: LOCAL_COMPUTER_ID,
      name: "MacBook Pro",
      workspaces: [{ id: "ws_local", name: "compozy" } as never],
      facets: {
        facets: { all: 3, needs_you: 1, working: 1, idle: 1 } as never,
        by_workspace: [
          {
            workspace_id: "ws_local",
            facets: { all: 3, needs_you: 1, working: 1, idle: 1 } as never,
          },
        ],
        by_profile: [],
        later: 0,
      },
    });
    const devbox = makeSource({
      key: "cmp_devbox",
      name: "devbox",
      workspaces: [{ id: "ws_remote", name: "compozy" } as never],
      facets: {
        facets: { all: 2, needs_you: 0, working: 1, idle: 1 } as never,
        by_workspace: [
          {
            workspace_id: "ws_remote",
            facets: { all: 2, needs_you: 0, working: 1, idle: 1 } as never,
          },
        ],
        by_profile: [],
        later: 0,
      },
    });

    const groups = buildTogetherProjectGroups([local, devbox], { profile: "default" });
    expect(groups).toHaveLength(2);
    expect(groups[0]).toMatchObject({
      key: "project:local:ws_local",
      computerName: "MacBook Pro",
      badge: { kind: "project", name: "compozy" },
      total: 3,
      needsYou: 1,
    });
    expect(groups[1]).toMatchObject({
      key: "project:cmp_devbox:ws_remote",
      computerId: "cmp_devbox",
      computerName: "devbox",
      badge: { kind: "project", name: "compozy" },
      total: 2,
      working: 1,
    });
  });

  it("Should merge flat rows by window sort (O1, O2) and name the oldest-cursor computer for Load more (S10)", () => {
    const local = makeSource({ key: LOCAL_COMPUTER_ID, name: "MacBook Pro" });
    const devbox = makeSource({ key: "cmp_devbox", name: "devbox" });
    const entries = [
      {
        source: local,
        sessions: [
          makeSession({
            id: "s1",
            badge: "idle",
            created_at: "2026-04-10T10:00:00.000Z",
            updated_at: "2026-04-10T12:00:00.000Z",
          }),
        ],
        hasMore: true,
        nextCursor: "cur_local",
      },
      {
        source: devbox,
        sessions: [
          makeSession({
            id: "s1",
            badge: "waiting_approval",
            created_at: "2026-04-10T08:00:00.000Z",
            updated_at: "2026-04-10T09:00:00.000Z",
          }),
        ],
        hasMore: true,
        nextCursor: "cur_devbox",
      },
    ];

    const byAttention = mergeTogetherFlatRows(entries, "attention");
    expect(byAttention.map(row => row.computerId)).toEqual(["cmp_devbox", LOCAL_COMPUTER_ID]);

    const byActivity = mergeTogetherFlatRows(entries, "last_activity");
    expect(byActivity.map(row => row.computerId)).toEqual([LOCAL_COMPUTER_ID, "cmp_devbox"]);

    const computerMap = buildTogetherComputerMap(entries);
    expect(computerMap.has("cmp_devbox::s1")).toBe(true);
    expect(computerMap.has("local::s1")).toBe(false);

    expect(selectNextTogetherComputer(entries)).toEqual({
      computerId: "cmp_devbox",
      computerName: "devbox",
      nextCursor: "cur_devbox",
    });
  });
});
