// Suite: session-hierarchy
// Invariant: sessions nest under their loaded creation parent (orphans stay roots),
// thread collection flattens descendants in list order, and collapsed-thread
// signals surface the most urgent child state.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { sessionRuntime } from "../../mocks/fixtures";
import type { SessionPayload } from "../../types";
import {
  buildSessionTree,
  childSessionSignalTone,
  collectThreadSessions,
  filterThreadSessions,
} from "../session-hierarchy";

function treeSession(
  id: string,
  options: { parent?: string; badge?: string } = {}
): SessionPayload {
  return {
    id,
    name: `Session ${id}`,
    agent_name: "coder",
    runtime: sessionRuntime("claude"),
    workspace_id: "ws-1",
    state: "active",
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
            spawn_depth: 1,
            auto_stop_on_parent: false,
            spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
            permission_policy: {
              tools: [],
              skills: [],
              mcp_servers: [],
              workspace_paths: [],
              network_channels: [],
              sandbox_profiles: [],
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

describe("childSessionSignalTone", () => {
  it("Should rank the needs-you class above runtime health above running", () => {
    const running = treeSession("sess-running", { parent: "p", badge: "running" });
    const hung = treeSession("sess-hung", { parent: "p", badge: "hung" });
    const waiting = treeSession("sess-waiting", { parent: "p", badge: "waiting-for-auth" });
    const asking = treeSession("sess-asking", { parent: "p", badge: "waiting-for-input" });
    const failed = treeSession("sess-failed", { parent: "p", badge: "failed" });
    const stopped = treeSession("sess-stopped", { parent: "p", badge: "stopped" });

    expect(childSessionSignalTone([stopped])).toBeNull();
    expect(childSessionSignalTone([stopped, running])).toBe("accent");
    expect(childSessionSignalTone([running, hung])).toBe("warning");
    expect(childSessionSignalTone([running, hung, waiting])).toBe("danger");
    expect(childSessionSignalTone([running, asking])).toBe("danger");
    expect(childSessionSignalTone([running, failed])).toBe("danger");
  });

  it("Should leave finished-unseen work unsignalled — done is an inbox item, not an escalation", () => {
    const done = treeSession("sess-done", { parent: "p", badge: "done" });
    const running = treeSession("sess-running", { parent: "p", badge: "running" });

    expect(childSessionSignalTone([done])).toBeNull();
    expect(childSessionSignalTone([done, running])).toBe("accent");
  });
});
