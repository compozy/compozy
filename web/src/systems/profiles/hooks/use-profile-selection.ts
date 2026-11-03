import { useEffect } from "react";
import { useMutation, useMutationState, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { notifyUser } from "@/lib/user-feedback";

import { putProfileSelection } from "../adapters/profiles-api";
import { PERMANENT_PROFILE, actingProfile, isArchived } from "../lib/profile-rows";
import { profileViewKey } from "../lib/profile-scope";
import { rememberedAfterSwitches, type SelectionSwitch } from "../lib/profile-scope-resolver";
import { profileKeys } from "../lib/query-keys";
import { profileSelectionOptions } from "../lib/query-options";
import {
  enterProfileView,
  failProfileSwitch,
  profileViewStore,
  setProfileView,
  settleProfileSwitch,
} from "../stores/profile-view-store";
import { PROFILE_AGGREGATE, type ProfileSelectionPayload, type ProfileView } from "../types";
import { usePinnedProfileView } from "./use-pinned-profile";
import { useProfiles } from "./use-profiles";

/** The remembered Global choice — daemon state, cached for instant render. */
export function useRememberedProfile(enabled = true) {
  return useQuery(profileSelectionOptions(enabled));
}

/**
 * The remembered choice the aggregate acts as. A switch still being persisted wins over the
 * daemon's answer: the aggregate rereads the choice freely, and a reread landing before the write
 * would otherwise bring the previous profile back, so X → Y → All could act as X (#1194).
 */
export function useRememberedProfileName(enabled = true): string | undefined {
  const remembered = useRememberedProfile(enabled);
  const switches = useMutationState({
    filters: { mutationKey: profileKeys.selectionWrite() },
    select: (mutation): SelectionSwitch => ({
      status: mutation.state.status,
      view: mutation.state.variables as ProfileView | undefined,
    }),
  });
  return rememberedAfterSwitches(switches, remembered.data?.profile);
}

/**
 * What this client is currently acting as, in every workspace at once.
 *
 * Local view first, remembered Global choice second, `default` last — the same
 * ladder the resolver applies, minus the flag and environment rungs that only
 * exist in a terminal. Switching workspaces never moves it (issue #792).
 */
export function useActiveProfileView(enabled = true): ProfileView {
  const remembered = useRememberedProfile(enabled);
  const local = useSelector(profileViewStore, state => state.context.view);
  const rememberedProfile = remembered.data?.profile;
  const remembering = remembered.isFetching;

  useEffect(() => {
    // A cached, invalidated choice is not the settled default for this client.
    if (!enabled || remembering || rememberedProfile === undefined) return;
    enterProfileView({ kind: "profile", profile: rememberedProfile });
  }, [enabled, rememberedProfile, remembering]);

  if (local) return local;
  return { kind: "profile", profile: rememberedProfile ?? PERMANENT_PROFILE };
}

/**
 * The concrete profile of the current view, read from the store alone, so a leaf rendered in every
 * row adds no daemon reads. `useActiveProfileView`, mounted by the shell, keeps the store current.
 */
export function useCurrentProfileName(): string {
  const local = usePinnedProfileView(
    useSelector(profileViewStore, state => state.context.view) ?? { kind: "aggregate" }
  );
  return local.kind === "profile" ? local.profile : PERMANENT_PROFILE;
}

/**
 * Advances on every explicit profile switch this client makes, and on nothing else: not the
 * boot-time entry of the remembered choice, not a sweep of a removed profile.
 */
export function useProfileSwitchEpoch(): number {
  return useSelector(profileViewStore, state => state.context.switchEpoch);
}

/** A view of routes that serve one profile at a time, for leaves rendered in every row. */
export interface ProfilePerProfileScope {
  /** Cache-key segment: a profile name, or the reserved `@all`. */
  key: string;
  /** The profiles to read one by one. Empty under the aggregate until the list resolves. */
  names: readonly string[];
  /** The profile anything created right now belongs to. */
  destination: string;
}

/**
 * The store's view, plus the two daemon reads only the aggregate needs: its profiles, without the
 * archived ones that refuse reads, and the remembered choice new work lands in.
 */
export function useProfilePerProfileScope(): ProfilePerProfileScope {
  const view = usePinnedProfileView(
    useSelector(profileViewStore, state => state.context.view) ?? {
      kind: "profile" as const,
      profile: PERMANENT_PROFILE,
    }
  );
  const aggregate = view.kind === "aggregate";
  const remembered = useRememberedProfileName(aggregate);
  const profiles = useProfiles(aggregate);
  return {
    key: profileViewKey(view),
    names:
      view.kind === "aggregate"
        ? (profiles.data ?? []).flatMap(row => (isArchived(row) ? [] : [row.name]))
        : [view.profile],
    destination: actingProfile(view, remembered),
  };
}

async function rememberProfileView(view: ProfileView): Promise<ProfileView> {
  if (view.kind === "aggregate") return view;
  await putProfileSelection({ scope: "global", profile: view.profile });
  return view;
}

/**
 * Switches the active view and remembers it.
 *
 * The local view moves first so the switch feels immediate, then the Global
 * remembered choice is persisted. If the persist fails the view rolls back to
 * exactly what it was — an operator is never left looking at a context the
 * machine disagrees about. The aggregate is a way of looking, not of acting, so
 * the daemon never stores it; the client keeps it across reloads instead.
 */
export function useSwitchProfile() {
  const queryClient = useQueryClient();
  const invalidateSelection = (view: ProfileView) => {
    if (view.kind === "profile") {
      void queryClient.invalidateQueries({ queryKey: profileKeys.selections() });
    }
  };
  // Once the daemon holds the new choice the cache says so at once, so the gap between the write
  // and its reread never acts as the previous choice; a read already in flight is dropped first so
  // its older answer cannot land on top. While the write is pending, useRememberedProfileName
  // answers from the mutation instead.
  const rememberPersisted = async (view: ProfileView) => {
    if (view.kind !== "profile") return;
    await queryClient.cancelQueries({ queryKey: profileKeys.selection(), exact: true });
    queryClient.setQueryData<ProfileSelectionPayload>(profileKeys.selection(), {
      scope: "global",
      profile: view.profile,
    });
  };
  const mutation = useMutation({
    mutationKey: profileKeys.selectionWrite(),
    // Writes run in the order the operator switched, so the daemon ends on the newest choice
    // even when an older write is slow; the view itself still moves at once in onMutate.
    scope: { id: "profile-selection-write" },
    mutationFn: rememberProfileView,
    onMutate: (view: ProfileView) => ({ switchId: setProfileView(view) }),
    onSuccess: async (view, _view, context) => {
      if (context.switchId !== null) settleProfileSwitch(context.switchId);
      await rememberPersisted(view);
    },
    onError: (error, _view, context) => {
      // Switches overlap: the store rolls this one back along the pending chain, so a later
      // switch keeps the view and a chain of failures returns to where the operator started.
      if (context && context.switchId !== null) failProfileSwitch(context.switchId);
      notifyUser({
        message: error instanceof Error ? error.message : "Could not switch profile.",
        tone: "error",
      });
    },
    onSettled: (_data, _error, view) => {
      invalidateSelection(view);
    },
  });
  return {
    ...mutation,
    // A delegated command must return its persisted result before activation
    // replaces the connection carrying that command.
    prepare: async (view: ProfileView) => {
      await rememberProfileView(view);
      await rememberPersisted(view);
      return () => {
        // Already persisted: the switch is settled the moment it is shown.
        const switchId = setProfileView(view);
        if (switchId !== null) settleProfileSwitch(switchId);
        invalidateSelection(view);
      };
    },
  };
}

export { PROFILE_AGGREGATE };
