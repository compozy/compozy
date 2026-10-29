// Suite: session-folders
// Invariant: a single-pair list groups provenance threads by their root's folder
// in served order (children follow their parent, empty folders show unless a
// filter or the archive narrows the list), collapsed folders drop out of the
// visible row order (the session-cycle shortcut follows the same projection),
// optimistic layout edits only move what they name, and a selection moves only
// its thread roots, in on-screen order, writing only what would change.
// Project folders (#1044) arrange project groups the same way: folders first in
// their order, then placed root projects, then never-placed ones as served.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionFolderPayload, SessionPayload } from "../../types";
import {
  applyFolderRemoval,
  applySessionMove,
  applySessionOrder,
  applySessionsMovedFirst,
  buildSessionListSections,
  placeId,
  sectionsVisibleOrder,
  sessionBulkMove,
  sessionBulkMoveBlockedReason,
  sessionBulkMoveIds,
  shiftId,
  visibleSessionOrder,
} from "../session-folders";
import { buildSessionListThreads } from "../session-list-threads";
import { projectSessionGroups } from "../session-list-groups";
import {
  applyWorkspaceFolderRemoval,
  applyWorkspaceMove,
  applyWorkspaceOrder,
  buildProjectGroupTree,
  withPlacedProjectGroups,
  projectGroupContainerIds,
} from "../workspace-folders";

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
  const base = { collapsedThreadIds: new Set<string>() };
  const order = (sessions: readonly SessionPayload[]) => sessions.map(item => item.id);

  it("Should match flat and grouped visible row order [UT-065]", () => {
    const rootA = session("sess-a");
    const childA = session("sess-a-child", undefined, "sess-a");
    const rootB = session("sess-b");

    expect(order(visibleSessionOrder([rootA, childA, rootB], base))).toEqual([
      "sess-a",
      "sess-a-child",
      "sess-b",
    ]);
    expect(
      order(
        visibleSessionOrder([rootA, childA, rootB], {
          ...base,
          collapsedThreadIds: new Set(["sess-a"]),
        })
      )
    ).toEqual(["sess-a", "sess-b"]);
    // A grouped list cycles each open group's own page; a collapsed group is skipped.
    expect(
      order(
        visibleSessionOrder([], {
          ...base,
          collapsedGroupKeys: new Set(["project:hidden"]),
          groups: [
            { key: "project:hidden", sessions: [rootA] },
            { key: "project:visible", sessions: [rootB] },
            { key: "user", sessions: [rootA, childA] },
          ],
        })
      )
    ).toEqual(["sess-b", "sess-a", "sess-a-child"]);
  });

  it("Should follow folder sections and skip sessions inside a collapsed folder", () => {
    const page = [session("r1"), session("a1", "fa"), session("b1", "fb"), session("a2", "fa")];
    const options = { ...base, folders: [folder("fa"), folder("fb")] };

    expect(order(visibleSessionOrder(page, options))).toEqual(["a1", "a2", "b1", "r1"]);
    expect(
      order(visibleSessionOrder(page, { ...options, collapsedFolderIds: new Set(["fa"]) }))
    ).toEqual(["b1", "r1"]);
  });

  it("Should apply folders only inside the group whose page is one pair", () => {
    const options = {
      ...base,
      collapsedFolderIds: new Set(["fa"]),
      groups: [
        // A pair group: its folders apply, so the collapsed folder hides a-filed.
        {
          key: "a",
          sessions: [session("a-filed", "fa"), session("a-root")],
          folders: [folder("fa")],
        },
        // No pair: folder members render as plain rows in served order.
        { key: "b", sessions: [session("b-filed", "fb"), session("b-root")] },
      ],
    };
    expect(order(visibleSessionOrder([], options))).toEqual(["a-root", "b-filed", "b-root"]);
    expect(order(visibleSessionOrder([], { ...options, filtering: true }))).toEqual([
      "a-filed",
      "a-root",
      "b-filed",
      "b-root",
    ]);
  });

  it("Should keep a search match inside a collapsed folder, as the list shows it", () => {
    const results = [session("a1", "fa"), session("r1")];
    const options = {
      ...base,
      folders: [folder("fa"), folder("fb")],
      collapsedFolderIds: new Set(["fa"]),
    };

    expect(order(visibleSessionOrder(results, { ...options, filtering: true }))).toEqual([
      "a1",
      "r1",
    ]);
    expect(order(visibleSessionOrder(results, options))).toEqual(["r1"]);
  });
});

