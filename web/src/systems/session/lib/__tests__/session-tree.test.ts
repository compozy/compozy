import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import { sessionChildrenChoice, sessionChildrenScope, sessionTreeBatchOrder } from "../session-tree";

const row = (id: string, patch: Partial<SessionPayload> = {}): SessionPayload =>
  ({ ...primarySessionFixture, id, archived_at: null, ...patch }) as SessionPayload;

describe("sessionChildrenScope", () => {
  it("counts subagents and archived children only for delete", () => {
    const subagent = row("subagent", {
      lineage: { ...primarySessionFixture.lineage, spawn_role: "subagent" },
    } as Partial<SessionPayload>);
    const archived = row("archived", { archived_at: "2026-11-01T00:00:00Z" });
    const trees = new Map([["root", [row("plain"), subagent, archived]]]);
    const ids = (action: "archive" | "delete") =>
      sessionChildrenScope(action, [row("root")], trees).members.map(member => member.id);
    expect(ids("archive")).toEqual(["plain"]);
    expect(ids("delete")).toEqual(["plain", "subagent", "archived"]);
  });

  it("never counts a selected target or a shared child twice", () => {
    const targets = [row("child"), row("root"), row("other")];
    const trees = new Map([
      ["root", [row("child"), row("grandchild")]],
      ["child", [row("grandchild")]],
      ["other", [row("grandchild")]],
    ]);
    const scope = sessionChildrenScope("delete", targets, trees);
    expect(scope.members.map(member => member.id)).toEqual(["grandchild"]);
    expect(scope.coveredBy).toEqual(new Map([["child", "root"]]));
    expect(sessionTreeBatchOrder(targets, scope.coveredBy).map(target => target.id)).toEqual([
      "root",
      "other",
      "child",
    ]);
  });

  it("offers no choice without children", () => {
    const scope = sessionChildrenScope("archive", [row("root")], new Map());
    expect(sessionChildrenChoice(scope, true, () => {})).toBeUndefined();
  });
});
