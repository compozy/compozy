import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";
import { ExternalLink, LogIn, RefreshCw, ShieldAlert } from "@compozy/ui/icons";

import { ActionResultBanner, Button, buttonVariants, Empty } from "@compozy/ui";

import { isHostedOriginCandidate } from "@/lib/hosted-browser-auth";

import { fetchAccountStatus, type AccountGracePayload } from "../adapters/account-api";
import { COMPOZY_STATUS_URL, formatGraceUntil } from "../lib/account-view";
import { accountKeys } from "../lib/query-keys";
import { accountStatusOptions } from "../lib/query-options";
import { isAccountGraceActive, sanitizeSignInRedirect } from "../lib/sign-in-gate";

export interface GateGraceBannerViewProps {
  grace: AccountGracePayload;
  isRetrying?: boolean;
  onSignIn?: () => void;
  onTryAgain?: () => void;
}

/**
 * Persistent, non-dismissible grace banner rendered at the top of the app shell
 * while `gate === "grace"` (`_uiux.md` U0, Rule 29, US-033).
 */
export function GateGraceBannerView({
  grace,
  isRetrying = false,
  onSignIn,
  onTryAgain,
}: GateGraceBannerViewProps) {
  const untilLabel = formatGraceUntil(grace.until);
  const isOffline = grace.source === "offline";
  const customMessage = grace.message?.trim();

  const message = isOffline
    ? `Can't reach CompozyOS to sign in. CompozyOS keeps working until ${untilLabel}.`
    : customMessage ||
      `Sign-in is having problems. CompozyOS keeps working until ${untilLabel}. Sign in when you can.`;

  return (
    <div
      className="border-b border-line bg-canvas px-3 py-2"
      data-grace-source={grace.source}
      data-slot="gate-grace-banner"
      data-testid="gate-grace-banner"
    >
      <ActionResultBanner
        actions={
          isOffline ? (
            <>
              <Button
                disabled={isRetrying}
                onClick={onTryAgain}
                size="sm"
                type="button"
                variant="neutral"
              >
                <RefreshCw aria-hidden="true" className={isRetrying ? "animate-spin" : undefined} />
                Try again
              </Button>
              <a
                className={buttonVariants({ size: "sm", variant: "ghost" })}
                href={COMPOZY_STATUS_URL}
                rel="noreferrer"
                target="_blank"
              >
                Status
                <ExternalLink aria-hidden="true" className="size-3" />
              </a>
            </>
          ) : (
            <Button onClick={onSignIn} size="sm" type="button" variant="neutral">
              <LogIn aria-hidden="true" />
              Sign in
            </Button>
          )
        }
        title={message}
        tone="warning"
      />
    </div>
  );
}

/** Connected grace banner mounted inside `DesktopShell`. */
export function GateGraceBanner() {
  const hosted = isHostedOriginCandidate();
  const queryClient = useQueryClient();
  const router = useRouter();
  const [isRetrying, setIsRetrying] = useState(false);
  const statusQuery = useQuery(accountStatusOptions(!hosted));

  if (hosted || !isAccountGraceActive(statusQuery.data) || !statusQuery.data?.grace) {
    return null;
  }

  const handleSignIn = () => {
    if (typeof router.navigate !== "function") return;
    const redirectTarget = sanitizeSignInRedirect(router.state?.location?.href);
    void router.navigate({
      to: "/sign-in",
      search: redirectTarget ? { redirect: redirectTarget } : {},
    });
  };

  const handleTryAgain = () => {
    if (isRetrying) return;
    setIsRetrying(true);
    void fetchAccountStatus(undefined, { refresh: true })
      .then(next => {
        queryClient.setQueryData(accountKeys.status(), next);
      })
      .catch(() => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      })
      .finally(() => setIsRetrying(false));
  };

  return (
    <GateGraceBannerView
      grace={statusQuery.data.grace}
      isRetrying={isRetrying}
      onSignIn={handleSignIn}
      onTryAgain={handleTryAgain}
    />
  );
}

export interface AccountGraceSignInPromptProps {
  onSignIn: () => void;
  testId?: string;
}

/**
 * Shown in account-dependent Settings surfaces (Computers, Account) while a
 * sign-in grace window applies (`_uiux.md` U0/U3, US-033 AC-3).
 */
export function AccountGraceSignInPrompt({
  onSignIn,
  testId = "account-grace-sign-in-prompt",
}: AccountGraceSignInPromptProps) {
  return (
    <div data-testid={testId}>
      <Empty
        action={
          <Button onClick={onSignIn} size="sm" type="button">
            <LogIn aria-hidden="true" />
            Sign in
          </Button>
        }
        description="Sign in to CompozyOS to manage computers, remote access, billing, and devices."
        fill={false}
        icon={ShieldAlert}
        title="Sign in to use this."
      />
    </div>
  );
}
