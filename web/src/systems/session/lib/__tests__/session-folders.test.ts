// Suite: session-folders
// Invariant: the workspace list groups provenance threads by their root's folder
// in served order (children follow their parent, empty folders show unless a
// filter or the archive narrows the list), collapsed folders drop out of the
// visible row order (the session-cycle shortcut follows the same projection),
// and optimistic layout edits only move what they name.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionFolderPayload, SessionPayload } from "../../types";
import {
  applyFolderRemoval,
  applySessionMove,
  applySessionOrder,
  buildSessionListSections,
  placeId,
  sectionsVisibleOrder,
  shiftId,
  visibleSessionOrder,
} from "../session-folders";
import { buildSessionListThreads } from "../session-list-threads";

function session(id: string, folderId?: string, parentId?: string): SessionPayload {
  return {
    ...primarySessionFixture,
    id,
    name: id,
    archived_at: null,
    ...(folderId ? { folder_id: folderId } : {}),
    ...(parentId
      ? {
          lineage: {
            parent_session_id: parentId,
            root_session_id: parentId,
            spawn_depth: 1,
            auto_stop_on_parent: false,
            notify_creator: false,
            spawn_budget: { max_children: 0, max_depth: 0, ttl_seconds: 0 },
            permission_policy: { tools: [], skills: [], mcp_servers: [], workspace_paths: [] },
          },
        }
      : {}),
  } as SessionPayload;
}

function folder(id: string, name = id): SessionFolderPayload {
  return {
    id,
    name,
    workspace_id: "ws",
    profile_id: "default",
    position: 0,
    session_count: 0,
    created_at: "2026-10-17T00:00:00Z",
    updated_at: "2026-10-17T00:00:00Z",
  };
}

const ids = (sessions: readonly { session: SessionPayload }[]) => sessions.map(t => t.session.id);

describe("buildSessionListSections", () => {
  it("Should group threads by folder in served order and keep children with their parent", () => {
    const threads = buildSessionListThreads(
      [
        session("root-a"),
        session("in-b", "b"),
        session("child", undefined, "in-b"),
        session("in-a", "a"),
        session("in-b-2", "b"),
      ],
      ""
    );
    const sections = buildSessionListSections(threads, [folder("a"), folder("b"), folder("c")], {
      filtering: false,
      archived: false,
    });
    expect(sections.folders.map(section => section.folder.id)).toEqual(["a", "b", "c"]);
    expect(ids(sections.folders[0]!.threads)).toEqual(["in-a"]);
    expect(ids(sections.folders[1]!.threads)).toEqual(["in-b", "in-b-2"]);
    expect(sections.folders[1]!.threads[0]!.childSessions.map(child => child.id)).toEqual([
      "child",
    ]);
    expect(sections.folders[2]!.threads).toEqual([]);
    expect(ids(sections.root)).toEqual(["root-a"]);
  });

  it.each([
    { filtering: true, archived: false },
    { filtering: false, archived: true },
  ])("Should hide empty folders while narrowed (%o)", options => {
    const threads = buildSessionListThreads([session("in-a", "a")], "");
    const sections = buildSessionListSections(threads, [folder("a"), folder("empty")], options);
    expect(sections.folders.map(section => section.folder.id)).toEqual(["a"]);
  });

  it("Should leave a session in the root when its folder is not loaded", () => {
    const threads = buildSessionListThreads([session("stray", "gone")], "");
    const sections = buildSessionListSections(threads, [], { filtering: false, archived: false });
    expect(ids(sections.root)).toEqual(["stray"]);
  });
});

describe("sectionsVisibleOrder", () => {
  it("Should skip a collapsed folder's rows unless a filter forces folders open", () => {
    const threads = buildSessionListThreads(
      [session("in-a", "a"), session("in-b", "b"), session("root")],
      ""
    );
    const sections = buildSessionListSections(threads, [folder("a"), folder("b")], {
      filtering: false,
      archived: false,
    });
    expect(sectionsVisibleOrder(sections, new Set(), new Set(["a"]), false)).toEqual([
      "in-b",
      "root",
    ]);
    expect(sectionsVisibleOrder(sections, new Set(), new Set(["a"]), true)).toEqual([
      "in-a",
      "in-b",
      "root",
    ]);
  });
});

