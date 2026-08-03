import { useSelector, useStore } from "@xstate/store-react";

import { createMarketplaceInstalledCardConfirmLogic } from "./marketplace-installed-card-confirm-store";

const marketplaceInstalledCardConfirmLogic = createMarketplaceInstalledCardConfirmLogic();

export function useMarketplaceInstalledCardConfirmation(execute: () => Promise<void>) {
  const store = useStore(marketplaceInstalledCardConfirmLogic);
  const state = useSelector(store, snapshot => snapshot.context);

  return {
    error: state.phase === "failed" ? state.error : null,
    isPending: state.phase === "pending",
    open: state.phase !== "closed",
    cancel: () => store.trigger.confirmationCancelled(),
    openConfirmation: () => store.trigger.confirmationOpened(),
    confirm: () => store.trigger.confirmationRequested({ execute }),
  };
}
