// Suite: session-hierarchy
// Invariant: sessions nest under their loaded hierarchy parent (orphans stay roots;
// continued and forked sessions are top-level, their source is provenance only),
// thread collection flattens descendants in list order, and collapsed-thread
// signals surface the most urgent child state.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { sessionRuntime } from "../../mocks/fixtures";
import type { SessionPayload } from "../../types";
import {
  buildSessionTree,
  childSessionSignalState,
  collectThreadSessions,
  filterThreadSessions,
  visibleSessionOrder,
} from "../session-hierarchy";
import { subagentContextRequests, withSubagentContext } from "../session-subagent-context";

function treeSession(
  id: string,
  options: {
    parent?: string;
    kind?: NonNullable<SessionPayload["lineage"]>["kind"];
    badge?: string;
    sessionType?: SessionPayload["type"];
  } = {}
): SessionPayload {
  return {
    supervision: null,
    profile_id: "00000000000000000000000000",
    profile_name: "default",
    id,
    name: `Session ${id}`,
    agent_name: "coder",
    runtime: sessionRuntime("claude"),
    workspace_id: "ws-1",
    state: "active",
    type: options.sessionType ?? "user",
    badge: options.badge ?? "running",
    attachable: true,
    archived_at: null,
    available_commands: [],
    pending_interactions: [],
    ...(options.parent
      ? {
          lineage: {
            parent_session_id: options.parent,
            root_session_id: options.parent,
            ...(options.kind ? { kind: options.kind } : {}),
            spawn_depth: 1,
            auto_stop_on_parent: false,
            notify_creator: true,
            spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
            permission_policy: {
              tools: [],
              skills: [],
              mcp_servers: [],
              workspace_paths: [],
            },
          },
        }
      : {}),
    created_at: "2026-04-17T12:00:00Z",
    updated_at: "2026-04-17T18:00:00Z",
  };
}

describe("buildSessionTree", () => {
  it("Should nest children under a loaded parent and keep orphans as roots", () => {
    const root = treeSession("sess-root");
    const child = treeSession("sess-child", { parent: "sess-root" });
    const orphan = treeSession("sess-orphan", { parent: "sess-unloaded" });

    const tree = buildSessionTree([root, child, orphan]);

    expect(tree.roots.map(session => session.id)).toEqual(["sess-root", "sess-orphan"]);
    expect(tree.childrenByParent.get("sess-root")?.map(session => session.id)).toEqual([
      "sess-child",
    ]);
  });

  it("Should nest a system Goal under its loaded origin session", () => {
    const origin = treeSession("sess-origin");
    const goal = treeSession("sess-goal", {
      parent: "sess-origin",
      sessionType: "system",
    });

    const tree = buildSessionTree([origin, goal]);

    expect(tree.roots.map(session => session.id)).toEqual(["sess-origin"]);
    expect(tree.childrenByParent.get("sess-origin")?.map(session => session.id)).toEqual([
      "sess-goal",
    ]);
  });

  it("Should keep continued and forked sessions top-level while spawned children nest", () => {
    const source = treeSession("sess-source");
    const continued = treeSession("sess-continued", { parent: "sess-source", kind: "continue" });
    const forked = treeSession("sess-forked", { parent: "sess-source", kind: "fork" });
    const spawned = treeSession("sess-spawned", { parent: "sess-source", kind: "spawn" });
    const spawnedByContinued = treeSession("sess-worker", {
      parent: "sess-continued",
      kind: "spawn",
    });

    const tree = buildSessionTree([source, continued, forked, spawned, spawnedByContinued]);

    expect(tree.roots.map(session => session.id)).toEqual([
      "sess-source",
      "sess-continued",
      "sess-forked",
    ]);
    expect(tree.childrenByParent.get("sess-source")?.map(session => session.id)).toEqual([
      "sess-spawned",
    ]);
    expect(tree.childrenByParent.get("sess-continued")?.map(session => session.id)).toEqual([
      "sess-worker",
    ]);
  });

  it("Should keep a self-parented session as a root", () => {
    const cyclic = treeSession("sess-cyclic", { parent: "sess-cyclic" });
    const tree = buildSessionTree([cyclic]);
    expect(tree.roots.map(session => session.id)).toEqual(["sess-cyclic"]);
    expect(tree.childrenByParent.size).toBe(0);
  });

  it("Should keep every participant in a multi-session cycle visible as a root", () => {
    const first = treeSession("sess-first", { parent: "sess-second" });
    const second = treeSession("sess-second", { parent: "sess-first" });
    const child = treeSession("sess-child", { parent: "sess-first" });

    const tree = buildSessionTree([first, second, child]);

    expect(tree.roots.map(session => session.id)).toEqual(["sess-first", "sess-second"]);
    expect(tree.childrenByParent.get("sess-first")?.map(session => session.id)).toEqual([
      "sess-child",
    ]);
  });
});

