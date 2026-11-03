import { describe, expect, it } from "vitest";

import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { humanizeAgentSlug, resolveAgentIdentity } from "../agent-identity";
import { agentFromOrphanInstance, isOrphanAgent } from "../agent-instance-wire";
import {
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceCapacity,
  agentNextWakeLine,
  agentPauseCopy,
  agentInstancesByUrgency,
} from "../agent-instance";
import {
  agentSymbolAppearance,
  agentSymbolBackgroundOf,
  agentSymbolOf,
} from "../agent-symbol-appearance";

describe("agentInstanceAvatarState", () => {
  it.each([
    [{ needs_attention: true, state: "paused" }, "needs-you"],
    [{ activity: "waiting" }, "needs-you"],
    [{ state: "paused" }, "paused"],
    [{ state: "starting", activity: "working" }, "starting"],
    [{ activity: "working" }, "working"],
    [{ activity: "failed" }, "failed"],
    [{ activity: "idle" }, "idle"],
    [{ activity: "unknown" }, "unknown"],
  ] as const)("Should read %j as %s", (overrides, state) => {
    expect(agentInstanceAvatarState(agentInstanceFixture(overrides))).toBe(state);
  });
});

describe("agentInstancesByUrgency", () => {
  it("Should keep the daemon's order inside one urgency", () => {
    const first = agentInstanceFixture({ id: "first" });
    const second = agentInstanceFixture({ id: "second" });
    expect(agentInstancesByUrgency([first, second]).map(i => i.id)).toEqual(["first", "second"]);
  });
});

describe("agentInstanceCapacity", () => {
  it("Should block at the daemon's configured limit, not a fixed number", () => {
    expect(agentInstanceCapacity({ limit: 25, active: 10 }).reached).toBe(false);
    expect(agentInstanceCapacity({ limit: 5, active: 5 }).reached).toBe(true);
  });

  it("Should block nothing before the daemon has answered", () => {
    expect(agentInstanceCapacity(undefined).reached).toBe(false);
  });
});

describe("agentPauseCopy", () => {
  it("Should say the fatigue limit the daemon applied", () => {
    const instance = agentInstanceFixture({ pause_reason: "fatigue", fatigue_limit: 3 });
    expect(agentPauseCopy(instance)).toBe("Paused after 3 failed checks in a row.");
  });

  it("Should say how many steps it took without you [UT-057]", () => {
    const instance = agentInstanceFixture({
      state: "paused",
      pause_reason: "unattended",
      unattended_turn_limit: 300,
      unattended_turns: 300,
    });
    expect(agentPauseCopy(instance)).toBe("Paused after 300 steps without you.");
  });
});

describe("agentNextWakeLine", () => {
  const now = Date.parse("2026-11-03T10:00:00Z");

  it("Should say what wakes it and when [UT-060]", () => {
    const instance = agentInstanceFixture({
      next_wake: { kind: "timer", description: "Check CI on #1166", at: "2026-11-03T10:12:00Z" },
    });
    expect(agentNextWakeLine(instance, now)).toBe("Check CI on #1166, in 12m");
  });

  it("Should name the kind when the wake has no description, and nothing when none is set", () => {
    expect(
      agentNextWakeLine(
        agentInstanceFixture({ next_wake: { kind: "check", description: "" } }),
        now
      )
    ).toBe("Its next check-in");
    expect(agentNextWakeLine(agentInstanceFixture({ next_wake: null }), now)).toBeNull();
  });
});