describe("layout order helpers", () => {
  it("Should shift one id and refuse moves past the edges", () => {
    expect(shiftId(["a", "b", "c"], "b", -1)).toEqual(["b", "a", "c"]);
    expect(shiftId(["a", "b", "c"], "b", 1)).toEqual(["a", "c", "b"]);
    expect(shiftId(["a", "b"], "a", -1)).toBeNull();
    expect(shiftId(["a", "b"], "b", 1)).toBeNull();
  });

  it("Should place an id before or after a target, from inside or outside the container", () => {
    expect(placeId(["a", "b", "c"], "c", "a", "before")).toEqual(["c", "a", "b"]);
    expect(placeId(["a", "b", "c"], "a", "c", "after")).toEqual(["b", "c", "a"]);
    expect(placeId(["a", "b"], "x", "a", "after")).toEqual(["a", "x", "b"]);
    expect(placeId(["a", "b", "c"], "a", "b", "before")).toBeNull();
    expect(placeId(["a", "b"], "a", "a", "after")).toBeNull();
  });

  it("Should reorder only the named sessions and give them the container's folder", () => {
    const page = [session("x"), session("a", "f"), session("y"), session("b")];
    const next = applySessionOrder(page, "f", ["b", "a"]);
    expect(next.map(item => item.id)).toEqual(["x", "b", "y", "a"]);
    expect(next.find(item => item.id === "b")?.folder_id).toBe("f");
    const root = applySessionOrder(next, null, ["a"]);
    expect(root.find(item => item.id === "a")?.folder_id).toBeUndefined();
  });

  it("Should move a session to the end of its new folder and clear a deleted folder", () => {
    const page = [session("m"), session("a", "f"), session("b", "f"), session("z")];
    expect(applySessionMove(page, "m", "f").map(item => [item.id, item.folder_id])).toEqual([
      ["a", "f"],
      ["b", "f"],
      ["m", "f"],
      ["z", undefined],
    ]);
    expect(applyFolderRemoval(page, "f").every(item => item.folder_id === undefined)).toBe(true);
  });
});

describe("visibleSessionOrder", () => {
  const base = {
    collapsedThreadIds: new Set<string>(),
    collapsedWorkspaceIds: new Set<string>(),
    workspaceGroups: [],
  };
  const order = (sessions: readonly SessionPayload[]) => sessions.map(item => item.id);

  it("Should match workspace and all-workspace visible row order [UT-065]", () => {
    const rootA = session("sess-a");
    const childA = session("sess-a-child", undefined, "sess-a");
    const rootB = session("sess-b");

    expect(
      order(visibleSessionOrder([rootA, childA, rootB], { ...base, scope: "workspace" }))
    ).toEqual(["sess-a", "sess-a-child", "sess-b"]);
    expect(
      order(
        visibleSessionOrder([rootA, childA, rootB], {
          ...base,
          scope: "workspace",
          collapsedThreadIds: new Set(["sess-a"]),
        })
      )
    ).toEqual(["sess-a", "sess-b"]);
    expect(
      order(
        visibleSessionOrder([], {
          ...base,
          scope: "all-workspaces",
          collapsedWorkspaceIds: new Set(["ws-hidden"]),
          workspaceGroups: [
            { workspaceId: "ws-hidden", sessions: [rootA] },
            { workspaceId: "ws-visible", sessions: [rootB] },
          ],
        })
      )
    ).toEqual(["sess-b"]);
  });

  it("Should follow folder sections and skip sessions inside a collapsed folder", () => {
    const page = [session("r1"), session("a1", "fa"), session("b1", "fb"), session("a2", "fa")];
    const options = { ...base, scope: "workspace" as const, folders: [folder("fa"), folder("fb")] };

    expect(order(visibleSessionOrder(page, options))).toEqual(["a1", "a2", "b1", "r1"]);
    expect(
      order(visibleSessionOrder(page, { ...options, collapsedFolderIds: new Set(["fa"]) }))
    ).toEqual(["b1", "r1"]);
  });
});
