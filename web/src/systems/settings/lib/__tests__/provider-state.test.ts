import { describe, expect, it } from "vitest";

import { settingsProviderFixtures } from "../../mocks/fixtures";
import { deriveProviderStateLabel } from "../provider-state";

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
});
