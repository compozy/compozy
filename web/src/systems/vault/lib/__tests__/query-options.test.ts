// Suite: vault list query options
// Invariant: typing a prefix keeps the previous rows on screen, but another namespace never shows
//   the previous namespace's secrets.
// Boundary IN: the list options' placeholder rule over the canonical list keys.
// Boundary OUT: the page hook and list rendering, owned by their suites.
import { describe, expect, it } from "vitest";

import type { VaultListFilter, VaultSecret } from "../../types";
import { vaultSecretsListOptions } from "../query-options";

const rows = [{ ref: "vault:providers/codex/api_key" }] as VaultSecret[];

/** What the list shows for `next` while `previous` is the query that was on screen. */
function placeholderFor(previous: VaultListFilter, next: VaultListFilter) {
  const placeholder = vaultSecretsListOptions(next).placeholderData;
  if (typeof placeholder !== "function") throw new Error("expected a placeholder rule");
  return placeholder(rows, { queryKey: vaultSecretsListOptions(previous).queryKey } as never);
}

describe("vault list options", () => {
  it("Should keep the previous rows across a prefix edit within one namespace", () => {
    expect(
      placeholderFor(
        { namespace: "providers", prefix: "co" },
        { namespace: "providers", prefix: "cod" }
      )
    ).toBe(rows);
    expect(
      placeholderFor({ namespace: "providers" }, { namespace: "providers", prefix: "co" })
    ).toBe(rows);
    expect(placeholderFor({}, { prefix: "co" })).toBe(rows);
  });

  it("Should show nothing stale when the namespace changes", () => {
    expect(
      placeholderFor({ namespace: "providers", prefix: "co" }, { namespace: "mcp", prefix: "co" })
    ).toBeUndefined();
    expect(placeholderFor({ namespace: "providers" }, {})).toBeUndefined();
  });
});
