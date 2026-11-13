// Suite: the one project menu tree
// Invariant: every project surface draws New ▸, Open files, Move ▸, then Archive and Remove
// (destructive, last); a verb the surface cannot reach is absent, never a dead row; an archived
// project offers only Unarchive and Remove; Move ▸ never offers the project's own folder and
// offers Move out only from inside one.
// Owning layer: workspace lib. Canonical suite: this file (surfaces only pass verbs).
import { describe, expect, it, vi } from "vitest";

import type { MenuTreeNode } from "@compozy/ui";

import { projectMenuTree, type ProjectMenuVerbs } from "../project-menu";

const PROJECT = { id: "ws-alpha", archived_at: null };

function labels(nodes: readonly MenuTreeNode[]): string[] {
  return nodes.map(node => (node.kind === "separator" ? "—" : node.label));
}

function child(nodes: readonly MenuTreeNode[], id: string): readonly MenuTreeNode[] {
  const node = nodes.find(candidate => candidate.id === id);
  if (!node || (node.kind !== "sub" && node.kind !== "group")) throw new Error(`no ${id}`);
  return node.children;
}

const ALL: ProjectMenuVerbs = {
  newSession: vi.fn(),
  newFolder: vi.fn(),
  addAlwaysOn: vi.fn(),
  openFiles: vi.fn(),
  move: {
    folders: [
      { id: "wf-1", name: "Clients" },
      { id: "wf-2", name: "Labs" },
    ],
    folderId: "wf-1",
    onMove: vi.fn(),
    order: { down: vi.fn() },
  },
};

describe("projectMenuTree", () => {
  it("Should draw the approved order with every verb reachable", () => {
    const nodes = projectMenuTree({ workspace: PROJECT, verbs: ALL, choose: vi.fn() });

    expect(labels(nodes)).toEqual([
      "New",
      "Open files",
      "—",
      "Move",
      "—",
      "Archive project",
      "Remove project",
    ]);
    expect(labels(child(nodes, "new"))).toEqual([
      "New session",
      "New folder",
      "Add always-on agent",
    ]);
    expect(nodes.at(-1)).toMatchObject({ kind: "item", destructive: true });
  });

  it("Should hide unreachable verbs and the submenus they leave empty", () => {
    const nodes = projectMenuTree({ workspace: PROJECT, verbs: {}, choose: vi.fn() });

    expect(labels(nodes)).toEqual(["Archive project", "Remove project"]);
  });

  it("Should offer only Unarchive and Remove on an archived project", () => {
    const choose = vi.fn();
    const nodes = projectMenuTree({
      workspace: { id: "ws-alpha", archived_at: "2026-11-01T00:00:00Z" },
      verbs: ALL,
      choose,
    });

    expect(labels(nodes)).toEqual(["Unarchive project", "Remove project"]);
    const unarchive = nodes[0];
    if (unarchive?.kind === "item") unarchive.onSelect?.();
    expect(choose).toHaveBeenCalledWith("archive");
  });

  it("Should move to other folders or out of the current one, and disable a missing step", () => {
    const move = child(
      projectMenuTree({ workspace: PROJECT, verbs: ALL, choose: vi.fn() }),
      "move"
    );

    expect(labels(child(move, "folder"))).toEqual(["Labs", "Move out of folder"]);
    expect(move.filter(node => node.kind === "item")).toMatchObject([
      { label: "Move project up", disabled: true },
      { label: "Move project down", disabled: false },
    ]);
  });

  it("Should keep only the surface's verbs without a project", () => {
    const nodes = projectMenuTree({
      workspace: undefined,
      verbs: {
        move: { folders: [{ id: "wf-1", name: "Clients" }], folderId: null, onMove: vi.fn() },
      },
      choose: vi.fn(),
    });

    expect(labels(nodes)).toEqual(["Move"]);
    expect(labels(child(child(nodes, "move"), "folder"))).toEqual(["Clients"]);
  });
});
