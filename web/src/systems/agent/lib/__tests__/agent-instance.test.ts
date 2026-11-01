import { describe, expect, it } from "vitest";

import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { humanizeAgentSlug, resolveAgentIdentity } from "../agent-identity";
import { agentFromOrphanInstance, isOrphanAgent } from "../agent-instance-wire";
import {
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceCapacity,
  agentPauseCopy,
  agentInstancesByUrgency,
} from "../agent-instance";

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
