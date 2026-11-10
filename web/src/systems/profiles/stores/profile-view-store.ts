import { createStore } from "@xstate/store";
import { isHydrated, persist, rehydrateStore } from "@xstate/store/persist";

import type { ProfileView } from "../types";

const PROFILE_VIEW_PERSIST_KEY = "compozy:profile-view";

interface ProfileViewContext {
  view: ProfileView | null;
  /**
   * Counts the explicit switches this client has made. Entering the remembered choice at boot
   * and sweeping a removed profile are not switches, so surfaces that must react to an operator
   * moving between profiles (the desk dropping another profile's windows) key off this alone.
   */
  switchEpoch: number;
  /** The switch (its epoch) that produced the current view; 0 when no switch did. */
  viewSwitch: number;
  /**
   * The current view is what a switch with an unknown outcome left behind, not something the
   * operator chose: the daemon may have applied the write anyway.
   */
  viewRolledBack: boolean;
  /**
   * Switches whose persist has not settled, by epoch, with the view each one replaced. Switches
   * can overlap, so a failure rolls back along this chain rather than to a captured snapshot: a
   * failed switch's successors inherit what it replaced, and the view only returns when the
   * failed switch is the one still showing.
   */
  pending: Readonly<Record<number, PendingSwitch>>;
}

interface PendingSwitch {
  previous: ProfileView | null;
  previousSwitch: number;
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
 *
 * A rollback restores the screen. It restores the choice a reload starts from only when the daemon
 * refused the write: a reload rejects the write it interrupts, even a keepalive write the daemon
 * goes on to apply, so a failure without an answer cannot prove the operator stayed in All
 * profiles, and the next document defers to the daemon until the operator chooses again (#1436).
 */
export const profileViewStore = createStore({
  context: {
    view: null,
    switchEpoch: 0,
    viewSwitch: 0,
    viewRolledBack: false,
    pending: {},
  } as ProfileViewContext,
  on: {
    viewEntered: (context, event: { view: ProfileView }) => {
      if (context.view !== null) return undefined;
      return { ...context, view: event.view };
    },
    viewChanged: (context, event: { view: ProfileView }) => {
      if (context.view && sameView(context.view, event.view)) {
        return context.viewRolledBack ? { ...context, viewRolledBack: false } : undefined;
      }
      const epoch = context.switchEpoch + 1;
      return {
        ...context,
        view: event.view,
        switchEpoch: epoch,
        viewSwitch: epoch,
        viewRolledBack: false,
        pending: {
          ...context.pending,
          [epoch]: { previous: context.view, previousSwitch: context.viewSwitch },
        },
      };
    },
    /** The switch's persist landed: nothing will roll it back any more. */
    switchSettled: (context, event: { switch: number }) => {
      if (!(event.switch in context.pending)) return undefined;
      const { [event.switch]: _settled, ...pending } = context.pending;
      return { ...context, pending };
    },
    /**
     * The switch's persist failed: it never happened. Its successors now replace what it replaced,
     * and if it is still the view on screen, that view returns. A rollback is not a switch.
     */
    switchFailed: (context, event: { switch: number; refused: boolean }) => {
      const failed = context.pending[event.switch];
      if (failed === undefined) return undefined;
      const pending: Record<number, PendingSwitch> = {};
      for (const [id, entry] of Object.entries(context.pending)) {
        if (Number(id) === event.switch) continue;
        pending[Number(id)] = entry.previousSwitch === event.switch ? { ...failed } : entry;
      }
      if (context.viewSwitch !== event.switch) return { ...context, pending };
      return {
        ...context,
        view: failed.previous,
        viewSwitch: failed.previousSwitch,
        viewRolledBack: !event.refused,
        pending,
      };
    },
    viewCleared: context =>
      context.view === null && !context.viewRolledBack
        ? undefined
        : { ...context, view: null, viewSwitch: 0, viewRolledBack: false },
    /** Drops a deleted or archived profile so the next read resolves a live one. */
    profileSwept: (context, event: { profile: string }) => {
      const view = context.view;
      if (view?.kind !== "profile" || view.profile !== event.profile) return undefined;
      return { ...context, view: null, viewSwitch: 0, viewRolledBack: false };
    },
  },
}).with(
  persist({
    name: PROFILE_VIEW_PERSIST_KEY,
    version: 1,
    pick: context => ({
      view: context.view?.kind === "aggregate" && !context.viewRolledBack ? context.view : null,
    }),
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

/** Switches the view; returns the switch's id, or null when the view was already this one. */
export function setProfileView(view: ProfileView): number | null {
  const before = profileViewStore.getSnapshot().context.switchEpoch;
  profileViewStore.trigger.viewChanged({ view });
  const after = profileViewStore.getSnapshot().context.switchEpoch;
  return after === before ? null : after;
}

/** The switch's persist landed. */
export function settleProfileSwitch(switchId: number): void {
  profileViewStore.trigger.switchSettled({ switch: switchId });
}

/**
 * The switch's persist failed: roll it back along the pending chain. `refused` means the daemon
 * answered and did not apply the write; otherwise the outcome is unknown.
 */
export function failProfileSwitch(switchId: number, { refused = false } = {}): void {
  profileViewStore.trigger.switchFailed({ switch: switchId, refused });
}

/** Captures the remembered choice the first time this client resolves one. */
export function enterProfileView(view: ProfileView): void {
  profileViewStore.trigger.viewEntered({ view });
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
