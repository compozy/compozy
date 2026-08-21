// Suite: profile identity normalization
// Invariant: starter and stored symbols always resolve to one valid, mutually exclusive identity.
// Owning layer: web/src/systems/profiles/lib/profile-identity.ts.
import { describe, expect, it } from "vitest";

import { starterIdentity, symbolOf, symbolPatch } from "../profile-identity";

describe("profile identity normalization", () => {
  it("Should rotate starter identities deterministically and clamp negative counts", () => {
    expect(starterIdentity(-1)).toEqual(starterIdentity(0));
    expect(starterIdentity(1)).not.toEqual(starterIdentity(0));
    expect(starterIdentity(8)).toEqual(starterIdentity(0));
  });

  it("Should prefer a trimmed emoji and fall back to a normalized icon", () => {
    expect(symbolOf({ emoji: " 🚀 ", icon: "rocket" })).toEqual({
      kind: "emoji",
      value: "🚀",
    });
    expect(symbolOf({ emoji: " ", icon: " megaphone " })).toEqual({
      kind: "icon",
      value: "megaphone",
    });
    expect(symbolOf({ emoji: null, icon: null })).toEqual({
      kind: "icon",
      value: "user-round",
    });
  });

  it("Should serialize exactly one symbol field", () => {
    expect(symbolPatch({ kind: "emoji", value: "🎯" })).toEqual({ emoji: "🎯" });
    expect(symbolPatch({ kind: "icon", value: "target" })).toEqual({ icon: "target" });
  });
});