describe("collectThreadSessions", () => {
  it("Should flatten deep descendants into their root thread in depth-first order", () => {
    const sessions = [
      treeSession("sess-root"),
      treeSession("sess-a", { parent: "sess-root" }),
      treeSession("sess-a1", { parent: "sess-a" }),
      treeSession("sess-b", { parent: "sess-root" }),
    ];
    const tree = buildSessionTree(sessions);

    expect(collectThreadSessions("sess-root", tree.childrenByParent).map(s => s.id)).toEqual([
      "sess-a",
      "sess-a1",
      "sess-b",
    ]);
    expect(collectThreadSessions("sess-b", tree.childrenByParent)).toEqual([]);
  });

  it("Should retain one matching descendant and its ancestors in linear tree order", () => {
    const sessions = [
      treeSession("sess-root"),
      treeSession("sess-parent", { parent: "sess-root" }),
      treeSession("sess-match", { parent: "sess-parent" }),
      treeSession("sess-sibling", { parent: "sess-root" }),
    ];
    const tree = buildSessionTree(sessions);

    expect(
      filterThreadSessions(
        sessions[0]!,
        tree.childrenByParent,
        session => session.id === "sess-match"
      )?.map(session => session.id)
    ).toEqual(["sess-parent", "sess-match"]);
  });
});

describe("visibleSessionOrder", () => {
  it("Should match workspace and all-workspace visible row order [UT-065]", () => {
    const rootA = treeSession("sess-a");
    const childA = treeSession("sess-a-child", { parent: "sess-a" });
    const rootB = { ...treeSession("sess-b"), agent_name: "reviewer" };
    const base = {
      collapsedThreadIds: new Set<string>(),
      collapsedWorkspaceIds: new Set<string>(),
      workspaceGroups: [],
    };

    expect(
      visibleSessionOrder([rootA, childA, rootB], { ...base, scope: "workspace" }).map(
        session => session.id
      )
    ).toEqual(["sess-a", "sess-a-child", "sess-b"]);
    expect(
      visibleSessionOrder([rootA, childA, rootB], {
        ...base,
        scope: "workspace",
        collapsedThreadIds: new Set(["sess-a"]),
      }).map(session => session.id)
    ).toEqual(["sess-a", "sess-b"]);
    expect(
      visibleSessionOrder([], {
        ...base,
        scope: "all-workspaces",
        collapsedWorkspaceIds: new Set(["ws-hidden"]),
        workspaceGroups: [
          { workspaceId: "ws-hidden", sessions: [rootA] },
          { workspaceId: "ws-visible", sessions: [rootB] },
        ],
      }).map(session => session.id)
    ).toEqual(["sess-b"]);
  });
});

