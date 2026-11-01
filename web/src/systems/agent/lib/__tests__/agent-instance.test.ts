import { describe, expect, it } from "vitest";

import { agentInstanceFixture } from "../../mocks/instance-fixtures";
import {
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
