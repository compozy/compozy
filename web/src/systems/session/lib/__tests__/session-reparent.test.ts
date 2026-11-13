// Suite: session-reparent
// Invariant: Move under… lists only parents the daemon would accept (same project and
// profile, unarchived user sessions, never the session itself or one of its descendants),
// and an ineligible session gets the actionable reason the picker shows instead of a list.
// Continued and forked sessions keep their source as their lineage parent, so they stay put.
// Owning layer: unit (systems/session/lib)
import { describe, expect, it } from "vitest";

import { primarySessionFixture } from "../../testing";
import type { SessionPayload } from "../../types";
import {
  sessionCanMakeTopLevel,
  sessionMoveUnderRefusal,
  sessionParentCandidates,
} from "../session-reparent";

type Lineage = NonNullable<SessionPayload["lineage"]>;

function session(
  id: string,
  overrides: Partial<Omit<SessionPayload, "lineage">> & { lineage?: Partial<Lineage> } = {}
): SessionPayload {
  const { lineage, ...rest } = overrides;
  return {
    ...primarySessionFixture,
    id,
    name: id,
    type: "user",
    archived_at: null,
    ...rest,
    lineage: {
      ...primarySessionFixture.lineage!,
      parent_session_id: "",
      root_session_id: id,
      spawn_depth: 0,
      spawn_role: "",
      kind: "",
      ...lineage,
    },
  } as SessionPayload;
}

describe("sessionMoveUnderRefusal", () => {
  it("Should admit a top-level user session", () => {
    expect(sessionMoveUnderRefusal(session("top"))).toBeNull();
  });

  it.each([
    ["continue", "Continued and forked sessions stay linked"],
    ["fork", "Continued and forked sessions stay linked"],
  ] as const)("Should keep a %s session with its source", (kind, reason) => {
    const derived = session("derived", { lineage: { parent_session_id: "source", kind } });
    expect(sessionMoveUnderRefusal(derived)).toContain(reason);
  });

  it("Should point an adopted child to Make top-level first", () => {
    const child = session("child", { lineage: { parent_session_id: "p", kind: "provenance" } });
    expect(sessionCanMakeTopLevel(child)).toBe(true);
    expect(sessionMoveUnderRefusal(child)).toBe(
      "Make this session top-level first, then move it under another session."
    );
  });

  it.each([
    [
      "subagent",
      session("sub", {
        type: "spawned",
        lineage: { parent_session_id: "p", spawn_role: "subagent", kind: "spawn" },
      }),
      "Subagent sessions stay with their parent.",
    ],
    [
      "spawned worker",
      session("worker", {
        type: "spawned",
        lineage: { parent_session_id: "p", spawn_role: "worker", kind: "spawn" },
      }),
      "Only your own sessions can move under another session.",
    ],
    [
      "archived session",
      session("old", { archived_at: "2026-11-01T00:00:00Z" }),
      "Unarchive this session first.",
    ],
  ])("Should refuse a %s with its reason", (_name, source, reason) => {
    expect(sessionMoveUnderRefusal(source)).toBe(reason);
  });
});

describe("sessionParentCandidates", () => {
  it("Should keep same project and profile parents, never itself or a descendant", () => {
    const source = session("source");
    const peer = session("peer");
    const descendant = session("descendant", {
      lineage: { parent_session_id: "source", root_session_id: "source", kind: "provenance" },
    });
    const otherProfile = session("other-profile", { profile_id: "another-profile" });
    const otherProject = session("other-project", { workspace_id: "another-project" });
    const archived = session("archived", { archived_at: "2026-11-01T00:00:00Z" });
    const subagent = session("subagent", {
      type: "spawned",
      lineage: { parent_session_id: "peer", spawn_role: "subagent", kind: "spawn" },
    });
    const continued = session("continued", {
      lineage: { parent_session_id: "x", kind: "continue" },
    });

    expect(
      sessionParentCandidates(source, [
        source,
        peer,
        descendant,
        otherProfile,
        otherProject,
        archived,
        subagent,
        continued,
      ]).map(candidate => candidate.id)
    ).toEqual(["peer", "continued"]);
  });

  it("Should offer no parents to a session that cannot move", () => {
    const forked = session("forked", { lineage: { parent_session_id: "src", kind: "fork" } });
    expect(sessionParentCandidates(forked, [session("peer")])).toEqual([]);
  });
});
