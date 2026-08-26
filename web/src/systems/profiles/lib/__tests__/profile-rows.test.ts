// Suite: profile row projection
// Invariant: one mapping decides current/archived/needs-setup, the typed disabled reason,
// and whether delete is offered — so the switcher, Settings, and the palette cannot drift
// into three explanations of the same refusal.
// Boundary IN: pure projection over profile payloads.
// Boundary OUT: rendering (component suites) and the daemon's own refusals (Go suites).

import { describe, expect, it } from "vitest";

import {
  canDelete,
  isQuiet,
  PERMANENT_PROFILE,
  toProfileRow,
  toProfileRows,
} from "../profile-rows";
import {
  defaultProfileFixture,
  growthProfileFixture,
  marketingProfileFixture,
  oldAgencyProfileFixture,
  profileFixtures,
  scratchProfileFixture,
} from "../../mocks/fixtures";

describe("toProfileRow", () => {
  it("Should mark the current profile and leave the others active", () => {
    expect(toProfileRow(marketingProfileFixture, "marketing").state).toBe("current");
    expect(toProfileRow(marketingProfileFixture, "marketing").current).toBe(true);
    expect(toProfileRow(defaultProfileFixture, "marketing").state).toBe("active");
  });

  it("Should carry a typed reason for a profile that cannot be switched into", () => {
    expect(toProfileRow(growthProfileFixture, "marketing")).toMatchObject({
      state: "needs-setup",
      needsSetup: true,
      disabledReason: "needs setup",
    });
    expect(toProfileRow(oldAgencyProfileFixture, "marketing")).toMatchObject({
      state: "archived",
      archived: true,
      disabledReason: "archived",
    });
  });

  it("Should leave an available profile with no reason at all", () => {
    expect(toProfileRow(defaultProfileFixture, "marketing").disabledReason).toBe("");
  });

  it("Should carry the persisted identity symbol so every surface renders the chosen glyph", () => {
    expect(toProfileRow(marketingProfileFixture, "marketing")).toMatchObject({
      icon: "megaphone",
      emoji: null,
    });
    expect(
      toProfileRow({ ...defaultProfileFixture, icon: null, emoji: "🦊" }, "default")
    ).toMatchObject({ icon: null, emoji: "🦊" });
  });

  it("Should flag the permanent profile", () => {
    expect(toProfileRow(defaultProfileFixture, "default").permanent).toBe(true);
    expect(toProfileRow(marketingProfileFixture, "default").permanent).toBe(false);
    expect(PERMANENT_PROFILE).toBe("default");
  });
});

describe("canDelete", () => {
  it("Should offer delete only for an archived profile that owns nothing", () => {
    expect(canDelete(scratchProfileFixture)).toBe(true);
  });

  it("Should withhold delete while the profile still holds work", () => {
    expect(canDelete(oldAgencyProfileFixture)).toBe(false);
  });

  it("Should withhold delete from an active profile and from default", () => {
    expect(canDelete(marketingProfileFixture)).toBe(false);
    expect(canDelete({ ...defaultProfileFixture, state: "archived", work_items: 0 })).toBe(false);
  });
});

describe("isQuiet", () => {
  it("Should stay quiet while only default exists", () => {
    expect(isQuiet([defaultProfileFixture])).toBe(true);
  });

  it("Should stay quiet when every other profile is archived", () => {
    expect(isQuiet([defaultProfileFixture, oldAgencyProfileFixture])).toBe(true);
  });

  it("Should become an identity element once a second profile is active", () => {
    expect(isQuiet(profileFixtures)).toBe(false);
  });
});

describe("toProfileRows", () => {
  it("Should preserve order and mark exactly one row current", () => {
    const rows = toProfileRows(profileFixtures, "consulting");
    expect(rows.map(row => row.name)).toEqual(profileFixtures.map(profile => profile.name));
    expect(rows.filter(row => row.current)).toHaveLength(1);
  });
});
