// Suite: profile active-view store transitions
// Invariant: the active view is one global answer shared by every workspace, rollback-exact —
// a failed persist restores the previous answer including "there wasn't one" — a profile that
// stops existing is dropped, and only the All profiles view is persisted for a reload.
// Boundary IN: the module-scoped @xstate/store, its imperative API, and its localStorage envelope.
// Boundary OUT: the remembered choice (server state, exercised through the selection hooks).

import { flushStorage } from "@xstate/store/persist";
import { beforeEach, describe, expect, it } from "vitest";

import {
  enterProfileView,
  localProfileView,
  profileViewStore,
  resetProfileViews,
  restoreProfileView,
  setProfileView,
  sweepProfileView,
} from "../profile-view-store";

const PERSIST_KEY = "compozy:profile-view";

function persistedView(): unknown {
  flushStorage(profileViewStore);
  const raw = window.localStorage.getItem(PERSIST_KEY);
  return raw === null ? null : (JSON.parse(raw) as { context: { view: unknown } }).context.view;
}

describe("profile view store", () => {
  beforeEach(() => {
    resetProfileViews();
  });

  it("Should start with no local answer so the remembered choice wins", () => {
    expect(localProfileView()).toBeNull();
  });

  it("Should capture the remembered choice once when a client first resolves one", () => {
    enterProfileView({ kind: "profile", profile: "default" });
    enterProfileView({ kind: "profile", profile: "marketing" });
    expect(localProfileView()).toEqual({ kind: "profile", profile: "default" });
  });

  it("Should roll back to the previous view after a failed persist", () => {
    setProfileView({ kind: "profile", profile: "marketing" });
    const previous = localProfileView();
    setProfileView({ kind: "profile", profile: "consulting" });
    restoreProfileView(previous);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "marketing" });
  });

  it("Should roll back to having no local answer at all", () => {
    // The first switch has no previous view; rolling back has to mean "defer to
    // the server again", not "stay where the failed switch put us".
    const previous = localProfileView();
    setProfileView({ kind: "profile", profile: "marketing" });
    restoreProfileView(previous);
    expect(localProfileView()).toBeNull();
  });

  it("Should drop a swept profile but leave any other view", () => {
    setProfileView({ kind: "profile", profile: "marketing" });
    sweepProfileView("consulting");
    expect(localProfileView()).toEqual({ kind: "profile", profile: "marketing" });
    sweepProfileView("marketing");
    expect(localProfileView()).toBeNull();
    setProfileView({ kind: "aggregate" });
    sweepProfileView("marketing");
    expect(localProfileView()).toEqual({ kind: "aggregate" });
  });

  it("Should not churn state when the same view is set twice", () => {
    setProfileView({ kind: "profile", profile: "marketing" });
    const snapshot = profileViewStore.getSnapshot().context;
    setProfileView({ kind: "profile", profile: "marketing" });
    expect(profileViewStore.getSnapshot().context).toBe(snapshot);
  });

  it("Should persist only the All profiles view for a reload", () => {
    setProfileView({ kind: "aggregate" });
    expect(persistedView()).toEqual({ kind: "aggregate" });
    // A concrete choice is daemon state; a reload resolves the remembered Global slot.
    setProfileView({ kind: "profile", profile: "marketing" });
    expect(persistedView()).toBeNull();
  });
});
