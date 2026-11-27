import { useMemo, type ReactNode } from "react";

import { UpgradeSheetContext } from "../contexts/upgrade-sheet-context";
import { useUpgradeSheet, type UpgradeSheetController } from "../hooks/use-upgrade-sheet";
import { UpgradeSheet } from "./account-upgrade-sheet";

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
  const { open, openUpgradeSheet } = sheet;
  const actions = useMemo(() => ({ open, openUpgradeSheet }), [open, openUpgradeSheet]);

  return (
    <UpgradeSheetContext value={actions}>
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
