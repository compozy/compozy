import { describe, expect, it } from "vitest";

import {
  buildCheckDescriptors,
  defaultCheckStates,
  initialCheckStates,
  parseEnabledChecks,
  serializeEnabledChecks,
} from "../loop-config-checks";
import type { LoopContract } from "../../types";

const deliveryContract: LoopContract = {
  goal: "Ship it.",
  definition_of_done: "Configured project checks are green.",
  iteration_cap: 50,
  budget: { tokens: 500_000, wall_clock_sec: 3_600, on_exceeded: "halt" },
  no_progress: { window: 3, hash_fields: [] },
  verification: [
    { id: "verify_build", type: "command", check: "npm test", expect: "exit 0" },
    { id: "acceptance_review", type: "agent-judge", agent: "reviewer", rubric: "Meets the goal." },
  ],
};

const watchContract: LoopContract = {
  goal: "Drain reviews.",
  definition_of_done: "All handled.",
  iteration_cap: 0,
  budget: { tokens: 0, wall_clock_sec: 0, on_exceeded: "halt" },
  no_progress: { window: 5, hash_fields: [] },
};

describe("buildCheckDescriptors", () => {
  it("Should mark command checks toggleable and every other type locked", () => {
    const [command, judge] = buildCheckDescriptors(deliveryContract);
    expect(command).toMatchObject({
      id: "verify_build",
      isCommand: true,
      locked: false,
      declaredCommand: "npm test",
    });
    expect(command.method).toContain("npm test");
    expect(judge).toMatchObject({ id: "acceptance_review", isCommand: false, locked: true });
    expect(judge.declaredCommand).toBe("");
  });

  it("Should return an empty list when the loop declares no verification checks", () => {
    expect(buildCheckDescriptors(watchContract)).toEqual([]);
  });
});

describe("parseEnabledChecks", () => {
  it("Should tolerate null, arrays, and non-object entries", () => {
    expect(parseEnabledChecks(null)).toEqual({});
    expect(parseEnabledChecks([1, 2])).toEqual({});
    expect(parseEnabledChecks({ a: 5, b: { enabled: true } })).toEqual({
      b: { enabled: true, command: undefined },
    });
  });

  it("Should read enabled + command overrides", () => {
    expect(
      parseEnabledChecks({ verify_build: { enabled: false, command: "go test ./..." } })
    ).toEqual({ verify_build: { enabled: false, command: "go test ./..." } });
  });
});

describe("initialCheckStates", () => {
  const descriptors = buildCheckDescriptors(deliveryContract);

  it("Should default command checks to enabled with the declared command", () => {
    const states = initialCheckStates(descriptors, null);
    expect(states.verify_build).toEqual({ enabled: true, command: "npm test" });
    expect(states.acceptance_review).toEqual({ enabled: true, command: "" });
  });

  it("Should apply stored overrides for command checks and keep locked checks on", () => {
    const states = initialCheckStates(descriptors, {
      verify_build: { enabled: false, command: "go test ./..." },
      acceptance_review: { enabled: false },
    });
    expect(states.verify_build).toEqual({ enabled: false, command: "go test ./..." });
    // A locked check ignores any stored disable — it can only change via a fork.
    expect(states.acceptance_review).toEqual({ enabled: true, command: "" });
  });
});

describe("defaultCheckStates", () => {
  it("Should enable every check and restore each command to its declared value", () => {
    const states = defaultCheckStates(buildCheckDescriptors(deliveryContract));
    expect(states.verify_build).toEqual({ enabled: true, command: "npm test" });
    expect(states.acceptance_review).toEqual({ enabled: true, command: "" });
  });
});

describe("serializeEnabledChecks", () => {
  const descriptors = buildCheckDescriptors(deliveryContract);

  it("Should serialize only command checks and trim a diverging command", () => {
    const states = {
      verify_build: { enabled: false, command: "  go test ./...  " },
      acceptance_review: { enabled: true, command: "" },
    };
    expect(serializeEnabledChecks(descriptors, states)).toEqual({
      verify_build: { enabled: false, command: "go test ./..." },
    });
  });

  it("Should omit a command check left enabled with its declared command (inherit)", () => {
    const states = {
      verify_build: { enabled: true, command: "npm test" },
      acceptance_review: { enabled: true, command: "" },
    };
    expect(serializeEnabledChecks(descriptors, states)).toBeNull();
  });

  it("Should emit only the enabled flag when a disabled check keeps its declared command", () => {
    const states = {
      verify_build: { enabled: false, command: "npm test" },
      acceptance_review: { enabled: true, command: "" },
    };
    expect(serializeEnabledChecks(descriptors, states)).toEqual({
      verify_build: { enabled: false },
    });
  });

  it("Should inherit (not pin an empty command) when an enabled check's command is cleared", () => {
    const states = {
      verify_build: { enabled: true, command: "   " },
      acceptance_review: { enabled: true, command: "" },
    };
    expect(serializeEnabledChecks(descriptors, states)).toBeNull();
  });

  it("Should return null when the loop declares no command checks", () => {
    const watchDescriptors = buildCheckDescriptors(watchContract);
    expect(serializeEnabledChecks(watchDescriptors, {})).toBeNull();
  });
});
