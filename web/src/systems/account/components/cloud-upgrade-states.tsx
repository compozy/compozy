import {
  ArrowLeft,
  CheckCircle2,
  ClockAlert,
  Download,
  ExternalLink,
  RefreshCw,
  ShieldOff,
  WifiOff,
} from "@compozy/ui/icons";

import { Button, Pill, Spinner } from "@compozy/ui";

import type { UpgradeInterval } from "@/systems/account/components/account-upgrade-sheet";
import type { AccountProvider } from "@/systems/computers/mocks/remote-computers-fixtures";
import { ACCOUNT_PRO_PLAN_OPTIONS } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import {
  CLOUD_DOWNLOAD_URL,
  CLOUD_OPEN_APP_URL,
  CLOUD_STATUS_URL,
  CLOUD_SUPPORT_EMAIL,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import { providerDisplayName } from "../lib/provider-display-name";

function findPlanOption(interval: UpgradeInterval) {
  return (
    ACCOUNT_PRO_PLAN_OPTIONS.find(option => option.interval === interval) ??
    ACCOUNT_PRO_PLAN_OPTIONS[0]!
  );
}

export interface CloudUpgradeAccountStripProps {
  email: string;
  provider: AccountProvider;
  ownerVerified?: boolean;
  switchLabel: string;
  onSignOut?: () => void;
}

/**
 * Signed-in account identity inset rendered inside the upgrade card.
 */
export function CloudUpgradeAccountStrip({
  email,
  provider,
  ownerVerified = false,
  switchLabel,
  onSignOut,
}: CloudUpgradeAccountStripProps) {
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-md bg-sunken px-3.5 py-2.5"
      data-slot="cloud-upgrade-account-strip"
    >
      <div className="flex min-w-0 items-center gap-2.5">
        <CheckCircle2 aria-hidden="true" className="size-4 shrink-0 text-success" />
        <div className="min-w-0">
          <p className="truncate text-small-body font-medium text-fg">{email}</p>
          <p className="text-eyebrow text-muted-foreground">
            Signed in with {providerDisplayName(provider)}
            {ownerVerified ? " · Owner verified" : ""}
          </p>
        </div>
      </div>
      {onSignOut ? (
        <Button className="shrink-0" onClick={onSignOut} size="xs" type="button" variant="ghost">
          {switchLabel}
        </Button>
      ) : null}
    </div>
  );
}

export interface CloudUpgradeRedirectingViewProps {
  interval: UpgradeInterval;
  email: string;
  hostname?: string;
  onOpenStripe?: () => void;
  onBack?: () => void;
}

