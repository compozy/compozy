import { useCallback, useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { evictAccountScopedState } from "@/lib/computer-scope";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";

import {
  AccountApiError,
  fetchAccountStatus,
  openAccountStream,
  startAccountCheckout,
} from "../adapters/account-api";
import type { UpgradeInterval, UpgradeSheetPhase } from "../components/account-upgrade-sheet";
import { describeBillingApiError } from "../lib/account-view";
import { openBillingBrowserWindow, reopenBillingUrl } from "../lib/billing-window";
import { accountKeys } from "../lib/query-keys";
import { accountStatusOptions } from "../lib/query-options";

/** 15 minutes before showing `Still waiting? Check again` (`_uiux.md` U9). */
export const UPGRADE_CHECKOUT_TIMEOUT_MS = 15 * 60 * 1000;

export interface OpenUpgradeSheetOptions {
  interval?: UpgradeInterval;
  /** Invoked once after the account reaches Pro and the sheet closes / finishes. */
  onUpgraded?: () => void;
}

export interface UseUpgradeSheetOptions {
  waitingTimeoutMs?: number;
}

export interface UpgradeSheetController {
  open: boolean;
  phase: UpgradeSheetPhase;
  interval: UpgradeInterval;
  checkoutUrl: string | null;
  checkoutError: string | null;
  checkoutCancelled: boolean;
  isStartingCheckout: boolean;
  isCheckingStatus: boolean;
  openUpgradeSheet: (options?: OpenUpgradeSheetOptions) => void;
  setInterval: (next: UpgradeInterval) => void;
  handleContinueToCheckout: (chosen: UpgradeInterval) => void;
  handleReopenCheckout: () => void;
  handleCheckAgain: () => void;
  handleCancelWaiting: () => void;
  handleCompleteUpgrade: () => void;
  handleClose: () => void;
}

/**
 * Live controller for the U9 Upgrade sheet (`_uiux.md` U9, US-002):
 * - Opens Stripe Checkout via `POST /api/account/checkout`
 * - Watches `GET /api/account/stream` (and account query invalidation) for `plan: "pro"`
 *   even when remote access is off (ADR-007 §6)
 * - Shows `Still waiting? Check again` after 15 minutes with a manual `GET /api/account` check
 * - Handles cancelled checkout waits and errors cleanly
 */
export function useUpgradeSheet(options: UseUpgradeSheetOptions = {}): UpgradeSheetController {
  const waitingTimeoutMs = options.waitingTimeoutMs ?? UPGRADE_CHECKOUT_TIMEOUT_MS;
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [checkoutPhase, setPhase] = useState<UpgradeSheetPhase>("select");
  const [interval, setCheckoutInterval] = useState<UpgradeInterval>("year");
  const [checkoutUrl, setCheckoutUrl] = useState<string | null>(null);
  const [checkoutError, setCheckoutError] = useState<string | null>(null);
  const [checkoutCancelled, setCheckoutCancelled] = useState(false);
  const [isStartingCheckout, setIsStartingCheckout] = useState(false);
  const [isCheckingStatus, setIsCheckingStatus] = useState(false);

  const onUpgradedRef = useRef<(() => void) | undefined>(undefined);
  const upgradedNotifiedRef = useRef(false);

  const waitingForCheckout =
    open && (checkoutPhase === "waiting" || checkoutPhase === "waiting_timeout");
  const statusQuery = useQuery(accountStatusOptions(waitingForCheckout));
  const phase = waitingForCheckout && statusQuery.data?.plan === "pro" ? "done" : checkoutPhase;
  const isWaiting = waitingForCheckout && phase !== "done";

  const invalidateBillingQueries = useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: accountKeys.all });
    void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
  }, [queryClient]);

  const markUpgraded = useCallback(() => {
    setPhase("done");
    setCheckoutError(null);
    setCheckoutCancelled(false);
    invalidateBillingQueries();
  }, [invalidateBillingQueries]);

  const flushOnUpgraded = useCallback(() => {
    if (upgradedNotifiedRef.current) return;
    upgradedNotifiedRef.current = true;
    const callback = onUpgradedRef.current;
    onUpgradedRef.current = undefined;
    callback?.();
  }, []);

  // Subscribe to `/api/account/stream` while waiting for checkout completion.
  useEffect(() => {
    if (!isWaiting) return;
    let active = true;
    const close = openAccountStream({
      onAccountChanged: event => {
        if (!active) return;
        if (event.evict_remote) {
          evictAccountScopedState(queryClient);
          onUpgradedRef.current = undefined;
          setOpen(false);
          return;
        }
        invalidateBillingQueries();
        if (event.plan === "pro") {
          markUpgraded();
          return;
        }
        if (event.kind === "plan.changed" || event.kind === "profile") {
          void fetchAccountStatus()
            .then(fresh => {
              if (!active) return;
              queryClient.setQueryData(accountKeys.status(), fresh);
              if (fresh.plan === "pro") {
                markUpgraded();
              }
            })
            .catch(() => undefined);
        }
      },
      onReset: () => {
        if (!active) return;
        void fetchAccountStatus()
          .then(fresh => {
            if (!active) return;
            queryClient.setQueryData(accountKeys.status(), fresh);
            if (fresh.plan === "pro") {
              markUpgraded();
            }
          })
          .catch(() => undefined);
      },
    });
    return () => {
      active = false;
      close();
    };
  }, [isWaiting, invalidateBillingQueries, markUpgraded, queryClient]);

  // Move from `waiting` to `waiting_timeout` after 15 minutes (`_uiux.md` U9).
  useEffect(() => {
    if (!open || phase !== "waiting") return;
    const timer = window.setTimeout(() => {
      setPhase("waiting_timeout");
    }, waitingTimeoutMs);
    return () => window.clearTimeout(timer);
  }, [open, phase, waitingTimeoutMs]);

  const openUpgradeSheet = useCallback((next?: OpenUpgradeSheetOptions) => {
    onUpgradedRef.current = next?.onUpgraded;
    upgradedNotifiedRef.current = false;
    setCheckoutInterval(next?.interval ?? "year");
    setPhase("select");
    setCheckoutUrl(null);
    setCheckoutError(null);
    setCheckoutCancelled(false);
    setIsStartingCheckout(false);
    setIsCheckingStatus(false);
    setOpen(true);
  }, []);

  const handleContinueToCheckout = useCallback(
    (chosen: UpgradeInterval) => {
      if (isStartingCheckout) return;
      setCheckoutInterval(chosen);
      setCheckoutError(null);
      setCheckoutCancelled(false);
      setIsStartingCheckout(true);

      const tab = openBillingBrowserWindow();
      void startAccountCheckout(chosen)
        .then(result => {
          setCheckoutUrl(result.url);
          tab.navigate(result.url);
          setPhase("waiting");
        })
        .catch(err => {
          tab.close();
          if (err instanceof AccountApiError && err.code === "already_pro") {
            markUpgraded();
            return;
          }
          setCheckoutError(describeBillingApiError(err, "Couldn't start checkout. Try again."));
        })
        .finally(() => {
          setIsStartingCheckout(false);
        });
    },
    [isStartingCheckout, markUpgraded]
  );

  const handleReopenCheckout = useCallback(() => {
    if (checkoutUrl) {
      reopenBillingUrl(checkoutUrl);
      return;
    }
    handleContinueToCheckout(interval);
  }, [checkoutUrl, handleContinueToCheckout, interval]);

  const handleCheckAgain = useCallback(() => {
    if (isCheckingStatus) return;
    setIsCheckingStatus(true);
    void fetchAccountStatus()
      .then(fresh => {
        queryClient.setQueryData(accountKeys.status(), fresh);
        invalidateBillingQueries();
        if (fresh.plan === "pro") {
          markUpgraded();
        } else {
          toast.message("Still on Free. Finish checkout in your browser, then check again.");
        }
      })
      .catch(err => {
        toast.error(describeBillingApiError(err, "Couldn't check your plan right now."));
      })
      .finally(() => {
        setIsCheckingStatus(false);
      });
  }, [isCheckingStatus, queryClient, invalidateBillingQueries, markUpgraded]);

  const handleCancelWaiting = useCallback(() => {
    setPhase("select");
    setCheckoutCancelled(true);
    setCheckoutError(null);
  }, []);

  const handleCompleteUpgrade = useCallback(() => {
    invalidateBillingQueries();
    setOpen(false);
    flushOnUpgraded();
  }, [invalidateBillingQueries, flushOnUpgraded]);

  const handleClose = useCallback(() => {
    const wasDone = phase === "done";
    setOpen(false);
    if (wasDone) {
      invalidateBillingQueries();
      flushOnUpgraded();
    } else {
      onUpgradedRef.current = undefined;
    }
  }, [phase, invalidateBillingQueries, flushOnUpgraded]);

  return {
    open,
    phase,
    interval,
    checkoutUrl,
    checkoutError,
    checkoutCancelled,
    isStartingCheckout,
    isCheckingStatus,
    openUpgradeSheet,
    setInterval: setCheckoutInterval,
    handleContinueToCheckout,
    handleReopenCheckout,
    handleCheckAgain,
    handleCancelWaiting,
    handleCompleteUpgrade,
    handleClose,
  };
}
