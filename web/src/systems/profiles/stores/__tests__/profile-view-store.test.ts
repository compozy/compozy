// Suite: profile active-view store transitions
// Invariant: the active view is one global answer shared by every workspace, rollback-exact —
// a failed persist restores the previous answer including "there wasn't one", and overlapping
// switches roll back along their chain (a later switch keeps the view; a chain of failures returns
// to where the operator started, #1194) — a profile that
// stops existing is dropped, only the All profiles view is persisted for a reload, and only an
// explicit change of view counts as a switch.
// Boundary IN: the module-scoped @xstate/store, its imperative API, and its localStorage envelope.
// Boundary OUT: the remembered choice (server state, exercised through the selection hooks).

import { flushStorage } from "@xstate/store/persist";
import { beforeEach, describe, expect, it } from "vitest";

import {
  enterProfileView,
  localProfileView,
  profileViewStore,
  rehydrateProfileViewStore,
  failProfileSwitch,
  resetProfileViews,
  setProfileView,
  settleProfileSwitch,
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
    settleProfileSwitch(setProfileView({ kind: "profile", profile: "marketing" }) ?? 0);
    const failed = setProfileView({ kind: "profile", profile: "consulting" }) ?? 0;
    failProfileSwitch(failed);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "marketing" });
  });

  it("Should roll back to having no local answer at all", () => {
    // The first switch has no previous view; rolling back has to mean "defer to
    // the server again", not "stay where the failed switch put us".
    failProfileSwitch(setProfileView({ kind: "profile", profile: "marketing" }) ?? 0);
    expect(localProfileView()).toBeNull();
  });

  it("Should keep a later switch when an earlier overlapping one fails [#1194]", () => {
    const yankee = setProfileView({ kind: "profile", profile: "yankee" }) ?? 0;
    const zulu = setProfileView({ kind: "profile", profile: "zulu" }) ?? 0;
    settleProfileSwitch(zulu);
    failProfileSwitch(yankee);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "zulu" });
  });

  it("Should return to where the operator started when overlapping switches all fail [#1194]", () => {
    settleProfileSwitch(setProfileView({ kind: "profile", profile: "xray" }) ?? 0);
    const yankee = setProfileView({ kind: "profile", profile: "yankee" }) ?? 0;
    const zulu = setProfileView({ kind: "profile", profile: "zulu" }) ?? 0;
    // Either order: the newer failure first restores yankee, the older one then xray; the older
    // failure first hands its fallback to zulu, which then restores xray.
    failProfileSwitch(zulu);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "yankee" });
    failProfileSwitch(yankee);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "xray" });

    const again = setProfileView({ kind: "profile", profile: "yankee" }) ?? 0;
    const last = setProfileView({ kind: "profile", profile: "zulu" }) ?? 0;
    failProfileSwitch(again);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "zulu" });
    failProfileSwitch(last);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "xray" });
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

  it("Should count a switch only for an explicit change of view", () => {
    const epoch = () => profileViewStore.getSnapshot().context.switchEpoch;
    const start = epoch();
    // Entering the remembered choice and sweeping a removed profile are not operator switches.
    enterProfileView({ kind: "profile", profile: "default" });
    sweepProfileView("default");
    expect(epoch()).toBe(start);
    setProfileView({ kind: "aggregate" });
    setProfileView({ kind: "aggregate" });
    setProfileView({ kind: "profile", profile: "marketing" });
    expect(epoch()).toBe(start + 2);
    // A failed switch rolls back to where the operator was; neither half of that moved them.
    const beforeFailedSwitch = epoch();
    failProfileSwitch(setProfileView({ kind: "profile", profile: "consulting" }) ?? 0);
    expect(epoch()).toBe(beforeFailedSwitch + 1);
    expect(localProfileView()).toEqual({ kind: "profile", profile: "marketing" });
  });

  it("Should keep counting explicit switches after a reload restores the view", async () => {
    const epoch = () => profileViewStore.getSnapshot().context.switchEpoch;
    setProfileView({ kind: "aggregate" });
    flushStorage(profileViewStore);
    const persisted = window.localStorage.getItem(PERSIST_KEY);
    expect(persisted).not.toBeNull();
    resetProfileViews();
    window.localStorage.setItem(PERSIST_KEY, persisted ?? "");
    await rehydrateProfileViewStore();
    expect(localProfileView()).toEqual({ kind: "aggregate" });
    const restored = epoch();
    expect(Number.isInteger(restored)).toBe(true);
    setProfileView({ kind: "profile", profile: "marketing" });
    expect(epoch()).toBe(restored + 1);
  });

  it("Should persist only the All profiles view for a reload", () => {
    setProfileView({ kind: "aggregate" });
    expect(persistedView()).toEqual({ kind: "aggregate" });
    // A concrete choice is daemon state; a reload resolves the remembered Global slot.
    setProfileView({ kind: "profile", profile: "marketing" });
    expect(persistedView()).toBeNull();
  });
});
