import { useSelector, useStore } from "@xstate/store-react";

import {
  createMarketplaceInstalledCardConfirmLogic,
  type MarketplaceInstalledCardAction,
} from "./marketplace-installed-card-confirm-store";

const marketplaceInstalledCardConfirmLogic = createMarketplaceInstalledCardConfirmLogic();

export function useMarketplaceInstalledCardConfirmation(
  execute: (action: MarketplaceInstalledCardAction) => Promise<void>
) {
  const store = useStore(marketplaceInstalledCardConfirmLogic);
  const state = useSelector(store, snapshot => snapshot.context);

  return {
    action: state.phase === "closed" ? null : state.action,
    error: state.phase === "failed" ? state.error : null,
    isPending: state.phase === "pending",
    open: state.phase !== "closed",
    cancel: () => store.trigger.confirmationCancelled(),
    openConfirmation: (action: MarketplaceInstalledCardAction) =>
      store.trigger.confirmationOpened({ action }),
    confirm: () => store.trigger.confirmationRequested({ execute }),
  };
}
