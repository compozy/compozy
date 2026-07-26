import { describe, expect, it } from "vitest";

import { settingsProviderFixtures } from "../../mocks/fixtures";
import { providerAuthSummary } from "../provider-copy";

const envProvider = settingsProviderFixtures[2]!;

describe("providerAuthSummary", () => {
  it("names an environment-backed bound secret without claiming Vault storage", () => {
    expect(providerAuthSummary(envProvider)).toBe("Uses OPENROUTER_API_KEY from the environment");
  });

  it("names Vault only for a vault secret reference", () => {
    expect(
      providerAuthSummary({
        ...envProvider,
        credentials: envProvider.credentials?.map(credential => ({
          ...credential,
          secret_ref: "vault:openrouter/api-key",
        })),
      })
    ).toBe("Key stored in Vault");
  });

  it("surfaces unknown native login state", () => {
    expect(providerAuthSummary(settingsProviderFixtures[0]!)).toBe("Local sign-in not verified");
  });
});
