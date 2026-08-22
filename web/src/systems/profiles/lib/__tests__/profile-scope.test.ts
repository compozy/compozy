// Suite: the profile read-scope seam.
// Invariant: the aggregate/scoped decision becomes exactly one wire scope and one
// cache-key segment, and the aggregate never borrows `default`'s identity.
// Boundary IN: params, key segment, owner projection.
// Boundary OUT: how a view is chosen, and what the daemon does with the scope.
import { describe, expect, it } from "vitest";

import { PROFILE_AGGREGATE, type ProfileView } from "../../types";
import {
  isAggregateView,
  ownerFromRow,
  profileScopeParams,
  profileViewKey,
} from "../profile-scope";

const SCOPED: ProfileView = { kind: "profile", profile: "marketing" };
const AGGREGATE: ProfileView = { kind: "aggregate" };

describe("profileScopeParams", () => {
  it("Should send one profile by name when scoped", () => {
    expect(profileScopeParams(SCOPED)).toEqual({ profile: "marketing" });
  });

  it("Should widen only through the explicit aggregate flag", () => {
    expect(profileScopeParams(AGGREGATE)).toEqual({ all_profiles: true });
  });

  it("Should never produce a scope that omits both modes", () => {
    for (const view of [SCOPED, AGGREGATE]) {
      const params = profileScopeParams(view);
      expect("profile" in params || "all_profiles" in params).toBe(true);
    }
  });
});

describe("profileViewKey", () => {
  it("Should name the profile for a scoped view", () => {
    expect(profileViewKey(SCOPED)).toBe("marketing");
  });

  it("Should use the reserved aggregate identity, not the default profile's name", () => {
    expect(profileViewKey(AGGREGATE)).toBe(PROFILE_AGGREGATE);
    expect(profileViewKey(AGGREGATE)).not.toBe("default");
  });
});

describe("ownerFromRow", () => {
  it("Should read a labeled row's owner without knowing its surface", () => {
    expect(
      ownerFromRow({
        profile_id: "01J9MARKETING00000000000000",
        profile_name: "marketing",
        profile_color: "#c26ad6",
        profile_icon: "megaphone",
      })
    ).toEqual({
      id: "01J9MARKETING00000000000000",
      name: "marketing",
      color: "#c26ad6",
      icon: "megaphone",
      emoji: null,
      archived: false,
    });
  });

  it("Should treat an absent archived flag as not archived rather than unknown", () => {
    expect(ownerFromRow({ profile_id: "id", profile_name: "default" }).archived).toBe(false);
    expect(
      ownerFromRow({ profile_id: "id", profile_name: "old agency", profile_archived: true })
        .archived
    ).toBe(true);
  });
});

describe("isAggregateView", () => {
  it("Should distinguish the aggregate from every real profile", () => {
    expect(isAggregateView(AGGREGATE)).toBe(true);
    expect(isAggregateView(SCOPED)).toBe(false);
    expect(isAggregateView({ kind: "profile", profile: "default" })).toBe(false);
  });
});
