import { useRef } from "react";
import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { toast } from "@compozy/ui";

import {
  BrowserSignInGate,
  type BrowserSignInGateKind,
} from "@/systems/computers/components/browser-sign-in-gate";
import {
  PHONE_DEVICE,
  PHONE_FULL_SCOPES,
  PHONE_GRACE_DAYS_LEFT,
  PHONE_HOSTED_URL,
  PHONE_NARROW_SCOPES,
  PHONE_OTHER_ACCOUNT_EMAIL,
  PHONE_OWNER_EMAIL,
  PHONE_PEERS_MULTI,
  PHONE_PEERS_SINGLE,
  PHONE_TARGET_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";

import { DraftBrowserFrame } from "../remote-computers-shared";
import { HostedWebApp } from "./hosted-web-app";
import { PhoneApp } from "./phone-app";
import { useIsWideBrowser } from "./phone-frame";
import { PhoneUpgradeCloudPage } from "./phone-upgrade-cloud-page";

export const meta = definePrototype({
  title: "Hosted route in a browser",
  spec: "remote-computers",
  surface: "U8",
  route: "https://<slug>.compozy.run",
  states: [
    { id: "signed-in-sessions", label: "Signed in · Sessions on studio (web app, phone layout)" },
    {
      id: "signed-in-session-live",
      label: "Session open · Streaming transcript, approval & composer",
    },
    { id: "computer-menu", label: "Computer menu · Hosted peers & Sign out" },
    { id: "computer-menu-only-host", label: "Computer menu · Only this computer reachable" },
    { id: "offline-reconnecting", label: "Offline · Reconnecting with read-only transcript" },
    { id: "signed-in-no-terminal-scope", label: "Narrowed permissions · Terminal hidden" },
    { id: "sign-in", label: "Sign in · Google or GitHub hosted-route gate" },
    { id: "waiting-approval", label: "Waiting for approval · Fingerprint & auto-polling" },
    { id: "approval-expired", label: "Approval expired · Request approval again" },
    { id: "not-approved", label: "Not approved · Denied on studio" },
    { id: "another-account", label: "Another account · Signed-in email mismatch" },
    { id: "needs-pro", label: "Hosted route needs Pro · 7-day plan-expired gate" },
    {
      id: "upgrade-cloud-page",
      label: "Upgrade on cloud · Sign in & plan choice on cloud.compozy.com",
    },
    { id: "device-removed", label: "Device removed · Revoked from account" },
  ],
});

const GATE_STATES: Partial<Record<string, BrowserSignInGateKind>> = {
  "sign-in": "sign_in",
  "waiting-approval": "waiting_approval",
  "approval-expired": "approval_expired",
  "not-approved": "not_approved",
  "another-account": "another_account",
  "needs-pro": "needs_pro",
  "device-removed": "device_removed",
};

function renderGate(kind: BrowserSignInGateKind) {
  return (
    <BrowserSignInGate
      browserFingerprint={PHONE_DEVICE.keyFingerprint}
      computerName={PHONE_TARGET_COMPUTER.name}
      deviceName={PHONE_DEVICE.name}
      graceDaysLeft={kind === "needs_pro" ? PHONE_GRACE_DAYS_LEFT : null}
      hostname={PHONE_HOSTED_URL}
      iconKind={PHONE_TARGET_COMPUTER.iconKind}
      kind={kind}
      onPrimaryAction={() => {
        switch (kind) {
          case "sign_in":
            toast("Redirecting to Compozy Cloud sign-in…");
            break;
          case "approval_expired":
          case "not_approved":
            toast(`Requested approval on ${PHONE_TARGET_COMPUTER.name}`);
            break;
          case "another_account":
            toast("Signed out. Redirecting to Compozy Cloud sign-in…");
            break;
          case "needs_pro":
            toast(`Opening https://cloud.compozy.com/billing/upgrade?hostname=${PHONE_HOSTED_URL}`);
            break;
          case "device_removed":
            toast("Starting fresh device sign-in…");
            break;
          default:
            break;
        }
      }}
      onSecondaryAction={() => {
        if (kind === "needs_pro") {
          toast("Opening Hosted route docs…");
        } else {
          toast("Switching account…");
        }
      }}
      onSignInProvider={provider =>
        toast(
          `Redirecting to Compozy Cloud (${provider === "google" ? "Google" : "GitHub"}) sign-in…`
        )
      }
      platform={PHONE_TARGET_COMPUTER.platform}
      signedInEmail={kind === "another_account" ? PHONE_OTHER_ACCOUNT_EMAIL : undefined}
      trustedAdminDeviceName="MacBook Pro"
    />
  );
}

const SIGNED_IN_STATES = new Set([
  "signed-in-sessions",
  "signed-in-session-live",
  "computer-menu",
  "computer-menu-only-host",
  "offline-reconnecting",
  "signed-in-no-terminal-scope",
]);

function addressFor(state: string): string {
  switch (state) {
    case "upgrade-cloud-page":
      return `cloud.compozy.com/billing/upgrade?hostname=${PHONE_HOSTED_URL}`;
    case "signed-in-session-live":
    case "offline-reconnecting":
      return `${PHONE_HOSTED_URL}/sessions/sess-studio-1`;
    case "sign-in":
    case "another-account":
    case "device-removed":
      return `${PHONE_HOSTED_URL}/sign-in`;
    case "waiting-approval":
    case "approval-expired":
    case "not-approved":
      return `${PHONE_HOSTED_URL}/sign-in/approval`;
    default:
      return PHONE_HOSTED_URL;
  }
}

/** Wide browsers (≥1024 px) get the CompozyOS web app shell; the phone form keeps its tabs. */
function renderWideSignedIn(state: string) {
  switch (state) {
    case "signed-in-session-live":
      return <HostedWebApp computer={PHONE_TARGET_COMPUTER} working />;
    case "computer-menu":
      return (
        <HostedWebApp computer={PHONE_TARGET_COMPUTER} computerMenuOpen peers={PHONE_PEERS_MULTI} />
      );
    case "computer-menu-only-host":
      return (
        <HostedWebApp
          computer={PHONE_TARGET_COMPUTER}
          computerMenuOpen
          peers={PHONE_PEERS_SINGLE}
        />
      );
    case "offline-reconnecting":
      return <HostedWebApp computer={PHONE_TARGET_COMPUTER} connectionLost />;
    case "signed-in-no-terminal-scope":
      return <HostedWebApp computer={PHONE_TARGET_COMPUTER} scopes={PHONE_NARROW_SCOPES} />;
    default:
      return <HostedWebApp computer={PHONE_TARGET_COMPUTER} />;
  }
}

function renderState(state: string, isWide: boolean) {
  if (state === "upgrade-cloud-page") {
    return (
      <PhoneUpgradeCloudPage
        computerName={PHONE_TARGET_COMPUTER.name}
        hostname={PHONE_HOSTED_URL}
        iconKind={PHONE_TARGET_COMPUTER.iconKind}
        ownerEmail={PHONE_OWNER_EMAIL}
        platform={PHONE_TARGET_COMPUTER.platform}
      />
    );
  }

  const gateKind = GATE_STATES[state];
  if (gateKind) {
    return renderGate(gateKind);
  }

  if (isWide && SIGNED_IN_STATES.has(state)) {
    return renderWideSignedIn(state);
  }

  switch (state) {
    case "signed-in-session-live":
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          initialOpenSessionId="sess-studio-1"
          scopes={PHONE_FULL_SCOPES}
        />
      );
    case "computer-menu":
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          initialComputerMenuOpen
          peers={PHONE_PEERS_MULTI}
          scopes={PHONE_FULL_SCOPES}
        />
      );
    case "computer-menu-only-host":
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          initialComputerMenuOpen
          peers={PHONE_PEERS_SINGLE}
          scopes={PHONE_FULL_SCOPES}
        />
      );
    case "offline-reconnecting":
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          initialOpenSessionId="sess-studio-1"
          offline
          scopes={PHONE_FULL_SCOPES}
        />
      );
    case "signed-in-no-terminal-scope":
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          scopes={PHONE_NARROW_SCOPES}
          showInstallHint={false}
        />
      );
    default:
      return (
        <PhoneApp
          computer={PHONE_TARGET_COMPUTER}
          peers={PHONE_PEERS_MULTI}
          scopes={PHONE_FULL_SCOPES}
        />
      );
  }
}

export default function RemoteComputersPhoneDraft({ state }: PrototypeProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const isWide = useIsWideBrowser(rootRef);

  return (
    <div className="w-full" ref={rootRef}>
      <DraftBrowserFrame address={addressFor(state)} dataSlot="hosted-route-browser-window">
        <div className="contents" key={`${state}-${isWide ? "wide" : "phone"}`}>
          {renderState(state, isWide)}
        </div>
      </DraftBrowserFrame>
    </div>
  );
}
