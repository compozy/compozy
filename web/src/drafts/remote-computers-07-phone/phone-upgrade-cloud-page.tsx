import { useState } from "react";
import { CheckCircle2, ExternalLink, Lock } from "@compozy/ui/icons";

import {
  Button,
  Logo,
  Pill,
  RadioCardGroup,
  Surface,
  toast,
  type RadioCardGroupOption,
} from "@compozy/ui";
import { GithubLogo, GoogleLogo } from "@compozy/ui/logos";

import type { UpgradeInterval } from "@/systems/account/components/account-upgrade-sheet";
import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import type {
  AccountProvider,
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ACCOUNT_PRO_PLAN_OPTIONS } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

export interface PhoneUpgradeCloudPageProps {
  computerName: string;
  hostname: string;
  ownerEmail: string;
  platform: ComputerPlatform;
  iconKind: ComputerIconKind;
  initialSignedInProvider?: AccountProvider | null;
}

/**
 * Cloud-hosted upgrade page opened from an expired hosted route's "Upgrade to Pro"
 * button (`https://cloud.compozy.com/billing/upgrade?hostname=<host>`, `_uiux.md` U8, Q5):
 * signs the owner in with Google or GitHub on Compozy Cloud, confirms ownership,
 * selects Pro (annual preselected), and redirects to Stripe Checkout.
 */
export function PhoneUpgradeCloudPage({
  computerName,
  hostname,
  ownerEmail,
  platform,
  iconKind,
  initialSignedInProvider = null,
}: PhoneUpgradeCloudPageProps) {
  const [provider, setProvider] = useState<AccountProvider | null>(initialSignedInProvider);
  const [interval, setInterval] = useState<UpgradeInterval>("year");

  const planOptions: RadioCardGroupOption<UpgradeInterval>[] = ACCOUNT_PRO_PLAN_OPTIONS.map(
    option => ({
      value: option.interval,
      title: option.title,
      badge: option.badge ? (
        <Pill size="xs" tone="brand">
          {option.badge}
        </Pill>
      ) : undefined,
      description: (
        <span className="mt-0.5 flex flex-col gap-0.5">
          <span className="text-body font-semibold text-fg-strong tabular-nums">
            {option.price}
          </span>
          <span className="text-eyebrow text-muted-foreground">{option.cadenceDetail}</span>
        </span>
      ),
      className: "p-3",
    })
  );

  const handleSelectProvider = (chosen: AccountProvider) => {
    setProvider(chosen);
    toast.success(`Signed in with ${chosen === "google" ? "Google" : "GitHub"} as ${ownerEmail}.`);
  };

  const handleCheckout = () => {
    const planLabel = interval === "year" ? "Pro annual" : "Pro monthly";
    toast(`Redirecting to Stripe Checkout (${planLabel}). Returns to https://${hostname}.`);
  };

  return (
    <div
      className="flex min-h-dvh w-full flex-1 flex-col overflow-y-auto bg-canvas text-fg"
      data-slot="phone-upgrade-cloud-page"
    >
      <div className="flex shrink-0 items-center justify-center border-b border-line-soft bg-sunken px-4 py-2">
        <div className="inline-flex max-w-full items-center gap-1.5 rounded-pill border border-line bg-canvas px-3 py-1 font-mono text-micro text-muted-foreground">
          <Lock aria-hidden="true" className="size-3 shrink-0 text-success" />
          <span className="truncate">cloud.compozy.com/billing/upgrade</span>
        </div>
      </div>

      <main className="flex flex-1 flex-col items-center justify-center px-4 py-6 sm:px-6 sm:py-10">
        <div className="flex w-full max-w-[440px] flex-col items-center gap-5">
          <div className="flex flex-col items-center gap-2.5">
            <Logo className="h-6.5" variant="logo" />
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
          </div>

          <Surface className="flex w-full flex-col gap-5 p-5 sm:p-6" variant="card">
            <div className="flex flex-col gap-1">
              <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                Upgrade to Compozy Pro
              </h1>
              <p className="text-small-body leading-relaxed text-muted-foreground">
                Sign in to confirm you own <span className="font-medium text-fg">{computerName}</span>
                , then choose a plan to restore its hosted route.
              </p>
            </div>

            <div className="flex flex-col gap-2.5">
              <span className="eyebrow text-subtle">1. Account</span>
              {provider ? (
                <div className="flex items-center justify-between gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5">
                  <div className="flex min-w-0 items-center gap-2">
                    <CheckCircle2 aria-hidden="true" className="size-4 shrink-0 text-success" />
                    <div className="min-w-0">
                      <p className="truncate text-small-body font-medium text-fg">{ownerEmail}</p>
                      <p className="text-eyebrow text-muted-foreground">
                        Signed in with {provider === "google" ? "Google" : "GitHub"} · Owner verified
                      </p>
                    </div>
                  </div>
                  <Button onClick={() => setProvider(null)} size="xs" type="button" variant="ghost">
                    Change
                  </Button>
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                  <Button
                    className="w-full"
                    onClick={() => handleSelectProvider("google")}
                    size="default"
                    type="button"
                    variant="secondary"
                  >
                    <GoogleLogo aria-hidden="true" className="size-4 shrink-0" />
                    Continue with Google
                  </Button>
                  <Button
                    className="w-full"
                    onClick={() => handleSelectProvider("github")}
                    size="default"
                    type="button"
                    variant="secondary"
                  >
                    <GithubLogo className="size-4 shrink-0" />
                    Continue with GitHub
                  </Button>
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
            </div>

            <div className="flex flex-col gap-2 pt-0.5">
              <Button
                className="w-full"
                disabled={provider === null}
                onClick={handleCheckout}
                size="cta-lg"
                type="button"
              >
                Continue to checkout
                <ExternalLink aria-hidden="true" className="size-3.5" />
              </Button>
              {provider ? (
                <p className="text-center text-eyebrow text-subtle">
                  Opens Stripe Checkout and returns to {hostname}.
                </p>
              ) : null}
            </div>
          </Surface>

          <p className="text-center text-eyebrow leading-relaxed text-faint">
            Nothing on {computerName} is unlocked until Pro is active.
          </p>
        </div>
      </main>
    </div>
  );
}
