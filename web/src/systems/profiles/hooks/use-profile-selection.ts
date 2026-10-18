import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";

import { notifyUser } from "@/lib/user-feedback";

import { putProfileSelection } from "../adapters/profiles-api";
import { PERMANENT_PROFILE } from "../lib/profile-rows";
import { profileKeys, profileLensKey } from "../lib/query-keys";
import { profileSelectionOptions } from "../lib/query-options";
import {
  carryProfileView,
  enterProfileView,
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
  const rememberedProfile = remembered.data?.profile;
  const remembering = remembered.isFetching;
  const workspaceId = lens.scope === "workspace" ? lens.workspaceId : null;
  const lensKey = profileLensKey(lens);
  const [activeLens, setActiveLens] = useState(lens);
  const [pendingTransition, setPendingTransition] = useState<ProfileViewTransition | null>(null);
  const activeLensKey = profileLensKey(activeLens);
  const destinationLocal = viewByLens[lensKey];

  if (activeLensKey !== lensKey) {
    const sourceView = viewByLens[activeLensKey];
    setActiveLens(lens);
    setPendingTransition({
      from: activeLens,
      to: lens,
      view: activeLens.scope !== lens.scope ? sourceView : undefined,
    });
  }

  const transitionForCurrentLens =
    pendingTransition && profileLensKey(pendingTransition.to) === lensKey
      ? pendingTransition
      : null;
  if (
    transitionForCurrentLens &&
    (transitionForCurrentLens.view
      ? sameProfileView(destinationLocal, transitionForCurrentLens.view)
      : viewByLens[profileLensKey(transitionForCurrentLens.from)] === undefined)
  ) {
    setPendingTransition(null);
  }
  const local = transitionForCurrentLens?.view ?? destinationLocal;

  useEffect(() => {
    if (!pendingTransition) return;
    if (pendingTransition.view) {
      carryProfileView(pendingTransition.from, pendingTransition.to);
    } else {
      // Re-entering a project must resolve its remembered choice, including after All profiles.
      restoreProfileView(pendingTransition.from, null);
    }
  }, [pendingTransition]);

  useEffect(() => {
    // A cached, invalidated choice is not the settled default for this entry.
    if (!enabled || remembering || rememberedProfile === undefined) return;
    const enteredLens: ProfileLens =
      workspaceId === null ? { scope: "global" } : { scope: "workspace", workspaceId };
    enterProfileView(enteredLens, { kind: "profile", profile: rememberedProfile });
  }, [enabled, lens.scope, rememberedProfile, remembering, workspaceId]);

  if (local) return local;
  return { kind: "profile", profile: rememberedProfile ?? PERMANENT_PROFILE };
}

interface ProfileViewTransition {
  from: ProfileLens;
  to: ProfileLens;
  view: ProfileView | undefined;
}

function sameProfileView(left: ProfileView | undefined, right: ProfileView): boolean {
  if (!left || left.kind !== right.kind) return false;
  if (left.kind === "aggregate") return true;
  return right.kind === "profile" && left.profile === right.profile;
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
  const remember = async (view: ProfileView) => {
    if (view.kind === "aggregate") return view;
    await putProfileSelection({
      scope: lens.scope,
      profile: view.profile,
      ...(lens.scope === "workspace" ? { workspace_id: lens.workspaceId } : {}),
    });
    return view;
  };
  const invalidateSelection = (view: ProfileView) => {
    if (view.kind === "profile") {
      void queryClient.invalidateQueries({ queryKey: profileKeys.selections() });
    }
  };
  const mutation = useMutation({
    mutationFn: remember,
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
    onSettled: (_data, _error, view) => {
      invalidateSelection(view);
    },
  });
  return {
    ...mutation,
    // A delegated command must return its persisted result before activation
    // replaces the connection carrying that command.
    prepare: async (view: ProfileView) => {
      await remember(view);
      return () => {
        setProfileView(lens, view);
        invalidateSelection(view);
      };
    },
  };
}

export { PROFILE_AGGREGATE };
