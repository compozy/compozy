// Suite: profile active-view store transitions
// Invariant: the active view is per-lens, ephemeral, and rollback-exact — a failed persist
// restores the previous answer including "there wasn't one", and a profile that stops
// existing is dropped from every lens it was left in.
// Boundary IN: the module-scoped @xstate/store and its imperative API.
// Boundary OUT: the remembered choice (server state, exercised through the selection hooks).

import { beforeEach, describe, expect, it } from "vitest";

import {
  carryProfileView,
  enterProfileView,
  localProfileView,
  profileViewStore,
  resetProfileViews,
  restoreProfileView,
  setProfileView,
  sweepProfileView,
} from "../profile-view-store";

const ACME = { scope: "workspace", workspaceId: "ws-acme" } as const;
const CLIENT = { scope: "workspace", workspaceId: "ws-client" } as const;
const GLOBAL = { scope: "global" } as const;

describe("profile view store", () => {
  beforeEach(() => {
    resetProfileViews();
  });

  it("Should start with no local answer so the remembered choice wins", () => {
    expect(localProfileView(ACME)).toBeNull();
    expect(localProfileView(GLOBAL)).toBeNull();
  });

  it("Should keep each lens independent, including the Global slot", () => {
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    setProfileView(GLOBAL, { kind: "aggregate" });
    expect(localProfileView(ACME)).toEqual({ kind: "profile", profile: "marketing" });
    expect(localProfileView(GLOBAL)).toEqual({ kind: "aggregate" });
    expect(localProfileView(CLIENT)).toBeNull();
  });

  it("Should capture the remembered choice once when a client enters a lens", () => {
    enterProfileView(ACME, { kind: "profile", profile: "default" });
    enterProfileView(ACME, { kind: "profile", profile: "marketing" });
    expect(localProfileView(ACME)).toEqual({ kind: "profile", profile: "default" });
  });

  it("Should carry the acting view when workspace breadth changes", () => {
    setProfileView(ACME, { kind: "aggregate" });
    setProfileView(GLOBAL, { kind: "profile", profile: "consulting" });

    carryProfileView(ACME, GLOBAL);

    expect(localProfileView(ACME)).toEqual({ kind: "aggregate" });
    expect(localProfileView(GLOBAL)).toEqual({ kind: "aggregate" });
  });

  it("Should roll back to the previous view after a failed persist", () => {
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    const previous = localProfileView(ACME);
    setProfileView(ACME, { kind: "profile", profile: "consulting" });
    restoreProfileView(ACME, previous);
    expect(localProfileView(ACME)).toEqual({ kind: "profile", profile: "marketing" });
  });

  it("Should roll back to having no local answer at all", () => {
    // The first switch in a lens has no previous view; rolling back has to mean
    // "defer to the server again", not "stay where the failed switch put us".
    const previous = localProfileView(ACME);
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    restoreProfileView(ACME, previous);
    expect(localProfileView(ACME)).toBeNull();
  });

  it("Should drop a swept profile from every lens but leave the others", () => {
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    setProfileView(CLIENT, { kind: "profile", profile: "marketing" });
    setProfileView(GLOBAL, { kind: "profile", profile: "consulting" });
    sweepProfileView("marketing");
    expect(localProfileView(ACME)).toBeNull();
    expect(localProfileView(CLIENT)).toBeNull();
    expect(localProfileView(GLOBAL)).toEqual({ kind: "profile", profile: "consulting" });
  });

  it("Should not churn state when the same view is set twice", () => {
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    const snapshot = profileViewStore.getSnapshot().context;
    setProfileView(ACME, { kind: "profile", profile: "marketing" });
    expect(profileViewStore.getSnapshot().context).toBe(snapshot);
  });
});
