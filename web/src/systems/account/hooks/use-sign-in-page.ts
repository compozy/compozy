import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { toast } from "@compozy/ui";

import { evictRemoteComputerQueries } from "@/lib/computer-scope";
import { unlinkRemoteHost } from "@/systems/computers/adapters/remote-host-api";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import type {
  SignInProvider,
  SignInWaitingAttemptFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import {
  AccountApiError,
  cancelAccountLogin,
  fetchAccountStatus,
  openAccountSignInStream,
  startAccountLogin,
} from "../adapters/account-api";
import { openBillingBrowserWindow, reopenBillingUrl } from "../lib/billing-window";
import { accountKeys } from "../lib/query-keys";
import { accountStatusOptions } from "../lib/query-options";
import {
  isAccountGraceActive,
  noticeFromSignInError,
  resolveSignInView,
  sanitizeSignInRedirect,
  type ResolvedSignInNotice,
} from "../lib/sign-in-gate";

export interface UseSignInPageOptions {
  redirectTo?: string;
}

export function useSignInPage(options: UseSignInPageOptions = {}) {
  const queryClient = useQueryClient();
  const router = useRouter();
  const redirectTarget = sanitizeSignInRedirect(options.redirectTo) ?? "/";

  const statusQuery = useQuery(accountStatusOptions());
  const [marketingOptIn, setMarketingOptIn] = useState(false);
  const [waiting, setWaiting] = useState<SignInWaitingAttemptFixture | null>(null);
  const [notice, setNotice] = useState<ResolvedSignInNotice | null>(null);
  const [cloudUnreachable, setCloudUnreachable] = useState(false);
  const [rateLimitUntilMs, setRateLimitUntilMs] = useState<number | null>(null);
  const [nowMs, setNowMs] = useState(() => Date.now());

  const activeAttemptIdRef = useRef<string | null>(null);
  const completedRef = useRef(false);

  const rateLimitRemainingSeconds = useMemo(() => {
    if (rateLimitUntilMs === null) return 0;
    return Math.max(0, Math.ceil((rateLimitUntilMs - nowMs) / 1000));
  }, [rateLimitUntilMs, nowMs]);

  useEffect(() => {
    if (rateLimitUntilMs === null) return;
    const timer = setInterval(() => {
      const currentNow = Date.now();
      setNowMs(currentNow);
      if (currentNow >= rateLimitUntilMs) {
        setRateLimitUntilMs(null);
        setNotice(current => (current?.kind === "rate_limited" ? null : current));
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [rateLimitUntilMs]);

  const finishSignIn = useCallback(async () => {
    if (completedRef.current) return;
    completedRef.current = true;
    activeAttemptIdRef.current = null;
    setWaiting(null);
    setNotice(null);
    setCloudUnreachable(false);
    try {
      const latest = await fetchAccountStatus();
      queryClient.setQueryData(accountKeys.status(), latest);
    } catch {
      await queryClient.invalidateQueries({ queryKey: accountKeys.status() });
    }
    evictRemoteComputerQueries(queryClient, { includeComputers: true });
    void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    void router.navigate({ href: redirectTarget, replace: true });
  }, [queryClient, redirectTarget, router]);

  // If the account becomes signed in (or dev build has gate off), hand over to the app immediately.
  useEffect(() => {
    const status = statusQuery.data;
    if (!status) return;
    if (status.signed_in || status.state === "signed_in" || status.gate === "off") {
      void finishSignIn();
    }
  }, [statusQuery.data, finishSignIn]);

  // Keep `/api/account/sign-in/events` subscribed while on U0 to drive waiting, errors, and completion.
  useEffect(() => {
    return openAccountSignInStream({
      onProgress: progress => {
        const currentAttempt = activeAttemptIdRef.current;
        if (progress.attempt_id && currentAttempt && progress.attempt_id !== currentAttempt) {
          return;
        }
        if (
          progress.state === "signed_in" ||
          progress.state === "done" ||
          progress.state === "succeeded"
        ) {
          void finishSignIn();
          return;
        }
        if (progress.state === "cancelled" || progress.state === "canceled") {
          activeAttemptIdRef.current = null;
          setWaiting(null);
          setNotice({ kind: "cancelled", message: "Sign-in was cancelled." });
          return;
        }
        if (progress.state === "expired") {
          activeAttemptIdRef.current = null;
          setWaiting(null);
          setNotice({ kind: "timed_out", message: "Sign-in timed out. Try again." });
          return;
        }
        if (progress.state === "failed") {
          activeAttemptIdRef.current = null;
          setWaiting(null);
          const resolved = noticeFromSignInError({ code: progress.code });
          if (resolved.kind === "cloud_unreachable") {
            setCloudUnreachable(true);
          }
          setNotice(resolved);
        }
      },
      onState: state => {
        if (state.state === "signed_in") {
          void finishSignIn();
          return;
        }
        void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      },
    });
  }, [finishSignIn, queryClient]);

  const startLoginMutation = useMutation({
    mutationFn: async (provider: SignInProvider) => {
      const browserWindow = openBillingBrowserWindow();
      try {
        const response = await startAccountLogin({
          provider,
          marketing_opt_in: marketingOptIn,
        });
        const targetUrl = response.verification_uri_complete || response.verification_uri;
        browserWindow.navigate(targetUrl);
        return { provider, response };
      } catch (err) {
        browserWindow.close();
        throw err;
      }
    },
    onSuccess: ({ provider, response }) => {
      activeAttemptIdRef.current = response.attempt_id;
      setNotice(null);
      setCloudUnreachable(false);
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      setWaiting({
        attemptId: response.attempt_id,
        provider,
        userCode: response.user_code,
        verificationUri: response.verification_uri,
        verificationUriComplete: response.verification_uri_complete,
      });
    },
    onError: err => {
      activeAttemptIdRef.current = null;
      setWaiting(null);
      if (err instanceof AccountApiError) {
        if (err.code === "rate_limited" || err.status === 429) {
          const seconds = err.retryAfterSeconds ?? 60;
          const now = Date.now();
          setNowMs(now);
          setRateLimitUntilMs(now + seconds * 1000);
          setNotice(noticeFromSignInError({ code: "rate_limited", retryAfterSeconds: seconds }));
          return;
        }
        if (
          err.code === "account_cloud_unreachable" ||
          err.code === "cloud_unavailable" ||
          err.status === 503
        ) {
          setCloudUnreachable(true);
          setNotice(noticeFromSignInError({ code: "account_cloud_unreachable" }));
          return;
        }
        setNotice(noticeFromSignInError({ code: err.code, message: err.message }));
        return;
      }
      setCloudUnreachable(true);
      setNotice(noticeFromSignInError({ code: "account_cloud_unreachable" }));
    },
  });

  const unlinkMutation = useMutation({
    mutationFn: () => unlinkRemoteHost(),
    onSuccess: () => {
      setNotice(null);
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      toast.success("Unlinked this computer from remote access.");
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to unlink this computer.");
    },
  });

  const handleSelectProvider = useCallback(
    (provider: SignInProvider) => {
      if (startLoginMutation.isPending) return;
      startLoginMutation.mutate(provider);
    },
    [startLoginMutation]
  );

  const handleCancelWaiting = useCallback(() => {
    const attemptId = waiting?.attemptId ?? activeAttemptIdRef.current ?? undefined;
    activeAttemptIdRef.current = null;
    setWaiting(null);
    setNotice({ kind: "cancelled", message: "Sign-in was cancelled." });
    void cancelAccountLogin(attemptId).catch(() => undefined);
  }, [waiting?.attemptId]);

  const handleReopenBrowser = useCallback((attempt: SignInWaitingAttemptFixture) => {
    reopenBillingUrl(attempt.verificationUriComplete || attempt.verificationUri);
  }, []);

  const handleRetryConnection = useCallback(() => {
    setCloudUnreachable(false);
    setNotice(null);
    void fetchAccountStatus(undefined, { refresh: true })
      .then(next => {
        queryClient.setQueryData(accountKeys.status(), next);
      })
      .catch(() => {
        setCloudUnreachable(true);
        setNotice(noticeFromSignInError({ code: "account_cloud_unreachable" }));
      });
  }, [queryClient]);

  const isGrace = isAccountGraceActive(statusQuery.data);
  const handleBackToApp = useMemo(
    () =>
      isGrace
        ? () => {
            void router.navigate({ href: redirectTarget });
          }
        : undefined,
    [isGrace, redirectTarget, router]
  );

  const view = useMemo(
    () =>
      resolveSignInView(statusQuery.data, {
        notice,
        rateLimitRemainingSeconds,
        cloudUnreachable,
      }),
    [statusQuery.data, notice, rateLimitRemainingSeconds, cloudUnreachable]
  );

  return {
    status: statusQuery.data,
    isLoading: statusQuery.isLoading,
    view,
    waiting: waiting ?? undefined,
    marketingOptIn,
    setMarketingOptIn,
    isUnlinking: unlinkMutation.isPending,
    handleSelectProvider,
    handleCancelWaiting,
    handleReopenBrowser,
    handleRetryConnection,
    handleUnlinkInstallation: () => unlinkMutation.mutate(),
    handleBackToApp,
  };
}
