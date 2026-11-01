import { describe, expect, it } from "vitest";

import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import { humanizeAgentSlug, resolveAgentIdentity } from "../agent-identity";
import { agentFromOrphanInstance, normalizeAgentInstance } from "../agent-instance-wire";
import {
  GLOBAL_SCOPE,
  agentAvatarAppearance,
  agentInstanceAvatarState,
  agentInstanceLimitReached,
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

describe("agentInstanceLimitReached", () => {
  it("Should count active and starting, never paused", () => {
    const active = Array.from({ length: 9 }, () => agentInstanceFixture());
    expect(agentInstanceLimitReached(active)).toBe(false);
    expect(agentInstanceLimitReached([...active, agentInstanceFixture({ state: "paused" })])).toBe(
      false
    );
    expect(
      agentInstanceLimitReached([...active, agentInstanceFixture({ state: "starting" })])
    ).toBe(true);
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

describe("normalizeAgentInstance", () => {
  it("Should read what the daemon leaves out as unknown, never as a state", () => {
    const instance = normalizeAgentInstance({
      id: "ainst-1",
      agent_name: "release-captain",
      state: "active",
      workspace_id: "",
    });
    expect(instance).toMatchObject({
      label: "Release Captain",
      scope: GLOBAL_SCOPE,
      activity: "unknown",
      needs_attention: false,
    });
    expect(agentInstanceAvatarState(instance)).toBe("unknown");
  });

  it("Should give an orphan enough of an agent to explain itself", () => {
    const agent = agentFromOrphanInstance(agentInstanceFixture({ label: "Triage Lead" }));
    expect(agent).toMatchObject({ name: "triage", label: "Triage Lead", kind: "stays_on" });
  });
});
