import { useState } from "react";
import { AlertTriangle, CircleAlert, ExternalLink } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  Disclosure,
  Pill,
  RadioCardGroup,
  type RadioCardGroupOption,
} from "@compozy/ui";

import type { UpgradeInterval } from "@/systems/account/components/account-upgrade-sheet";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import type {
  AccountProvider,
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ACCOUNT_PRO_PLAN_OPTIONS } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import {
  CLOUD_STATUS_URL,
  SIGN_IN_PRIVACY_URL,
  SIGN_IN_TERMS_URL,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

import {
  CloudUpgradeAccountStrip,
  CloudUpgradeAlreadyProView,
  CloudUpgradeBlockedAccountView,
  CloudUpgradeRedirectingView,
  CloudUpgradeSuccessView,
  CloudUpgradeUnavailableView,
} from "./cloud-upgrade-states";
import {
  CloudPageChrome,
  SignInProviderButton,
  type CloudLegalLinkKind,
} from "./sign-in-provider-buttons";

export type CloudUpgradeStatus =
  | "ready"
  | "redirecting"
  | "success"
  | "already_pro"
  | "rate_limited"
  | "cloud_unavailable"
  | "suspended"
  | "deleting";

/** Art-panel line per status: the offer only while buying, a neutral line when blocked. */
const STAGE_STATEMENT: Record<CloudUpgradeStatus, string> = {
  ready: "Free on your computer. Pro across all of them.",
  redirecting: "Free on your computer. Pro across all of them.",
  success: "Your computers, reachable from anywhere.",
  already_pro: "Your computers, reachable from anywhere.",
  rate_limited: "Your agents, on all your computers.",
  cloud_unavailable: "Your agents, on all your computers.",
  suspended: "Your agents, on all your computers.",
  deleting: "Your agents, on all your computers.",
};

export interface CloudUpgradePageProps {
  /** Present on the hosted-route phone flow (`?hostname=<host>`, U8); omitted on `/billing/upgrade` (U12). */
  hostname?: string;
  computerName?: string;
  platform?: ComputerPlatform;
  iconKind?: ComputerIconKind;
  ownerEmail: string;
  status?: CloudUpgradeStatus;
  signedInProvider?: AccountProvider | null;
  interval?: UpgradeInterval;
  /** Shown after returning from a cancelled Stripe Checkout session (`checkout=cancelled`). */
  checkoutCancelled?: boolean;
  /** Opens the inline promotion-code note explaining that codes are entered in Stripe Checkout. */
  initialPromoNoteOpen?: boolean;
  /** Retry-After countdown in seconds when `status === "rate_limited"`. */
  retryAfterSeconds?: number;
  renewsLabel?: string;
  hostedRoutesSummary?: string;
  onSelectProvider?: (provider: AccountProvider) => void;
  onSignOut?: () => void;
  onIntervalChange?: (interval: UpgradeInterval) => void;
  onContinueToCheckout?: (interval: UpgradeInterval) => void;
  onOpenStripe?: () => void;
  onBackToPlans?: () => void;
  onManageBilling?: () => void;
  onOpenApp?: () => void;
  onDownload?: () => void;
  onTryAgain?: () => void;
  onOpenStatus?: () => void;
  onOpenLegalLink?: (kind: CloudLegalLinkKind) => void;
  className?: string;
}

/**
 * Reusable `cloud.compozy.com/billing/upgrade` page shared by:
 * - U8 (`?hostname=<host>`): phone/browser flow from an expired hosted route
 * - U12 (no `hostname`): standalone web purchase flow from `compozy.com/pricing`
 */
export function CloudUpgradePage({
  hostname,
  computerName,
  platform,
  iconKind,
  ownerEmail,
  status = "ready",
  signedInProvider = null,
  interval: controlledInterval,
  checkoutCancelled = false,
  initialPromoNoteOpen = false,
  retryAfterSeconds = 45,
  renewsLabel,
  hostedRoutesSummary,
  onSelectProvider,
  onSignOut,
  onIntervalChange,
  onContinueToCheckout,
  onOpenStripe,
  onBackToPlans,
  onManageBilling,
  onOpenApp,
  onDownload,
  onTryAgain,
  onOpenStatus,
  onOpenLegalLink,
  className,
}: CloudUpgradePageProps) {
  const [uncontrolledInterval, setUncontrolledInterval] = useState<UpgradeInterval>(
    controlledInterval ?? "year"
  );
  const [promoOpen, setPromoOpen] = useState(initialPromoNoteOpen);

  const interval = controlledInterval ?? uncontrolledInterval;
  const setInterval = (next: UpgradeInterval) => {
    setUncontrolledInterval(next);
    onIntervalChange?.(next);
  };

  const isHostedFlow = Boolean(hostname);
  const isRateLimited = status === "rate_limited";
  const checkoutDisabled = signedInProvider === null || isRateLimited;

  const planOptions: RadioCardGroupOption<UpgradeInterval>[] = ACCOUNT_PRO_PLAN_OPTIONS.map(
    option => {
      const isAnnual = option.interval === "year";
      const badge = isAnnual ? "Save 33%" : option.badge;
      const cadenceDetail = isAnnual ? "USD 6.67/month, billed yearly" : option.cadenceDetail;

      return {
        value: option.interval,
        title: option.title,
        badge: badge ? (
          <Pill size="xs" tone="brand">
            {badge}
          </Pill>
        ) : undefined,
        description: (
          <span className="mt-0.5 flex flex-col gap-0.5">
            <span className="text-body font-semibold text-fg-strong tabular-nums">
              {option.price}
            </span>
            <span className="text-eyebrow text-muted-foreground">{cadenceDetail}</span>
          </span>
        ),
        className: "p-3",
      };
    }
  );

  const headerAccessory =
    hostname && computerName && platform && iconKind ? (
      <div className="inline-flex max-w-full items-center gap-1.5 rounded-pill border border-line bg-sunken px-2.5 py-1 text-eyebrow text-muted-foreground">
        <ComputerPlatformIcon
          className="size-4 bg-transparent"
          iconKind={iconKind}
          platform={platform}
          size="sm"
        />
        <span className="truncate font-medium text-fg">{computerName}</span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span className="truncate font-mono text-mono-id text-subtle">{hostname}</span>
      </div>
    ) : null;

  const belowCard =
    isHostedFlow && computerName && status === "ready" ? (
      <p className="text-center text-eyebrow leading-relaxed text-faint">
        Nothing on {computerName} is unlocked until Pro is active.
      </p>
    ) : null;

  const renderLegalInlineLink = (kind: "terms" | "privacy", label: string) => {
    const href = kind === "terms" ? SIGN_IN_TERMS_URL : SIGN_IN_PRIVACY_URL;
    return (
      <a
        className="text-fg-2 underline decoration-line-strong underline-offset-3 transition-colors hover:text-fg"
        href={href}
        onClick={event => {
          if (onOpenLegalLink) {
            event.preventDefault();
            onOpenLegalLink(kind);
          }
        }}
        rel="noreferrer"
        target="_blank"
      >
        {label}
      </a>
    );
  };

  return (
    <CloudPageChrome
      belowCard={belowCard}
      cardClassName="p-5 sm:p-6"
      className={className}
      dataCloudKind={`billing-${status}`}
      dataSlot="cloud-upgrade-page"
      headerAccessory={headerAccessory}
      maxWidthClass="max-w-[440px]"
      onOpenLegalLink={onOpenLegalLink}
      statement={STAGE_STATEMENT[status]}
      visual={status === "success" ? "converge" : "cone"}
    >
      {status === "redirecting" ? (
        <CloudUpgradeRedirectingView
          email={ownerEmail}
          hostname={hostname}
          interval={interval}
          onBack={onBackToPlans}
          onOpenStripe={onOpenStripe}
        />
      ) : null}

      {status === "success" ? (
        <CloudUpgradeSuccessView
          email={ownerEmail}
          interval={interval}
          onDownload={onDownload}
          onOpenApp={onOpenApp}
        />
      ) : null}

      {status === "already_pro" && signedInProvider ? (
        <CloudUpgradeAlreadyProView
          email={ownerEmail}
          hostedRoutesSummary={hostedRoutesSummary}
          interval={interval}
          onManageBilling={onManageBilling}
          onOpenApp={onOpenApp}
          onSignOut={onSignOut}
          provider={signedInProvider}
          renewsLabel={renewsLabel}
        />
      ) : null}

      {status === "cloud_unavailable" ? (
        <CloudUpgradeUnavailableView onOpenStatus={onOpenStatus} onTryAgain={onTryAgain} />
      ) : null}

      {status === "suspended" || status === "deleting" ? (
        <CloudUpgradeBlockedAccountView
          email={ownerEmail}
          kind={status}
          onSignOut={onSignOut}
          provider={signedInProvider}
        />
      ) : null}

      {status === "ready" || status === "rate_limited" ? (
        <>
          <div className="flex flex-col gap-1">
            <h1 className="text-detail-h1 font-medium tracking-detail-h1 text-fg-strong">
              {isHostedFlow ? "Upgrade to Compozy Pro" : "Get Compozy Pro"}
            </h1>
            <p className="text-small-body leading-relaxed text-muted-foreground">
              {isHostedFlow && computerName ? (
                <>
                  Sign in to confirm you own{" "}
                  <span className="font-medium text-fg">{computerName}</span>, then choose a plan to
                  restore its hosted route.
                </>
              ) : (
                "Unlimited remote computers, hosted routes, phone access and hosted webhooks."
              )}
            </p>
          </div>

          {checkoutCancelled ? (
            <Alert variant="default">
              <CircleAlert aria-hidden="true" />
              <AlertDescription className="text-fg">
                Checkout was cancelled. Nothing was charged.
              </AlertDescription>
            </Alert>
          ) : null}

          {isRateLimited ? (
            <Alert variant="warning">
              <AlertTriangle aria-hidden="true" />
              <AlertDescription className="text-fg">
                Too many requests. Try again in {retryAfterSeconds} seconds.
              </AlertDescription>
              <AlertActions className="justify-start pt-0.5">
                <Button
                  onClick={() => {
                    if (onOpenStatus) {
                      onOpenStatus();
                    } else if (typeof window !== "undefined") {
                      window.open(CLOUD_STATUS_URL, "_blank", "noreferrer");
                    }
                  }}
                  size="xs"
                  type="button"
                  variant="secondary"
                >
                  <span>Status</span>
                  <ExternalLink aria-hidden="true" className="size-3" />
                </Button>
              </AlertActions>
            </Alert>
          ) : null}

          <div className="flex flex-col gap-2.5">
            <span className="eyebrow text-subtle">1. Account</span>
            {signedInProvider ? (
              <CloudUpgradeAccountStrip
                email={ownerEmail}
                onSignOut={onSignOut}
                ownerVerified={isHostedFlow}
                provider={signedInProvider}
                switchLabel={isHostedFlow ? "Change" : "Not you?"}
              />
            ) : (
              <div className="flex flex-col gap-2.5">
                <div className="flex flex-col gap-2">
                  <SignInProviderButton
                    disabled={isRateLimited}
                    onClick={() => onSelectProvider?.("google")}
                    provider="google"
                  />
                  <SignInProviderButton
                    disabled={isRateLimited}
                    onClick={() => onSelectProvider?.("github")}
                    provider="github"
                  />
                </div>
                <p className="text-center text-meta leading-relaxed text-subtle">
                  By continuing, you agree to the {renderLegalInlineLink("terms", "Terms")} and{" "}
                  {renderLegalInlineLink("privacy", "Privacy Policy")}.
                </p>
              </div>
            )}
          </div>

          <div className="flex flex-col gap-2.5 border-t border-line-soft pt-4">
            <span className="eyebrow text-subtle">2. Plan</span>
            <RadioCardGroup
              aria-label="Choose a Pro billing period"
              className="grid-cols-1 gap-2"
              onValueChange={setInterval}
              options={planOptions}
              value={interval}
            />
            <p className="text-eyebrow text-muted-foreground">
              Prices in USD. Plus applicable taxes, calculated at checkout.
            </p>
            {!isHostedFlow ? (
              <Disclosure
                label="Have a promotion code?"
                onOpenChange={setPromoOpen}
                open={promoOpen}
                size="sm"
                variant="inline"
              >
                <p className="rounded-md bg-sunken px-3 py-2 text-meta leading-relaxed text-muted-foreground">
                  Promotion and migration codes are entered on the next screen in Stripe Checkout.
                  Select &ldquo;Add promotion code&rdquo; before paying.
                </p>
              </Disclosure>
            ) : null}
          </div>

          <div className="flex flex-col gap-2 pt-0.5">
            <Button
              className="w-full"
              disabled={checkoutDisabled}
              onClick={() => onContinueToCheckout?.(interval)}
              size="cta-lg"
              type="button"
            >
              <span>Continue to checkout</span>
              <ExternalLink aria-hidden="true" className="size-3.5" />
            </Button>
            {signedInProvider === null ? (
              <p className="text-center text-eyebrow text-subtle">
                Sign in above to continue to checkout.
              </p>
            ) : isHostedFlow && hostname ? (
              <p className="text-center text-eyebrow text-subtle">
                Opens Stripe Checkout and returns to {hostname}.
              </p>
            ) : null}
          </div>
        </>
      ) : null}
    </CloudPageChrome>
  );
}
