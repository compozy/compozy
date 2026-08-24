// Suite: the profile read-scope seam.
// Invariant: the aggregate/scoped decision becomes exactly one wire scope and one
// cache-key segment, and the aggregate never borrows `default`'s identity.
// Boundary IN: params, key segment, owner projection.
// Boundary OUT: how a view is chosen, and what the daemon does with the scope.
import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it } from "vitest";

import { PROFILE_AGGREGATE, type ProfileView } from "../../types";
import { profileKeys } from "../query-keys";
import { readProfileScopeParams, readProfileView } from "../profile-scope-resolver";
import {
  isAggregateView,
  ownerFromRow,
  profileScopeParams,
  profileViewKey,
} from "../profile-scope";
import { resetProfileViews, setProfileView } from "../../stores/profile-view-store";

const SCOPED: ProfileView = { kind: "profile", profile: "marketing" };
const AGGREGATE: ProfileView = { kind: "aggregate" };

beforeEach(() => resetProfileViews());

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

describe("profile scope resolver precedence", () => {
  const lens = { scope: "workspace", workspaceId: "workspace:alpha" } as const;

  it("Should prefer a local view over the remembered selection", () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(profileKeys.selection(lens), {
      scope: "workspace",
      workspace_id: "workspace:alpha",
      profile: "marketing",
    });
    setProfileView(lens, AGGREGATE);

    expect(readProfileView(queryClient, lens)).toEqual(AGGREGATE);
    expect(readProfileScopeParams(queryClient, lens)).toEqual({ all_profiles: true });
  });

  it("Should use the remembered selection before falling back to default", () => {
    const queryClient = new QueryClient();
    expect(readProfileView(queryClient, lens)).toEqual({
      kind: "profile",
      profile: "default",
    });

    queryClient.setQueryData(profileKeys.selection(lens), {
      scope: "workspace",
      workspace_id: "workspace:alpha",
      profile: "marketing",
    });
    expect(readProfileView(queryClient, lens)).toEqual({
      kind: "profile",
      profile: "marketing",
    });
  });
});
