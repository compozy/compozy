import { useState } from "react";
import { definePrototype, useSearch, type PrototypeProps } from "@compozy/proto";

import { toast } from "@compozy/ui";

import type { UpgradeInterval } from "@/systems/account/components/account-upgrade-sheet";
import {
  CloudUpgradePage,
  type CloudUpgradeStatus,
} from "@/systems/account/components/cloud-upgrade-page";
import type { CloudLegalLinkKind } from "@/systems/account/components/sign-in-provider-buttons";
import {
  ACCOUNT_PRESETS,
  OWNER_EMAIL,
  type AccountProvider,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { DraftBrowserFrame } from "../remote-computers-shared/browser-frame";

export const meta = definePrototype({
  title: "Buy Pro",
  spec: "remote-computers",
  surface: "U12",
  route: "/billing/upgrade",
  states: [
    { id: "signed-out", label: "Signed out · Sign in first, checkout disabled" },
    { id: "signed-in-annual", label: "Signed in · Annual plan preselected" },
    { id: "signed-in-monthly", label: "Signed in · Monthly plan preselected" },
    { id: "promo-code", label: "Promotion code · Entered at Stripe Checkout" },
    { id: "redirecting", label: "Redirecting · Opening Stripe Checkout" },
    { id: "success", label: "Success · You're on Pro, open or download CompozyOS" },
    { id: "already-pro", label: "Already Pro · Active plan and Manage billing" },
    { id: "checkout-cancelled", label: "Checkout cancelled · Nothing was charged" },
    { id: "rate-limited", label: "Rate limited · Retry wait and status page link" },
    { id: "cloud-unavailable", label: "Cloud unavailable · Try again and status link" },
    { id: "suspended", label: "Account suspended · Support contact, no checkout" },
  ],
});

interface PurchaseStatePreset {
  status: CloudUpgradeStatus;
  provider: AccountProvider | null;
  interval: UpgradeInterval;
  browserAddress: string;
  checkoutCancelled?: boolean;
  promoNoteOpen?: boolean;
  retryAfterSeconds?: number;
}

const PURCHASE_PRESETS: Record<string, PurchaseStatePreset> = {
  "signed-out": {
    status: "ready",
    provider: null,
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade",
  },
  "signed-in-annual": {
    status: "ready",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade?interval=year",
  },
  "signed-in-monthly": {
    status: "ready",
    provider: "google",
    interval: "month",
    browserAddress: "cloud.compozy.com/billing/upgrade?interval=month",
  },
  "promo-code": {
    status: "ready",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade?interval=year",
    promoNoteOpen: true,
  },
  redirecting: {
    status: "redirecting",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade/checkout",
  },
  success: {
    status: "success",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade/success",
  },
  "already-pro": {
    status: "already_pro",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade",
  },
  "checkout-cancelled": {
    status: "ready",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade?checkout=cancelled",
    checkoutCancelled: true,
  },
  "rate-limited": {
    status: "rate_limited",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade",
    retryAfterSeconds: 45,
  },
  "cloud-unavailable": {
    status: "cloud_unavailable",
    provider: null,
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade",
  },
  suspended: {
    status: "suspended",
    provider: "google",
    interval: "year",
    browserAddress: "cloud.compozy.com/billing/upgrade",
  },
};

/**
 * Browser window chrome matching Screen 00 (`remote-computers-00-sign-in`),
 * adapted so 390 px phone viewports keep the address pill centered without clipping.
 */
function resolveIntervalParam(search: string): UpgradeInterval | null {
  const raw = new URLSearchParams(search).get("interval");
  if (raw === "month" || raw === "year") return raw;
  return null;
}

function handleOpenPurchaseLegalLink(kind: CloudLegalLinkKind) {
  toast.message(
    kind === "terms"
      ? "Opened Terms (compozy.com/legal/terms)"
      : kind === "privacy"
        ? "Opened Privacy Policy (compozy.com/legal/privacy)"
        : "Opened Refund Policy (compozy.com/legal/refunds)"
  );
}

function InteractiveCloudPurchase({
  preset,
  queryInterval,
}: {
  preset: PurchaseStatePreset;
  queryInterval: UpgradeInterval | null;
}) {
  const initialStatus = preset.status;
  const initialProvider = preset.provider;
  const initialAddress = preset.browserAddress;
  const initialInterval =
    preset.browserAddress.includes("interval=month") ||
    preset.browserAddress.includes("interval=year")
      ? preset.interval
      : (queryInterval ?? preset.interval);

  const [status, setStatus] = useState<CloudUpgradeStatus>(initialStatus);
  const [provider, setProvider] = useState<AccountProvider | null>(initialProvider);
  const [interval, setInterval] = useState<UpgradeInterval>(initialInterval);
  const [checkoutCancelled, setCheckoutCancelled] = useState(Boolean(preset.checkoutCancelled));
  const [address, setAddress] = useState(initialAddress);

  const proPreset = ACCOUNT_PRESETS.pro;
  const hostedRoutesSummary = `${proPreset.cloudRoutesUsed} of ${proPreset.cloudRoutesLimit} used`;

  const handleSelectProvider = (chosen: AccountProvider) => {
    setProvider(chosen);
    setStatus("ready");
    setAddress(`cloud.compozy.com/billing/upgrade?interval=${interval}`);
    toast.success(`Signed in with ${chosen === "google" ? "Google" : "GitHub"} as ${OWNER_EMAIL}.`);
  };

  const handleSignOut = () => {
    setProvider(null);
    setCheckoutCancelled(false);
    setStatus("ready");
    setAddress("cloud.compozy.com/billing/upgrade");
    toast.message("Signed out of Compozy Cloud");
  };

  const handleIntervalChange = (next: UpgradeInterval) => {
    setInterval(next);
    if (status === "ready" && provider) {
      setAddress(`cloud.compozy.com/billing/upgrade?interval=${next}`);
    }
  };

  const handleContinueToCheckout = (chosenInterval: UpgradeInterval) => {
    setCheckoutCancelled(false);
    setStatus("redirecting");
    setAddress("cloud.compozy.com/billing/upgrade/checkout");
    const planLabel = chosenInterval === "year" ? "Pro annual" : "Pro monthly";
    toast.message(`Creating Stripe Checkout session for ${planLabel}…`);
  };

  const handleOpenStripe = () => {
    setStatus("success");
    setAddress("cloud.compozy.com/billing/upgrade/success");
    toast.success("Checkout completed. You're on Pro.");
  };

  const handleBackToPlans = () => {
    setStatus("ready");
    setCheckoutCancelled(true);
    setAddress("cloud.compozy.com/billing/upgrade?checkout=cancelled");
  };

  const handleTryAgain = () => {
    setStatus("ready");
    setAddress("cloud.compozy.com/billing/upgrade");
    toast.success("Connected to Compozy Cloud");
  };

  return (
    <DraftBrowserFrame address={address} dataSlot="cloud-purchase-browser-window">
      <CloudUpgradePage
        checkoutCancelled={checkoutCancelled}
        hostedRoutesSummary={hostedRoutesSummary}
        initialPromoNoteOpen={Boolean(preset.promoNoteOpen)}
        interval={interval}
        onBackToPlans={handleBackToPlans}
        onContinueToCheckout={handleContinueToCheckout}
        onDownload={() => toast.message("Opened compozy.com/download")}
        onIntervalChange={handleIntervalChange}
        onManageBilling={() => toast.message("Opening Stripe Customer Portal…")}
        onOpenApp={() => toast.message("Opening CompozyOS…")}
        onOpenLegalLink={handleOpenPurchaseLegalLink}
        onOpenStatus={() => toast.message("Opened status.compozy.com")}
        onOpenStripe={handleOpenStripe}
        onSelectProvider={handleSelectProvider}
        onSignOut={handleSignOut}
        onTryAgain={handleTryAgain}
        ownerEmail={OWNER_EMAIL}
        renewsLabel="Dec 22, 2026"
        retryAfterSeconds={preset.retryAfterSeconds}
        signedInProvider={provider}
        status={status}
      />
    </DraftBrowserFrame>
  );
}

export default function RemoteComputersCloudPurchaseDraft({ state }: PrototypeProps) {
  const search = useSearch();
  const queryInterval = resolveIntervalParam(search);
  const preset = PURCHASE_PRESETS[state] ?? PURCHASE_PRESETS["signed-out"]!;

  return <InteractiveCloudPurchase key={state} preset={preset} queryInterval={queryInterval} />;
}
