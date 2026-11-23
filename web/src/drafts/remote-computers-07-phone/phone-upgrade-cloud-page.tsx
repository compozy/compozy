import { useState } from "react";

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

function handleOpenLegalLink(kind: CloudLegalLinkKind) {
  toast.message(
    kind === "terms"
      ? "Opened Terms"
      : kind === "privacy"
        ? "Opened Privacy Policy"
        : "Opened Refund Policy"
  );
}

/**
 * Cloud-hosted upgrade page opened from an expired hosted route's "Upgrade to Pro"
 * button (`https://cloud.compozy.com/billing/upgrade?hostname=<host>`, `_uiux.md` U8, Q5):
 * the reusable `CloudUpgradePage`; the draft browser frame shows its address.
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
    toast.message(`Redirecting to Stripe Checkout (${planLabel}). Returns to https://${hostname}.`);
  };

  return (
    <div
      className="flex min-h-0 w-full flex-1 flex-col bg-canvas text-fg"
      data-slot="phone-upgrade-cloud-page"
    >
      <CloudUpgradePage
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