describe("applySessionsMovedFirst", () => {
  const order = (sessions: readonly SessionPayload[]) =>
    sessions.map(entry => [entry.id, entry.folder_id ?? null]);

  it("Should lead the folder's members with the moved sessions in listed order", () => {
    const page = [session("r1"), session("f1", "fa"), session("f2", "fa"), session("r2")];

    expect(order(applySessionsMovedFirst(page, "fa", ["r2", "r1"]))).toEqual([
      ["r2", "fa"],
      ["r1", "fa"],
      ["f1", "fa"],
      ["f2", "fa"],
    ]);
    expect(order(applySessionsMovedFirst(page, "fb", ["f2", "r1"]))).toEqual([
      ["f2", "fb"],
      ["r1", "fb"],
      ["f1", "fa"],
      ["r2", null],
    ]);
  });

  it("Should keep slots for a move to the root, whose unplaced sessions lead", () => {
    const page = [session("r1"), session("f1", "fa"), session("r2")];

    expect(order(applySessionsMovedFirst(page, null, ["f1"]))).toEqual([
      ["r1", null],
      ["f1", null],
      ["r2", null],
    ]);
  });
});

describe("sessionBulkMove", () => {
  const subagent = (id: string, parentId: string): SessionPayload => {
    const base = session(id, undefined, parentId);
    return { ...base, lineage: { ...base.lineage!, spawn_role: "subagent" } };
  };

  it("Should move selected thread roots in on-screen order, hidden ones last in catalog order", () => {
    const catalog = [
      session("r1"),
      session("child", undefined, "r1"),
      session("hidden-b"),
      session("f1", "fa"),
      session("hidden-a"),
    ];
    const move = sessionBulkMove(catalog, new Set(["hidden-a", "r1", "child", "f1", "hidden-b"]), [
      "f1",
      "r1",
      "child",
    ]);

    expect(move.sessions.map(entry => entry.id)).toEqual(["f1", "r1", "hidden-b", "hidden-a"]);
    expect(move.profileIds).toEqual([primarySessionFixture.profile_id ?? ""]);
    expect(sessionBulkMoveBlockedReason(move)).toBeNull();
  });

  it("Should write only sessions outside the target container", () => {
    const sessions = [session("f1", "fa"), session("r1"), session("f2", "fb")];

    expect(sessionBulkMoveIds(sessions, "fa")).toEqual(["r1", "f2"]);
    expect(sessionBulkMoveIds(sessions, null)).toEqual(["f1", "f2"]);
    expect(sessionBulkMoveIds([session("f1", "fa")], "fa")).toEqual([]);
  });

  it("Should block a selection of child rows or one spanning profiles", () => {
    const children = sessionBulkMove(
      [session("r1"), session("child", undefined, "r1"), subagent("orphan", "gone")],
      new Set(["child", "orphan"]),
      []
    );
    expect(children.sessions).toEqual([]);
    expect(sessionBulkMoveBlockedReason(children)).toBe("Child sessions move with their parent.");

    const profiles = sessionBulkMove(
      [
        { ...session("a"), profile_id: "p1" },
        { ...session("b"), profile_id: "p2" },
      ],
      new Set(["a", "b"]),
      ["a", "b"]
    );
    expect(sessionBulkMoveBlockedReason(profiles)).toBe(
      "Select sessions from one profile to move them."
    );
  });
});

