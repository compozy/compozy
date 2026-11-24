import { createContext, useContext, type ReactNode } from "react";

import { UpgradeSheet } from "../components/account-upgrade-sheet";
import {
  useUpgradeSheet,
  type OpenUpgradeSheetOptions,
  type UpgradeSheetController,
} from "../hooks/use-upgrade-sheet";

export interface UpgradeSheetActions {
  open: boolean;
  openUpgradeSheet: (options?: OpenUpgradeSheetOptions) => void;
}

const UpgradeSheetContext = createContext<UpgradeSheetActions | null>(null);

export interface AccountUpgradeSheetProviderProps {
  children: ReactNode;
  waitingTimeoutMs?: number;
}

/**
 * Mounts one live U9 Upgrade sheet at the app shell so every paywall surface
 * (Settings → Account, Settings → Computers, Add computer, Run on pickers,
 * and session ComputerChip) opens the same checkout flow without losing drafts.
 */
export function AccountUpgradeSheetProvider({
  children,
  waitingTimeoutMs,
}: AccountUpgradeSheetProviderProps) {
  const sheet = useUpgradeSheet({ waitingTimeoutMs });

  return (
    <UpgradeSheetContext value={{ open: sheet.open, openUpgradeSheet: sheet.openUpgradeSheet }}>
      {children}
      <ConnectedUpgradeSheet sheet={sheet} />
    </UpgradeSheetContext>
  );
}

export function ConnectedUpgradeSheet({ sheet }: { sheet: UpgradeSheetController }) {
  return (
    <UpgradeSheet
      checkoutCancelled={sheet.checkoutCancelled}
      checkoutError={sheet.checkoutError}
      interval={sheet.interval}
      isCheckingStatus={sheet.isCheckingStatus}
      isStartingCheckout={sheet.isStartingCheckout}
      onCancelWaiting={sheet.handleCancelWaiting}
      onCheckAgain={sheet.handleCheckAgain}
      onClose={sheet.handleClose}
      onCompleteUpgrade={sheet.handleCompleteUpgrade}
      onContinueToCheckout={sheet.handleContinueToCheckout}
      onIntervalChange={sheet.setInterval}
      onReopenCheckout={sheet.handleReopenCheckout}
      open={sheet.open}
      phase={sheet.phase}
    />
  );
}

/** Returns the shell-level Upgrade sheet trigger when mounted inside `AccountUpgradeSheetProvider`. */
export function useOptionalUpgradeSheetActions(): UpgradeSheetActions | null {
  return useContext(UpgradeSheetContext);
}
