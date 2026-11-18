import { describe, expect, it } from "vitest";

import { settingsProviderFixtures } from "../../mocks/fixtures";
import { deriveProviderStateLabel, getProviderStateView } from "../provider-state";

const base = settingsProviderFixtures[0]!;

describe("deriveProviderStateLabel", () => {
  it.each([
    ["authenticated", "installed"],
    ["none", "installed"],
    ["needs_login", "needs-sign-in"],
    ["unknown", "auth-unknown"],
    ["permission_denied", "auth-unavailable"],
    ["rate_limited", "auth-unavailable"],
    ["transient", "auth-unavailable"],
    ["missing_cli", "binary-missing"],
  ] as const)("maps %s auth to %s", (authState, expected) => {
    expect(
      deriveProviderStateLabel({
        ...base,
        auth_status: { ...base.auth_status!, state: authState },
      })
    ).toBe(expected);
  });

  it("keeps unresolved required credentials unconfigured", () => {
    expect(deriveProviderStateLabel(settingsProviderFixtures[2]!)).toBe("unconfigured");
  });

  // UT-020: turned off outranks install and sign-in facts, and a missing field reads as on.
  it("reads a disabled provider as disabled ahead of binary-missing and needs-sign-in (UT-020)", () => {
    const off = { ...base, settings: { ...base.settings, enabled: false } };
    for (const state of ["missing_cli", "needs_login"] as const) {
      const entry = { ...off, auth_status: { ...base.auth_status!, state } };
      expect(deriveProviderStateLabel(entry)).toBe("disabled");
      expect(deriveProviderStateLabel({ ...entry, command_available: false })).toBe("disabled");
    }
    expect(getProviderStateView(off)).toMatchObject({ tone: "neutral", display: "Disabled" });
    const { enabled: _enabled, ...legacy } = base.settings;
    expect(deriveProviderStateLabel({ ...base, settings: legacy as typeof base.settings })).toBe(
      deriveProviderStateLabel(base)
    );
    expect(deriveProviderStateLabel(base)).not.toBe("disabled");
  });

  it("keeps a present-but-unverified protocol login out of ready", () => {
    const agy = settingsProviderFixtures.find(entry => entry.name === "agy")!;
    expect(agy.auth_status?.login?.presence).toBe("present");
    expect(deriveProviderStateLabel(agy)).toBe("auth-unknown");
  });
});

describe("getProviderStateView", () => {
  const unverified = {
    ...base,
    command_available: true,
    credentials: [],
    auth_status: { ...base.auth_status!, mode: "native_cli", state: "unknown" },
  };

  it.each([
    ["a status command", { auth_status_command: "claude auth status" }, "check"],
    [
      "an ACP sign-in method",
      { auth_status_command: "", auth_acp_method: "oauth-personal" },
      "check",
    ],
    ["no way to check", { auth_status_command: "", auth_acp_method: "" }, "edit"],
  ] as const)("runs Check sign-in in place only with %s", (_label, settings, intent) => {
    const view = getProviderStateView({
      ...unverified,
      settings: { ...base.settings, ...settings },
    });
    expect(view.cta).toEqual({ label: "Check sign-in", intent });
  });
});