describe("project folders", () => {
  const facet = (all: number) => ({
    all,
    working: 0,
    needs_you: 0,
    idle: all,
    finished: 0,
    terminal_approvals: 0,
    board: { needs_you: 0, working: 0, in_review: 0, done: 0 },
  });
  const groups = projectSessionGroups(
    {
      by_workspace: ["ws-a", "ws-b", "ws-c", ""].map(id => ({
        workspace_id: id,
        facets: facet(1),
      })),
    },
    {
      workspaces: [
        { id: "ws-a", name: "A" },
        { id: "ws-b", name: "B" },
        { id: "ws-c", name: "C" },
      ],
      lens: { profile: "default" },
    }
  );
  const folder = (id: string, position: number) => ({
    id,
    name: id,
    position,
    profile_id: "default",
    workspace_count: 0,
    created_at: "2026-10-29T00:00:00Z",
    updated_at: "2026-10-29T00:00:00Z",
  });
  const layout = {
    folders: [folder("f2", 1), folder("f1", 0)],
    placements: [
      { workspace_id: "ws-c", folder_id: "f1", position: 1 },
      { workspace_id: "ws-a", folder_id: "f1", position: 0 },
      { workspace_id: "@user", position: 0 },
      // A placement into a folder the read no longer lists falls back to the root.
      { workspace_id: "ws-b", folder_id: "gone", position: 0 },
    ],
  };

  it("Should put folders first, then placed root projects, then the rest as served", () => {
    const tree = buildProjectGroupTree(groups, layout);
    expect(tree.folders.map(node => [node.folder.id, node.groups.map(group => group.key)])).toEqual(
      [
        ["f1", ["project:ws-a", "project:ws-c"]],
        ["f2", []],
      ]
    );
    expect(tree.root.map(group => group.key)).toEqual(["user", "project:ws-b"]);
    expect(projectGroupContainerIds(tree, null)).toEqual(["@user", "ws-b"]);
  });

  it("Should keep a placed project with no sessions in scope so its folder can still move it", () => {
    const served = groups.filter(group => group.key !== "project:ws-c");
    const kept = withPlacedProjectGroups(served, layout.placements, {
      workspaces: [
        { id: "ws-a", name: "A" },
        { id: "ws-b", name: "B" },
        { id: "ws-c", name: "C" },
      ],
      lens: { profile: "default" },
    });
    const tree = buildProjectGroupTree(kept, layout);
    expect(tree.folders[0]!.groups.map(group => [group.key, group.total])).toEqual([
      ["project:ws-a", 1],
      ["project:ws-c", 0],
    ]);
    // An unregistered placement stays out.
    const unknown = withPlacedProjectGroups(served, [{ workspace_id: "ws-gone", position: 0 }], {
      workspaces: [],
      lens: { profile: "default" },
    });
    expect(unknown).toHaveLength(served.length);
  });

  it("Should move, order, and unfile projects optimistically without touching others", () => {
    const moved = applyWorkspaceMove(layout, "@user", "f1");
    expect(moved.placements).toContainEqual({
      workspace_id: "@user",
      folder_id: "f1",
      position: 2,
    });

    const ordered = applyWorkspaceOrder(layout, "f1", ["ws-c"]);
    expect(
      ordered.placements
        .filter(row => row.folder_id === "f1")
        .sort((l, r) => l.position - r.position)
    ).toEqual([
      { workspace_id: "ws-c", folder_id: "f1", position: 0 },
      { workspace_id: "ws-a", folder_id: "f1", position: 1 },
    ]);
    expect(ordered.placements).toContainEqual({ workspace_id: "@user", position: 0 });

    const removed = applyWorkspaceFolderRemoval(layout, "f1");
    expect(removed.folders.map(row => row.id)).toEqual(["f2"]);
    expect(removed.placements.map(row => row.workspace_id)).toEqual(["@user", "ws-b"]);
  });
});
