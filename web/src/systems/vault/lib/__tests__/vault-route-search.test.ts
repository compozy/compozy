// Suite: Vault route validation
// Invariant: a direct Vault selection survives route normalization as an exact, trimmed ref.
// Boundary IN: TanStack Router search values.
// Boundary OUT: Vault page selection and API transport.
import { describe, expect, it } from "vitest";

import { validateVaultSearch } from "../vault-route-search";

describe("validateVaultSearch", () => {
  it("Should preserve a trimmed direct-selection ref alongside list controls", () => {
    expect(
      validateVaultSearch({
        namespace: "providers",
        q: "vault:providers/",
        ref: "  vault:providers/openai  ",
        view: "cards",
      })
    ).toEqual({
      namespace: "providers",
      q: "vault:providers/",
      ref: "vault:providers/openai",
      view: "cards",
    });
  });

  it("Should drop a blank direct-selection ref", () => {
    expect(validateVaultSearch({ ref: "   " })).toEqual({
      namespace: undefined,
      q: undefined,
      ref: undefined,
      view: undefined,
    });
  });
});
