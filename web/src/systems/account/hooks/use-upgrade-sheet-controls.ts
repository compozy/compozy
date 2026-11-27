import { useState } from "react";

import type {
  UpgradeInterval,
  UpgradeSheetPhase,
  UpgradeSheetProps,
} from "../components/account-upgrade-sheet";

export function useUpgradeSheetControls({
  phase: controlledPhase,
  initialPhase = "select",
  interval: controlledInterval,
  initialInterval = "year",
  onIntervalChange,
  onClose,
  onCancelWaiting,
  onContinueToCheckout,
  onCheckAgain,
  onCompleteUpgrade,
}: UpgradeSheetProps) {
  const [uncontrolledPhase, setUncontrolledPhase] = useState<UpgradeSheetPhase>(initialPhase);
  const [uncontrolledInterval, setUncontrolledInterval] =
    useState<UpgradeInterval>(initialInterval);

  const isControlledPhase = controlledPhase !== undefined;
  const phase = isControlledPhase ? controlledPhase : uncontrolledPhase;
  const interval = controlledInterval ?? uncontrolledInterval;

  const handleIntervalChange = (next: UpgradeInterval) => {
    if (controlledInterval === undefined) {
      setUncontrolledInterval(next);
    }
    onIntervalChange?.(next);
  };

  const handleContinue = () => {
    onContinueToCheckout?.(interval);
    if (!isControlledPhase) {
      setUncontrolledPhase("waiting");
    }
  };

  const handleCheckAgain = () => {
    onCheckAgain?.();
    if (!isControlledPhase) {
      onCompleteUpgrade?.();
      setUncontrolledPhase("done");
    }
  };

  const handleFinish = () => {
    onCompleteUpgrade?.();
    onClose();
  };

  const handleCancelWaiting = () => {
    if (onCancelWaiting) {
      onCancelWaiting();
      return;
    }
    onClose();
  };

  return {
    phase,
    interval,
    handleIntervalChange,
    handleContinue,
    handleCheckAgain,
    handleFinish,
    handleCancelWaiting,
  };
}