describe("childSessionSignalState", () => {
  it("Should rank waiting on the operator above failures above running", () => {
    const running = treeSession("sess-running", { parent: "p", badge: "running" });
    const hung = treeSession("sess-hung", { parent: "p", badge: "hung" });
    const waiting = treeSession("sess-waiting", { parent: "p", badge: "waiting-for-auth" });
    const asking = treeSession("sess-asking", { parent: "p", badge: "waiting-for-input" });
    const failed = treeSession("sess-failed", { parent: "p", badge: "failed" });
    const stopped = treeSession("sess-stopped", { parent: "p", badge: "stopped" });
    const unverified = treeSession("sess-unverified", { parent: "p", badge: "needs-attention" });

    expect(childSessionSignalState([stopped])).toBeNull();
    expect(childSessionSignalState([stopped, running])).toBe("running");
    expect(childSessionSignalState([running, hung])).toBe("failed");
    expect(childSessionSignalState([running, failed])).toBe("failed");
    expect(childSessionSignalState([running, hung, waiting])).toBe("attention");
    expect(childSessionSignalState([running, asking])).toBe("attention");
    expect(childSessionSignalState([failed, asking])).toBe("attention");
    // An unverified stop needs the operator: it escalates like any needs-you ask.
    expect(childSessionSignalState([running, unverified])).toBe("attention");
  });

  it("Should leave finished-unseen work unsignalled — done is an inbox item, not an escalation", () => {
    const done = treeSession("sess-done", { parent: "p", badge: "done" });
    const running = treeSession("sess-running", { parent: "p", badge: "running" });

    expect(childSessionSignalState([done])).toBeNull();
    expect(childSessionSignalState([done, running])).toBe("running");
  });
});

// S9: subagent sessions are off the page; context rules decide which ancestors to read and
// where the rows land, so a subagent always nests and never becomes a root.
describe("subagent context", () => {
  const subagent = (id: string, parent: string): SessionPayload => {
    const session = treeSession(id, { parent });
    return { ...session, lineage: { ...session.lineage!, spawn_role: "subagent" } };
  };
  const ids = (sessions: readonly SessionPayload[]) => sessions.map(session => session.id);

  it("Should request each missing hop and nest a depth-3 reveal under the on-page ancestor", () => {
    const root = treeSession("root");
    const a = subagent("a", "root");
    const b = subagent("b", "a");
    const viewed = subagent("viewed", "b");
    const input = { sessions: [root], revealed: viewed, searching: false };

    expect(subagentContextRequests({ ...input, loaded: new Map() })).toEqual([
      { sessionId: "b", workspaceId: "ws-1" },
    ]);
    expect(subagentContextRequests({ ...input, loaded: new Map([["b", b]]) })).toEqual([
      { sessionId: "a", workspaceId: "ws-1" },
    ]);
    const loaded = new Map([
      ["a", a],
      ["b", b],
    ]);
    expect(subagentContextRequests({ ...input, loaded })).toEqual([]);
    const listed = withSubagentContext({ ...input, loaded });
    expect(ids(listed)).toEqual(["root", "a", "b", "viewed"]);
    expect(buildSessionTree(listed).roots.map(session => session.id)).toEqual(["root"]);
  });

  it("Should reveal nothing when the chain reaches a non-subagent ancestor off the page", () => {
    const offPage = treeSession("off-page");
    const viewed = subagent("viewed", "off-page");
    const listed = withSubagentContext({
      sessions: [treeSession("other")],
      loaded: new Map([["off-page", offPage]]),
      revealed: viewed,
      searching: false,
    });

    expect(ids(listed)).toEqual(["other"]);
  });

  it("Should lead a search match's thread with its non-matching ancestor and hold it while loading", () => {
    const parent = treeSession("parent");
    const middle = subagent("middle", "parent");
    const match = subagent("match", "middle");
    const page = [treeSession("other"), match];

    expect(
      ids(withSubagentContext({ sessions: page, loaded: new Map(), searching: true }))
    ).toEqual(["other"]);
    const listed = withSubagentContext({
      sessions: page,
      loaded: new Map([
        ["middle", middle],
        ["parent", parent],
      ]),
      searching: true,
    });
    expect(ids(listed)).toEqual(["other", "parent", "middle", "match"]);
    expect(buildSessionTree(listed).roots.map(session => session.id)).toEqual(["other", "parent"]);
  });
});