export function CloudUpgradeRedirectingView({
  interval,
  email,
  hostname,
  onOpenStripe,
  onBack,
}: CloudUpgradeRedirectingViewProps) {
  const plan = findPlanOption(interval);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-3 pt-1 text-center">
        <span
          aria-hidden="true"
          className="inline-flex size-11 items-center justify-center rounded-lg bg-surface-2 text-fg-2"
        >
          <Spinner className="size-5" />
        </span>
        <div className="flex flex-col gap-1">
          <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
            Opening Stripe Checkout
          </h1>
          <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
            {hostname
              ? `Preparing checkout for ${email}. After payment, you'll return to ${hostname}.`
              : `Preparing checkout for ${email}. You'll be redirected in a moment.`}
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-2 rounded-md bg-sunken px-3.5 py-3 text-small-body">
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">Plan</span>
          <span className="font-medium text-fg tabular-nums">
            {plan.title} · {plan.price}
          </span>
        </div>
        <div className="flex items-center justify-between gap-3">
          <span className="text-muted-foreground">Account</span>
          <span className="truncate font-medium text-fg">{email}</span>
        </div>
      </div>

      <div className="flex flex-col gap-2 border-t border-line-soft pt-4">
        <Button className="w-full" onClick={onOpenStripe} size="cta-lg" type="button">
          <span>Open Stripe Checkout</span>
          <ExternalLink aria-hidden="true" className="size-3.5" />
        </Button>
        {onBack ? (
          <Button className="w-full" onClick={onBack} size="default" type="button" variant="ghost">
            <ArrowLeft aria-hidden="true" className="size-3.5" />
            <span>Back to plans</span>
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export interface CloudUpgradeSuccessViewProps {
  interval: UpgradeInterval;
  email: string;
  onOpenApp?: () => void;
  onDownload?: () => void;
}

export function CloudUpgradeSuccessView({
  interval,
  email,
  onOpenApp,
  onDownload,
}: CloudUpgradeSuccessViewProps) {
  const plan = findPlanOption(interval);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-3 pt-1 text-center">
        <span
          aria-hidden="true"
          className="inline-flex size-11 items-center justify-center rounded-lg bg-success-tint text-success"
        >
          <CheckCircle2 className="size-5" />
        </span>
        <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
          You&apos;re on Pro. Open CompozyOS to use your computers.
        </h1>
      </div>

      <div className="flex flex-col divide-y divide-line-soft border-t border-line-soft">
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Account</span>
          <span className="truncate font-medium text-fg">{email}</span>
        </div>
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Plan</span>
          <span className="font-medium text-fg tabular-nums">
            {plan.title} · {plan.price}
          </span>
        </div>
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Status</span>
          <Pill form="plain" tone="success">
            <Pill.Dot />
            Active
          </Pill>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <Button
          className="w-full"
          nativeButton={false}
          render={
            <a
              aria-label="Open CompozyOS"
              href={CLOUD_OPEN_APP_URL}
              onClick={event => {
                if (onOpenApp) {
                  event.preventDefault();
                  onOpenApp();
                }
              }}
            />
          }
          size="cta-lg"
        >
          Open CompozyOS
        </Button>
        <Button
          className="w-full"
          nativeButton={false}
          render={
            <a
              aria-label="Download CompozyOS"
              href={CLOUD_DOWNLOAD_URL}
              onClick={event => {
                if (onDownload) {
                  event.preventDefault();
                  onDownload();
                }
              }}
              rel="noreferrer"
              target="_blank"
            />
          }
          size="cta-lg"
          variant="secondary"
        >
          <Download aria-hidden="true" className="size-4" />
          <span>Download CompozyOS</span>
        </Button>
      </div>

      <p className="text-center text-meta text-muted-foreground">
        Sign in with <span className="font-medium text-fg">{email}</span> in the app.
      </p>
    </div>
  );
}

export interface CloudUpgradeAlreadyProViewProps {
  email: string;
  provider: AccountProvider;
  interval: UpgradeInterval;
  renewsLabel?: string;
  hostedRoutesSummary?: string;
  onManageBilling?: () => void;
  onOpenApp?: () => void;
  onSignOut?: () => void;
}

export function CloudUpgradeAlreadyProView({
  email,
  provider,
  interval,
  renewsLabel = "Dec 22, 2026",
  hostedRoutesSummary = "3 of 10 used",
  onManageBilling,
  onOpenApp,
  onSignOut,
}: CloudUpgradeAlreadyProViewProps) {
  const plan = findPlanOption(interval);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-3 pt-1 text-center">
        <span
          aria-hidden="true"
          className="inline-flex size-11 items-center justify-center rounded-lg bg-brand-tint text-brand-strong"
        >
          <CheckCircle2 className="size-5" />
        </span>
        <div className="flex flex-col gap-1">
          <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
            You&apos;re already on Pro.
          </h1>
          <p className="text-small-body text-muted-foreground">
            Switch billing interval, update your payment method or view invoices in billing.
          </p>
        </div>
      </div>

      <CloudUpgradeAccountStrip
        email={email}
        onSignOut={onSignOut}
        provider={provider}
        switchLabel="Not you?"
      />

      <div className="flex flex-col divide-y divide-line-soft border-t border-line-soft">
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Current plan</span>
          <span className="font-medium text-fg tabular-nums">
            {plan.title} · {plan.price}
          </span>
        </div>
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Renews</span>
          <span className="font-medium text-fg">{renewsLabel}</span>
        </div>
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Remote computers</span>
          <span className="font-medium text-fg">Unlimited</span>
        </div>
        <div className="flex items-center justify-between gap-3 py-2.5 text-small-body">
          <span className="text-muted-foreground">Hosted routes</span>
          <span className="font-medium text-fg tabular-nums">{hostedRoutesSummary}</span>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <Button className="w-full" onClick={onManageBilling} size="cta-lg" type="button">
          <span>Manage billing</span>
          <ExternalLink aria-hidden="true" className="size-3.5" />
        </Button>
        {onOpenApp ? (
          <Button
            className="w-full"
            nativeButton={false}
            render={
              <a
                aria-label="Open CompozyOS"
                href={CLOUD_OPEN_APP_URL}
                onClick={event => {
                  event.preventDefault();
                  onOpenApp();
                }}
              />
            }
            size="default"
            variant="ghost"
          >
            Open CompozyOS
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export interface CloudUpgradeUnavailableViewProps {
  onTryAgain?: () => void;
  onOpenStatus?: () => void;
}

export function CloudUpgradeUnavailableView({
  onTryAgain,
  onOpenStatus,
}: CloudUpgradeUnavailableViewProps) {
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-3 pt-1 text-center">
        <span
          aria-hidden="true"
          className="inline-flex size-11 items-center justify-center rounded-lg bg-warning-tint text-warning"
        >
          <WifiOff className="size-5" />
        </span>
        <div className="flex flex-col gap-1.5">
          <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
            Can&apos;t reach Compozy Cloud
          </h1>
          <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
            Local work is unaffected. Try again in a moment, or check the status page for updates.
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <Button className="w-full" onClick={onTryAgain} size="cta-lg" type="button">
          <RefreshCw aria-hidden="true" className="size-3.5" />
          <span>Try again</span>
        </Button>
        <Button
          className="w-full"
          onClick={() => {
            if (onOpenStatus) {
              onOpenStatus();
            } else if (typeof window !== "undefined") {
              window.open(CLOUD_STATUS_URL, "_blank", "noreferrer");
            }
          }}
          size="cta-lg"
          type="button"
          variant="secondary"
        >
          <span>Status</span>
          <ExternalLink aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

export interface CloudUpgradeBlockedAccountViewProps {
  kind: "suspended" | "deleting";
  email?: string;
  provider?: AccountProvider | null;
  onSignOut?: () => void;
}

export function CloudUpgradeBlockedAccountView({
  kind,
  email,
  provider,
  onSignOut,
}: CloudUpgradeBlockedAccountViewProps) {
  const isSuspended = kind === "suspended";

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-3 pt-1 text-center">
        <span
          aria-hidden="true"
          className={
            isSuspended
              ? "inline-flex size-11 items-center justify-center rounded-lg bg-destructive-tint text-destructive"
              : "inline-flex size-11 items-center justify-center rounded-lg bg-warning-tint text-warning"
          }
        >
          {isSuspended ? <ShieldOff className="size-5" /> : <ClockAlert className="size-5" />}
        </span>
        <div className="flex flex-col gap-1.5">
          <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong text-balance">
            {isSuspended ? "This account is suspended" : "Your account is being deleted"}
          </h1>
          <p className="text-small-body leading-relaxed text-muted-foreground text-pretty">
            {isSuspended ? (
              <>
                Contact{" "}
                <a
                  className="font-medium text-fg underline decoration-line-strong underline-offset-3 transition-colors hover:text-fg-strong"
                  href={`mailto:${CLOUD_SUPPORT_EMAIL}`}
                >
                  {CLOUD_SUPPORT_EMAIL}
                </a>
                .
              </>
            ) : (
              "Try again in a few minutes."
            )}
          </p>
        </div>
      </div>

      {email && provider ? (
        <CloudUpgradeAccountStrip
          email={email}
          onSignOut={onSignOut}
          provider={provider}
          switchLabel="Not you?"
        />
      ) : null}
    </div>
  );
}
