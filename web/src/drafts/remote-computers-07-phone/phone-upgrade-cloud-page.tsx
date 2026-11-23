import { useState } from "react";
import { Lock } from "@compozy/ui/icons";

import { toast } from "@compozy/ui";

import type { UpgradeInterval } from "@/systems/account/components/account-upgrade-sheet";
import { CloudUpgradePage } from "@/systems/account/components/cloud-upgrade-page";
import type { CloudLegalLinkKind } from "@/systems/account/components/sign-in-provider-buttons";
import type {
  AccountProvider,
  ComputerIconKind,
  ComputerPlatform,
} from "@/systems/computers/mocks/remote-computers-fixtures";

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
 * wraps the reusable `CloudUpgradePage` inside the mobile browser address bar.
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

  const handleSelectProvider = (chosen: AccountProvider) => {
    setProvider(chosen);
    toast.success(`Signed in with ${chosen === "google" ? "Google" : "GitHub"} as ${ownerEmail}.`);
  };

  const handleCheckout = (chosenInterval: UpgradeInterval) => {
    const planLabel = chosenInterval === "year" ? "Pro annual" : "Pro monthly";
    toast(`Redirecting to Stripe Checkout (${planLabel}). Returns to https://${hostname}.`);
  };

  const handleOpenLegalLink = (kind: CloudLegalLinkKind) => {
    toast(
      kind === "terms"
        ? "Opened Terms"
        : kind === "privacy"
          ? "Opened Privacy Policy"
          : "Opened Refund Policy"
    );
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

      <CloudUpgradePage
        className="py-6 sm:py-10"
        computerName={computerName}
        hostname={hostname}
        iconKind={iconKind}
        interval={interval}
        onContinueToCheckout={handleCheckout}
        onIntervalChange={setInterval}
        onOpenLegalLink={handleOpenLegalLink}
        onSelectProvider={handleSelectProvider}
        onSignOut={() => setProvider(null)}
        ownerEmail={ownerEmail}
        platform={platform}
        signedInProvider={provider}
      />
    </div>
  );
}
