import { useState, type ReactNode } from "react";
import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { ChevronLeft, ChevronRight, Lock } from "@compozy/ui/icons";

import { toast } from "@compozy/ui";

import { SignInCloudPage } from "@/systems/account/components/sign-in-cloud-page";
import { SignInScreen } from "@/systems/account/components/sign-in-screen";
import {
  buildWaitingAttempt,
  SIGN_IN_CLOUD_PRESETS,
  SIGN_IN_DESKTOP_PRESETS,
  type CloudPageKind,
  type CloudPagePreset,
  type SignInNoticeFixture,
  type SignInProvider,
  type SignInScreenPreset,
  type SignInWaitingAttemptFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

export const meta = definePrototype({
  title: "Sign in",
  spec: "remote-computers",
  surface: "U0",
  route: "/sign-in",
  states: [
    { id: "first-run", label: "First run · Sign in to CompozyOS" },
    { id: "existing-install", label: "Existing install · After updating, nothing lost" },
    { id: "waiting-browser", label: "Waiting for browser · Google code WDJB-MJHT" },
    { id: "cancelled", label: "Cancelled · Sign-in was cancelled" },
    { id: "cloud-unreachable", label: "Cloud unreachable · Retry with buttons disabled" },
    { id: "reauth-device-removed", label: "Signed out · Computer removed from account" },
    { id: "timed-out", label: "Timed out · Sign-in timed out" },
    { id: "account-token-revoked", label: "Account token revoked · CLI hint, no buttons" },
    { id: "account-deleting", label: "Account deleting · Try again in a few minutes" },
    { id: "cloud-activate", label: "Cloud browser · Activate MacBook Pro with code" },
    { id: "cloud-signed-in", label: "Cloud browser · Signed in, return to CompozyOS" },
    { id: "cloud-email-unverified", label: "Cloud browser · GitHub email not verified" },
    { id: "cloud-code-used", label: "Cloud browser · Code already used" },
    { id: "cloud-unsubscribe", label: "Cloud browser · Unsubscribe confirmation" },
  ],
});

/**
 * Frameless desktop window chrome matching boot (`desktop-boot`):
 * full-viewport canvas with quiet top-left traffic lights, no rail, no sidebar, no palette.
 */
function DesktopAppWindowFrame({ children }: { children: ReactNode }) {
  return (
    <div
      className="flex h-dvh min-h-dvh w-full flex-col overflow-y-auto bg-canvas text-fg"
      data-slot="sign-in-desktop-window"
    >
      <div className="flex h-11 shrink-0 items-center px-4 select-none" aria-hidden="true">
        <div className="flex items-center gap-2">
          <span className="size-3 rounded-full bg-line-strong" />
          <span className="size-3 rounded-full bg-line-strong" />
          <span className="size-3 rounded-full bg-line-strong" />
        </div>
      </div>
      {children}
    </div>
  );
}

/**
 * Minimal browser window chrome so `cloud.compozy.com` pages clearly read as
 * external browser pages rather than inside the desktop app.
 */
function BrowserWindowFrame({ address, children }: { address: string; children: ReactNode }) {
  return (
    <div
      className="flex h-dvh min-h-dvh w-full flex-col overflow-hidden bg-canvas text-fg"
      data-slot="sign-in-browser-window"
    >
      <div
        aria-label="Browser address bar"
        className="grid h-12 shrink-0 grid-cols-[1fr_minmax(0,440px)_1fr] items-center gap-3 border-b border-line bg-rail px-4 select-none"
      >
        <div aria-hidden="true" className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <span className="size-3 rounded-full bg-line-strong" />
            <span className="size-3 rounded-full bg-line-strong" />
            <span className="size-3 rounded-full bg-line-strong" />
          </div>
          <div className="hidden items-center gap-1 text-faint sm:flex">
            <ChevronLeft className="size-4" />
            <ChevronRight className="size-4 opacity-50" />
          </div>
        </div>

        <div className="flex h-7.5 min-w-0 items-center justify-center gap-1.5 rounded-pill border border-line-soft bg-canvas px-3.5 text-eyebrow text-muted-foreground">
          <Lock aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          <span className="truncate font-mono text-mono-id">{address}</span>
        </div>

        <div aria-hidden="true" />
      </div>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">{children}</div>
    </div>
  );
}

function handleReopenBrowser(attempt: SignInWaitingAttemptFixture) {
  toast.message(`Reopened ${attempt.verificationUri} in your browser`);
}

function handleOpenDesktopLegalLink(kind: "terms" | "privacy") {
  toast.message(
    kind === "terms" ? "Opened Terms in your browser" : "Opened Privacy Policy in your browser"
  );
}

function handleOpenCloudLegalLink(linkKind: "terms" | "privacy" | "refunds") {
  toast.message(
    linkKind === "terms"
      ? "Opened Terms"
      : linkKind === "privacy"
        ? "Opened Privacy Policy"
        : "Opened Refund Policy"
  );
}

function InteractiveDesktopSignIn({ preset }: { preset: SignInScreenPreset }) {
  const [waiting, setWaiting] = useState<SignInWaitingAttemptFixture | undefined>(preset.waiting);
  const [notice, setNotice] = useState<SignInNoticeFixture | undefined>(preset.notice);
  const [buttonsDisabled, setButtonsDisabled] = useState(Boolean(preset.buttonsDisabled));
  const [marketingOptIn, setMarketingOptIn] = useState(Boolean(preset.initialMarketingOptIn));

  const handleSelectProvider = (provider: SignInProvider) => {
    const attempt = buildWaitingAttempt(provider);
    setNotice(undefined);
    setWaiting(attempt);
    toast.message(`Opened ${attempt.verificationUri} in your browser`);
  };

  const handleCancelWaiting = () => {
    setWaiting(undefined);
    setNotice({ kind: "cancelled", message: "Sign-in was cancelled." });
  };

  const handleRetryConnection = () => {
    setNotice(undefined);
    setButtonsDisabled(false);
    toast.success("Connected to Compozy Cloud");
  };

  return (
    <DesktopAppWindowFrame>
      <SignInScreen
        buttonsDisabled={buttonsDisabled}
        cliHintCommand={preset.cliHintCommand}
        marketingOptIn={marketingOptIn}
        notice={notice}
        onCancelWaiting={handleCancelWaiting}
        onMarketingOptInChange={setMarketingOptIn}
        onOpenLegalLink={handleOpenDesktopLegalLink}
        onReopenBrowser={handleReopenBrowser}
        onRetryConnection={handleRetryConnection}
        onSelectProvider={handleSelectProvider}
        subtitle={preset.subtitle}
        title={preset.title}
        tokenRevoked={preset.tokenRevoked}
        waiting={waiting}
      />
    </DesktopAppWindowFrame>
  );
}

function InteractiveCloudPage({ preset }: { preset: CloudPagePreset }) {
  const initialKind = preset.kind;
  const initialAddress = preset.browserAddress;
  const [kind, setKind] = useState<CloudPageKind>(initialKind);
  const [address, setAddress] = useState(initialAddress);
  const [unsubscribed, setUnsubscribed] = useState(Boolean(preset.initialUnsubscribed));

  const handleContinueProvider = (provider: SignInProvider) => {
    setKind("signed_in");
    setAddress("cloud.compozy.com/activate/done");
    toast.success(`Signed in with ${provider === "google" ? "Google" : "GitHub"}`);
  };

  const handleConfirmUnsubscribe = () => {
    setUnsubscribed(true);
    toast.success("Unsubscribed from product news and offers");
  };

  return (
    <BrowserWindowFrame address={address}>
      <SignInCloudPage
        accountEmail={preset.accountEmail}
        installationName={preset.installationName}
        kind={kind}
        onConfirmUnsubscribe={handleConfirmUnsubscribe}
        onContinueProvider={handleContinueProvider}
        onOpenLegalLink={handleOpenCloudLegalLink}
        provider={preset.provider}
        unsubscribed={unsubscribed}
        userCode={preset.userCode}
      />
    </BrowserWindowFrame>
  );
}

export default function RemoteComputersSignInDraft({ state }: PrototypeProps) {
  const cloudPreset = SIGN_IN_CLOUD_PRESETS[state];
  if (cloudPreset) {
    return <InteractiveCloudPage key={state} preset={cloudPreset} />;
  }

  const desktopPreset = SIGN_IN_DESKTOP_PRESETS[state] ?? SIGN_IN_DESKTOP_PRESETS["first-run"]!;
  return <InteractiveDesktopSignIn key={state} preset={desktopPreset} />;
}
