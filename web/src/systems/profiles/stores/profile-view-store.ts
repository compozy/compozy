import { createStore } from "@xstate/store";

import { profileLensKey } from "../lib/query-keys";
import type { ProfileLens, ProfileView } from "../types";

/**
 * The active view, per lens — what this client is looking through right now.
 *
 * Deliberately ephemeral and deliberately NOT persisted (ADR-014). The
 * remembered choice is daemon state read through the selection routes; this
 * store only records that *this* client has since looked somewhere else, so
 * entering a lens for the first time falls through to the server's answer.
 *
 * Nothing outside an explicit operator gesture writes here. In particular the
 * profile event stream cannot: an external switch refreshes the remembered
 * choice without moving an open client (US-010.EC-4).
 */
export const profileViewStore = createStore({
  context: {
    viewByLens: {} as Record<string, ProfileView>,
  },
  on: {
    viewChanged: (context, event: { lens: string; view: ProfileView }) => {
      const current = context.viewByLens[event.lens];
      if (current && sameView(current, event.view)) return undefined;
      return { ...context, viewByLens: { ...context.viewByLens, [event.lens]: event.view } };
    },
    viewCleared: (context, event: { lens: string }) => {
      if (!(event.lens in context.viewByLens)) return undefined;
      const next = { ...context.viewByLens };
      delete next[event.lens];
      return { ...context, viewByLens: next };
    },
    /** Drops a deleted or archived profile from every lens it was left in. */
    profileSwept: (context, event: { profile: string }) => {
      const entries = Object.entries(context.viewByLens).filter(
        ([, view]) => view.kind !== "profile" || view.profile !== event.profile
      );
      if (entries.length === Object.keys(context.viewByLens).length) return undefined;
      return { ...context, viewByLens: Object.fromEntries(entries) };
    },
    reset: context =>
      Object.keys(context.viewByLens).length === 0 ? undefined : { ...context, viewByLens: {} },
  },
});

function sameView(left: ProfileView, right: ProfileView): boolean {
  if (left.kind === "aggregate") return right.kind === "aggregate";
  return right.kind === "profile" && left.profile === right.profile;
}

export const profileViewSelectors = {
  viewByLens: profileViewStore.select(context => context.viewByLens),
};

/** The view this client has chosen for a lens, or null to defer to the server. */
export function localProfileView(lens: ProfileLens): ProfileView | null {
  return profileViewStore.getSnapshot().context.viewByLens[profileLensKey(lens)] ?? null;
}

export function setProfileView(lens: ProfileLens, view: ProfileView): void {
  profileViewStore.trigger.viewChanged({ lens: profileLensKey(lens), view });
}

/** Rolls a lens back after a failed persist, restoring the previous local answer. */
export function restoreProfileView(lens: ProfileLens, previous: ProfileView | null): void {
  if (previous === null) {
    profileViewStore.trigger.viewCleared({ lens: profileLensKey(lens) });
    return;
  }
  profileViewStore.trigger.viewChanged({ lens: profileLensKey(lens), view: previous });
}

export function sweepProfileView(profile: string): void {
  profileViewStore.trigger.profileSwept({ profile });
}

export function resetProfileViews(): void {
  profileViewStore.trigger.reset();
}
