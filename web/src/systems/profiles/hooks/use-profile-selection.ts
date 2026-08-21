import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { notifyUser } from "@/lib/user-feedback";

import { putProfileSelection } from "../adapters/profiles-api";
import { PERMANENT_PROFILE } from "../lib/profile-rows";
import { profileKeys, profileLensKey } from "../lib/query-keys";
import { profileSelectionOptions } from "../lib/query-options";
import {
  localProfileView,
  profileViewStore,
  restoreProfileView,
  setProfileView,
} from "../stores/profile-view-store";
import { PROFILE_AGGREGATE, type ProfileLens, type ProfileView } from "../types";

/** The remembered choice for a lens — daemon state, cached for instant render. */
export function useRememberedProfile(lens: ProfileLens, enabled = true) {
  return useQuery(profileSelectionOptions(lens, enabled));
}

/**
 * What this client is currently acting as, for one lens.
 *
 * Local view first, remembered choice second, `default` last — the same ladder
 * the resolver applies, minus the flag and environment rungs that only exist in
 * a terminal.
 */
export function useActiveProfileView(lens: ProfileLens, enabled = true): ProfileView {
  const remembered = useRememberedProfile(lens, enabled);
  const viewByLens = useSelector(profileViewStore, state => state.context.viewByLens);
  const local = viewByLens[profileLensKey(lens)];
  if (local) return local;
  return { kind: "profile", profile: remembered.data?.profile ?? PERMANENT_PROFILE };
}

/**
 * Switches the active view and remembers it.
 *
 * The local view moves first so the switch feels immediate, then the remembered
 * choice is persisted. If the persist fails the view rolls back to exactly what
 * it was — an operator is never left looking at a context the machine disagrees
 * about. The aggregate is a way of looking, not of acting, so it is never
 * persisted (ADR-003).
 */
export function useSwitchProfile(lens: ProfileLens) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (view: ProfileView) => {
      if (view.kind === "aggregate") return view;
      await putProfileSelection({
        scope: lens.scope,
        profile: view.profile,
        ...(lens.scope === "workspace" ? { workspace_id: lens.workspaceId } : {}),
      });
      return view;
    },
    onMutate: (view: ProfileView) => {
      const previous = localProfileView(lens);
      setProfileView(lens, view);
      return { previous };
    },
    onError: (error, _view, context) => {
      restoreProfileView(lens, context?.previous ?? null);
      notifyUser({
        message: error instanceof Error ? error.message : "Could not switch profile.",
        tone: "error",
      });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: profileKeys.selections() }),
  });
}

export { PROFILE_AGGREGATE };
