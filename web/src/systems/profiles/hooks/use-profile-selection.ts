import { useEffect } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { notifyUser } from "@/lib/user-feedback";

import { putProfileSelection } from "../adapters/profiles-api";
import { PERMANENT_PROFILE } from "../lib/profile-rows";
import { profileKeys } from "../lib/query-keys";
import { profileSelectionOptions } from "../lib/query-options";
import {
  enterProfileView,
  localProfileView,
  profileViewStore,
  restoreProfileView,
  setProfileView,
} from "../stores/profile-view-store";
import { PROFILE_AGGREGATE, type ProfileView } from "../types";

/** The remembered Global choice — daemon state, cached for instant render. */
export function useRememberedProfile(enabled = true) {
  return useQuery(profileSelectionOptions(enabled));
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
  const local = useSelector(profileViewStore, state => state.context.view);
  return local?.kind === "profile" ? local.profile : PERMANENT_PROFILE;
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
  const mutation = useMutation({
    mutationFn: rememberProfileView,
    onMutate: (view: ProfileView) => {
      const previous = localProfileView();
      setProfileView(view);
      return { previous };
    },
    onError: (error, _view, context) => {
      restoreProfileView(context?.previous ?? null);
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
      return () => {
        setProfileView(view);
        invalidateSelection(view);
      };
    },
  };
}

export { PROFILE_AGGREGATE };
