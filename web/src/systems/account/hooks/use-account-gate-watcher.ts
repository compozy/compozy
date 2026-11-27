import { useCallback, useEffect, useRef } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { observeAccountGateSignal } from "@/lib/account-gate-signal";
import { evictAccountScopedState } from "@/lib/computer-scope";
import { isHostedOriginCandidate } from "@/lib/hosted-browser-auth";

import { fetchAccountStatus, openAccountSignInStream } from "../adapters/account-api";
import { accountKeys } from "../lib/query-keys";
import { accountStatusOptions } from "../lib/query-options";
import {
  isAccountGateBlocked,
  isAccountGraceActive,
  sanitizeSignInRedirect,
} from "../lib/sign-in-gate";

const MAX_TIMEOUT_MS = 2_147_483_647;

/**
 * Watches daemon sign-in gate transitions (`4401` stream close, `401 sign_in_required`,
 * and grace expiration) and moves the app to `/sign-in` via TanStack Router without a
 * page reload or redirect loop. Disabled on hosted origins (owned by U8).
 */
export function useAccountGateWatcher(): void {
  const hosted = isHostedOriginCandidate();
  const queryClient = useQueryClient();
  const router = useRouter();
  const inFlightRef = useRef(false);

  const statusQuery = useQuery(accountStatusOptions(!hosted));
  const graceActive = !hosted && isAccountGraceActive(statusQuery.data);
  const graceUntil = statusQuery.data?.grace?.until ?? null;

  const verifyGateAndRedirect = useCallback(async () => {
    if (hosted || inFlightRef.current) return;
    inFlightRef.current = true;
    try {
      const next = await fetchAccountStatus();
      if (!isAccountGateBlocked(next)) {
        queryClient.setQueryData(accountKeys.status(), next);
        return;
      }

      // Cancel and remove all prior account- and computer-scoped caches and stores, seeding
      // only the newly confirmed blocked status (Rule 23, AST-010).
      evictAccountScopedState(queryClient, { seedStatus: next });
      const location = router.state?.location;
      if (!location || typeof router.navigate !== "function") return;
      if (location.pathname === "/sign-in") return;
      const redirectTarget = sanitizeSignInRedirect(location.href);
      await router.navigate({
        to: "/sign-in",
        search: redirectTarget ? { redirect: redirectTarget } : {},
        replace: true,
      });
    } catch {
      // Do not bounce to sign-in on transient network failures when the daemon is unreachable.
    } finally {
      inFlightRef.current = false;
    }
  }, [hosted, queryClient, router]);

  useEffect(() => {
    if (hosted) return;
    return observeAccountGateSignal(() => {
      void verifyGateAndRedirect();
    });
  }, [hosted, verifyGateAndRedirect]);

  // While running in grace mode, watch `/api/account/sign-in/events` and the grace deadline
  // so ending grace moves the app to U0 immediately.
  useEffect(() => {
    if (!graceActive) return;
    const closeStream = openAccountSignInStream({
      onState: () => {
        void verifyGateAndRedirect();
      },
    });

    let timer: ReturnType<typeof setTimeout> | null = null;
    if (graceUntil) {
      const targetMs = Date.parse(graceUntil);
      if (!Number.isNaN(targetMs)) {
        const delay = Math.min(MAX_TIMEOUT_MS, Math.max(500, targetMs - Date.now() + 500));
        timer = setTimeout(() => {
          void verifyGateAndRedirect();
        }, delay);
      }
    }

    return () => {
      closeStream();
      if (timer !== null) clearTimeout(timer);
    };
  }, [graceActive, graceUntil, verifyGateAndRedirect]);
}
