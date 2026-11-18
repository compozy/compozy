import { runSessionBatch, type SessionBatchResult } from "../session-batch";
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import {
  sessionChildrenChoice,
  sessionChildrenScope,
  sessionTreeBatchOrder,
} from "../session-tree";

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

// Invariant: independent trees overlap within a fixed bound, related mutations stay ordered,
// and every attempt retains its receipt. Owner: session batch scheduler; canonical tree suite.
describe("session tree batch execution", () => {
  it("bounds 15 targets, isolates failures and preserves receipts across retries", async () => {
    const ids = Array.from({ length: 15 }, (_, index) => `session-${index}`);
    let active = 0;
    let peak = 0;
    const snapshots: Array<readonly SessionBatchResult[]> = [];
    const results = await runSessionBatch({
      action: "stop",
      ids,
      previous: [],
      onProgress: results => snapshots.push(results),
      execute: async id => {
        active++;
        peak = Math.max(peak, active);
        await Promise.resolve();
        active--;
        if (id === ids[4]) throw new Error("locked");
      },
    });
    expect(peak).toBe(3);
    expect(results.map(result => result.status)).toEqual(
      ids.map((_, index) => (index === 4 ? "failed" : "done"))
    );
    expect(results[4]?.error).toBe("locked");
    expect(snapshots[0]?.every(result => result.status === "pending")).toBe(true);
    const retried: string[] = [];
    const retry = await runSessionBatch({
      action: "stop",
      ids: [ids[4]!],
      previous: results,
      onProgress: () => {},
      execute: async id => {
        retried.push(id);
      },
    });
    expect(retried).toEqual([ids[4]]);
    expect(retry.every(result => result.status === "done")).toBe(true);
  });

  it("serializes a fork and its child using their inherited daemon root", async () => {
    const sessions = [
      row("fork", {
        lineage: {
          ...primarySessionFixture.lineage,
          kind: "fork",
          parent_session_id: "source",
          root_session_id: "source",
        },
      } as Partial<SessionPayload>),
      row("child", {
        lineage: {
          ...primarySessionFixture.lineage,
          kind: "spawn",
          parent_session_id: "fork",
          root_session_id: "source",
        },
      } as Partial<SessionPayload>),
      row("independent", { lineage: undefined }),
    ];
    let release!: () => void;
    const blocked = new Promise<void>(resolve => {
      release = resolve;
    });
    const started: string[] = [];
    const running = runSessionBatch({
      action: "stop",
      ids: sessions.map(session => session.id),
      sessions,
      previous: [],
      onProgress: () => {},
      execute: async id => {
        started.push(id);
        if (id === "fork") await blocked;
      },
    });
    await Promise.resolve();
    expect(started).toEqual(["fork", "independent"]);
    release();
    await running;
    expect(started).toEqual(["fork", "independent", "child"]);
  });

  it("settles a covered child only after its root even if the root failed", async () => {
    const sessions = [row("root"), row("child"), row("other", { lineage: undefined })];
    let release!: () => void;
    const blocked = new Promise<void>(resolve => {
      release = resolve;
    });
    const started: string[] = [];
    const running = runSessionBatch({
      action: "delete",
      ids: sessions.map(session => session.id),
      sessions,
      previous: [],
      coveredBy: new Map([["child", "root"]]),
      onProgress: () => {},
      execute: async id => {
        started.push(id);
        if (id === "root") {
          await blocked;
          throw new Error("root failed");
        }
      },
    });
    await Promise.resolve();
    expect(started).toEqual(["root", "other"]);
    release();
    expect((await running).map(result => result.status)).toEqual(["failed", "done", "done"]);
    expect(started).toEqual(["root", "other", "child"]);
  });
});
