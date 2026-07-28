import { useSelector } from "@xstate/store-react";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useEffectEvent } from "react";
import { toast } from "sonner";

import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  useActivateMarketplaceBundle,
  usePreviewMarketplaceBundle,
} from "../hooks/use-marketplace-actions";
import type { MarketplaceEntryResponse } from "../types";
import { buildBundleRequest } from "./bundle-activation-model";
import {
  createBundleActivationDialogLogic,
  type BundleActivationDraft,
} from "./bundle-activation-dialog-store";

interface UseBundleActivationDialogOptions {
  data: MarketplaceEntryResponse;
  onOpenChange: (open: boolean) => void;
  workspaceId?: string | null;
}

const bundleActivationDialogLogic = createBundleActivationDialogLogic();

export function useBundleActivationDialog({
  data,
  onOpenChange,
  workspaceId,
}: UseBundleActivationDialogOptions) {
  const bundle = data.bundle;
  const preview = usePreviewMarketplaceBundle();
  const activate = useActivateMarketplaceBundle();
  const navigate = useNavigate();
  const bindingKey = JSON.stringify([data.entry.entry_id, workspaceId ?? null]);
  const { store } = useStoreBinding(bindingKey, () =>
    bundleActivationDialogLogic.createStore({
      profile: bundle?.profiles[0]?.name ?? "default",
      scope: workspaceId ? "workspace" : "global",
      confirmNetworkRequirement: false,
    })
  );
  const state = useSelector(store, snapshot => snapshot.context);

  const executePreview = (draft: BundleActivationDraft) =>
    preview
      .mutateAsync(buildBundleRequest(data, draft.profile, draft.scope, workspaceId))
      .then(() => undefined);
  const executeActivation = (draft: BundleActivationDraft) =>
    activate
      .mutateAsync(
        buildBundleRequest(
          data,
          draft.profile,
          draft.scope,
          workspaceId,
          draft.confirmNetworkRequirement
        )
      )
      .then(() => undefined);

  const providePreviewExecution = useEffectEvent(() => {
    store.trigger.previewExecutionProvided({ preview: executePreview });
  });
  const notifyActivation = () => {
    toast.success(`${data.entry.name} activated`, {
      action: {
        label: "Manage →",
        onClick: () => void navigate({ to: "/marketplace/bundles", search: { tab: "installed" } }),
      },
    });
  };
  const handleActivated = useEffectEvent(() => {
    onOpenChange(false);
  });

  useEffect(() => {
    if (bundle) providePreviewExecution();
  }, [bundle, store]);

  useEffect(() => {
    const subscription = store.on("activated", handleActivated);
    return () => subscription.unsubscribe();
  }, [store]);

  return {
    activateBundle: () =>
      store.trigger.activationSubmitted({
        activate: executeActivation,
        notifySuccess: notifyActivation,
      }),
    confirmNetworkRequirement: state.confirmNetworkRequirement,
    error: state.phase === "failed" ? new Error(state.error) : null,
    phase: state.phase,
    preview,
    profile: state.profile,
    rerunPreview: () => {
      activate.reset();
      store.trigger.previewRetried({ preview: executePreview });
    },
    scope: state.scope,
    setConfirmNetworkRequirement: (confirmed: boolean) =>
      store.trigger.networkRequirementConfirmed({ confirmed }),
    setProfile: (profile: string) =>
      store.trigger.profileSelected({ preview: executePreview, profile }),
    setScope: (scope: "global" | "workspace") =>
      store.trigger.scopeSelected({ preview: executePreview, scope }),
  };
}

export type { UseBundleActivationDialogOptions };
