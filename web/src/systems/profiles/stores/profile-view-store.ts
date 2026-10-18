import { createStore } from "@xstate/store";
import { isHydrated, persist, rehydrateStore } from "@xstate/store/persist";

import type { ProfileView } from "../types";

const PROFILE_VIEW_PERSIST_KEY = "compozy:profile-view";

interface ProfileViewContext {
  view: ProfileView | null;
}

/**
 * The one active view this client is looking through, shared by every
 * workspace and by the Global breadth (issue #792). Switching projects never
 * moves it; only an explicit operator gesture does.
 *
 * A concrete profile stays daemon state: the Global remembered choice is what a
 * reload resolves, and this store only records that the client has since looked
 * elsewhere, so an external switch refreshes the remembered choice without
 * moving an open client. All profiles is the one view the daemon cannot store,
 * so it is the only answer persisted here — it survives a reload, and the
 * remembered choice underneath it stays the creation destination.
 */
export const profileViewStore = createStore({
  context: { view: null } as ProfileViewContext,
  on: {
    viewEntered: (context, event: { view: ProfileView }) => {
      if (context.view !== null) return undefined;
      return { ...context, view: event.view };
    },
    viewChanged: (context, event: { view: ProfileView }) => {
      if (context.view && sameView(context.view, event.view)) return undefined;
      return { ...context, view: event.view };
    },
    viewCleared: context => (context.view === null ? undefined : { ...context, view: null }),
    /** Drops a deleted or archived profile so the next read resolves a live one. */
    profileSwept: (context, event: { profile: string }) => {
      const view = context.view;
      if (view?.kind !== "profile" || view.profile !== event.profile) return undefined;
      return { ...context, view: null };
    },
  },
}).with(
  persist({
    name: PROFILE_VIEW_PERSIST_KEY,
    version: 1,
    pick: context => ({ view: context.view?.kind === "aggregate" ? context.view : null }),
  })
);

function sameView(left: ProfileView, right: ProfileView): boolean {
  if (left.kind === "aggregate") return right.kind === "aggregate";
  return right.kind === "profile" && left.profile === right.profile;
}

/** The view this client has chosen, or null to defer to the remembered choice. */
export function localProfileView(): ProfileView | null {
  return profileViewStore.getSnapshot().context.view;
}

export function setProfileView(view: ProfileView): void {
  profileViewStore.trigger.viewChanged({ view });
}

/** Captures the remembered choice the first time this client resolves one. */
export function enterProfileView(view: ProfileView): void {
  profileViewStore.trigger.viewEntered({ view });
}

/** Rolls back after a failed persist, restoring the previous local answer. */
export function restoreProfileView(previous: ProfileView | null): void {
  if (previous === null) {
    profileViewStore.trigger.viewCleared();
    return;
  }
  profileViewStore.trigger.viewChanged({ view: previous });
}

export function sweepProfileView(profile: string): void {
  profileViewStore.trigger.profileSwept({ profile });
}

export function resetProfileViews(): void {
  profileViewStore.trigger.viewCleared();
}

export function isProfileViewStoreHydrated(): boolean {
  return isHydrated(profileViewStore);
}

export function rehydrateProfileViewStore(): Promise<void> {
  return rehydrateStore(profileViewStore);
}