describe("agentAvatarAppearance", () => {
  it("Should let the seed pick the hue of a derived appearance", () => {
    const derived = {
      version: 1,
      avatar: "generated" as const,
      seed: "triage",
      generation: 1,
      color: "#7c5cff",
    };
    expect(agentAvatarAppearance(derived, "triage")).toEqual({
      avatar: "generated",
      seed: "triage",
      generation: 1,
    });
    expect(agentAvatarAppearance({ ...derived, seed: "triage~1" }, "triage")).toMatchObject({
      color: "#7c5cff",
    });
  });

  it("Should keep icon and emoji, and fall back to the slug for anything else", () => {
    expect(
      agentAvatarAppearance({ version: 1, avatar: "icon", icon: "compass", color: "#2aa8a8" }, "a")
    ).toEqual({ avatar: "icon", icon: "compass", color: "#2aa8a8" });
    expect(agentAvatarAppearance(null, "general")).toMatchObject({ seed: "general" });
    expect(agentAvatarAppearance({ version: 1, avatar: "icon" }, "general")).toMatchObject({
      avatar: "generated",
      seed: "general",
    });
  });

  // Invariant: the plate choice survives the wire round trip and the editor's color and symbol
  // picks; only `none` changes the mark. Owner: agent appearance mapping; canonical suite: this file.
  it("Should carry a bare symbol through the mapping and every editor pick", () => {
    const bare = { version: 1, avatar: "icon" as const, icon: "code-xml", color: "#8a8f98" };
    expect(agentAvatarAppearance({ ...bare, background: "none" }, "a")).toEqual({
      avatar: "icon",
      icon: "code-xml",
      color: "#8a8f98",
      background: "none",
    });
    expect(agentAvatarAppearance({ ...bare, background: "plate" }, "a")).not.toHaveProperty(
      "background"
    );

    const picked = agentSymbolAppearance({ kind: "icon", value: "code-xml" }, "#8a8f98", "none");
    expect(agentSymbolBackgroundOf(picked)).toBe("none");
    const recolored = agentSymbolAppearance(agentSymbolOf(picked), "#4ea7fc", "none");
    expect(recolored).toEqual({
      avatar: "icon",
      icon: "code-xml",
      color: "#4ea7fc",
      background: "none",
    });
    expect(agentSymbolAppearance({ kind: "emoji", value: "🦊" }, "#e8b04a", "plate")).toEqual({
      avatar: "emoji",
      emoji: "🦊",
      color: "#e8b04a",
    });
    expect(agentSymbolBackgroundOf({ avatar: "generated", seed: "triage", generation: 1 })).toBe(
      "plate"
    );
  });
});

// Invariant: a slug reads as `AgentDef.Label` does, and no name ever falls through as a raw slug.
// Owner: agent identity resolution; canonical suite: this file.
describe("agent identity", () => {
  it("Should humanize slugs like the daemon's label rule", () => {
    expect(humanizeAgentSlug("code_implementer")).toBe("Code Implementer");
    expect(humanizeAgentSlug("general")).toBe("General");
    expect(humanizeAgentSlug("release-captain.v2")).toBe("Release Captain V2");
  });

  it("Should use the catalog's label and resolved appearance", () => {
    const agent = {
      name: "triage",
      label: "Triage Lead",
      resolved_appearance: { version: 1, avatar: "emoji", emoji: "🦊", color: "#112233" },
    };
    expect(resolveAgentIdentity("triage", [agent as never])).toMatchObject({
      label: "Triage Lead",
      appearance: { avatar: "emoji", emoji: "🦊" },
      known: true,
      missing: false,
    });
  });

  it("Should give a deleted agent its humanized slug and slug-derived face", () => {
    expect(resolveAgentIdentity("release_captain", [])).toMatchObject({
      name: "release_captain",
      label: "Release Captain",
      appearance: { avatar: "generated", seed: "release_captain" },
      known: false,
      missing: true,
    });
  });

  it("Should not call an agent deleted before the catalog answers", () => {
    expect(resolveAgentIdentity("release_captain").missing).toBe(false);
  });
});

describe("agentFromOrphanInstance", () => {
  it("Should mark the stand-in as an orphan, so its page stays read-only", () => {
    expect(isOrphanAgent(agentFromOrphanInstance(agentInstanceFixture()))).toBe(true);
    expect(isOrphanAgent({ definition_digest: "sha256:abc" })).toBe(false);
  });

  it("Should give an orphan enough of an agent to explain itself", () => {
    const agent = agentFromOrphanInstance(agentInstanceFixture({ label: "Triage Lead" }));
    expect(agent).toMatchObject({ name: "triage", label: "Triage Lead", kind: "stays_on" });
  });
});
