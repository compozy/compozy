import {
  ClockAlert,
  ExternalLink,
  Lock,
  LogIn,
  RefreshCw,
  ShieldAlert,
  ShieldOff,
  Smartphone,
  Sparkles,
  UserRound,
} from "@compozy/ui/icons";

import { Button, cn, Logo, Spinner, Surface } from "@compozy/ui";

import type { ComputerIconKind, ComputerPlatform } from "../mocks/remote-computers-fixtures";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { KeyFingerprint } from "./key-fingerprint";

export type BrowserSignInGateKind =
  | "sign_in"
  | "waiting_approval"
  | "approval_expired"
  | "not_approved"
  | "another_account"
  | "needs_pro"
  | "device_removed";

export interface BrowserSignInGateProps {
  kind: BrowserSignInGateKind;
  computerName: string;
  hostname: string;
  platform?: ComputerPlatform;
  iconKind?: ComputerIconKind;
  /** Shown on `waiting_approval` so the owner can match it on the desktop. */
  browserFingerprint?: string;
  /** Shown on `waiting_approval` (e.g. `iPhone (Safari)`). */
  deviceName?: string;
  /** Optional trusted admin device example for guidance copy. */
  trustedAdminDeviceName?: string;
  /** Shown on `another_account` so the owner sees which account is signed in. */
  signedInEmail?: string;
  /** Days remaining in the 7-day entitlement-only window (`needs_pro`). */
  graceDaysLeft?: number | null;
  onPrimaryAction?: () => void;
  onSecondaryAction?: () => void;
  className?: string;
}

interface GateIconVisual {
  toneClass: string;
  glyph: React.ReactNode;
}

function renderGateIcon(kind: BrowserSignInGateKind): GateIconVisual {
  switch (kind) {
    case "sign_in":
      return {
        toneClass: "bg-surface-2 text-fg",
        glyph: <LogIn className="size-5" />,
      };
    case "waiting_approval":
      return {
        toneClass: "bg-warning-tint text-warning",
        glyph: <Smartphone className="size-5" />,
      };
    case "approval_expired":
      return {
        toneClass: "bg-warning-tint text-warning",
        glyph: <ClockAlert className="size-5" />,
      };
    case "not_approved":
      return {
        toneClass: "bg-destructive-tint text-destructive",
        glyph: <ShieldAlert className="size-5" />,
      };
    case "another_account":
      return {
        toneClass: "bg-surface-2 text-subtle",
        glyph: <UserRound className="size-5" />,
      };
    case "needs_pro":
      return {
        toneClass: "bg-brand-tint text-brand",
        glyph: <Sparkles className="size-5" />,
      };
    case "device_removed":
      return {
        toneClass: "bg-destructive-tint text-destructive",
        glyph: <ShieldOff className="size-5" />,
      };
  }
}

/**
 * Centered entry gate for a computer's hosted route (`https://<slug>.remote.compozy.com`, U8).
 *
 * Works at mobile (390 px) and desktop (1396 px+) widths with a centered max-w-[420px] card:
 * icon + one title + plain body + optional context block + actions, with no redundant status pills.
 */
export function BrowserSignInGate({
  kind,
  computerName,
  hostname,
  platform,
  iconKind = "desktop",
  browserFingerprint,
  deviceName = "This browser",
  trustedAdminDeviceName = "MacBook Pro",
  signedInEmail,
  graceDaysLeft,
  onPrimaryAction,
  onSecondaryAction,
  className,
}: BrowserSignInGateProps) {
  const iconVisual = renderGateIcon(kind);

  return (
    <main
      className={cn(
        "flex min-h-dvh w-full flex-1 flex-col items-center justify-center bg-canvas px-5 py-10 text-fg",
        className
      )}
      data-gate-kind={kind}
      data-slot="browser-sign-in-gate"
    >
      <div className="flex w-full max-w-[420px] flex-col items-center gap-6">
        <div className="flex flex-col items-center gap-2.5">
          <Logo className="h-7" variant="logo" />
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

        <Surface className="flex w-full flex-col gap-5 p-6" variant="card">
          <div className="flex flex-col items-center gap-3 text-center">
            <span
              aria-hidden="true"
              className={cn(
                "inline-flex size-11 items-center justify-center rounded-lg",
                iconVisual.toneClass
              )}
            >
              {iconVisual.glyph}
            </span>

            {kind === "sign_in" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  Sign in to Compozy
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  Sign in with your Compozy account to open sessions and terminals on{" "}
                  <span className="font-medium text-fg">{computerName}</span>.
                </p>
              </>
            ) : null}

            {kind === "waiting_approval" ? (
              <>
                <h1 className="inline-flex items-center justify-center gap-2 text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  <Spinner aria-hidden="true" className="size-4 shrink-0 text-warning" />
                  <span>Waiting for approval on {computerName}</span>
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  Approve <span className="font-medium text-fg">{deviceName}</span> on{" "}
                  <span className="font-medium text-fg">{computerName}</span> in{" "}
                  <span className="font-medium text-fg">Settings → Computers</span>, or on a trusted
                  admin device such as{" "}
                  <span className="font-medium text-fg">{trustedAdminDeviceName}</span> in{" "}
                  <span className="font-medium text-fg">Settings → Account</span>.
                </p>
              </>
            ) : null}

            {kind === "approval_expired" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  Approval request expired
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  Approval request expired. Try again. Requests time out after 10 minutes if nobody
                  decides on <span className="font-medium text-fg">{computerName}</span>.
                </p>
              </>
            ) : null}

            {kind === "not_approved" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  {computerName} didn&apos;t approve this device
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  This browser was not approved to connect to{" "}
                  <span className="font-medium text-fg">{computerName}</span>. Request approval
                  again if that was a mistake, or switch accounts.
                </p>
              </>
            ) : null}

            {kind === "another_account" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  This computer belongs to another account
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  <span className="font-medium text-fg">{computerName}</span> is linked to a
                  different Compozy account. Sign out to switch to the account that owns it.
                </p>
              </>
            ) : null}

            {kind === "needs_pro" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  This computer&apos;s hosted route needs Pro
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  Your Pro plan ended, so browser access to{" "}
                  <span className="font-medium text-fg">{computerName}</span> is paused. Agents on{" "}
                  <span className="font-medium text-fg">{computerName}</span> keep running, and
                  local or SSH access is unaffected.
                </p>
              </>
            ) : null}

            {kind === "device_removed" ? (
              <>
                <h1 className="text-empty-h1 font-medium tracking-empty-h1 text-fg-strong">
                  This device was removed from your account
                </h1>
                <p className="text-small-body leading-relaxed text-muted-foreground">
                  Open streams on this browser were closed. Sign in again to register this device
                  and reconnect to <span className="font-medium text-fg">{computerName}</span>.
                </p>
              </>
            ) : null}
          </div>

          {kind === "waiting_approval" && browserFingerprint ? (
            <div className="flex flex-col gap-2">
              <KeyFingerprint
                columns={3}
                label={`${deviceName} fingerprint`}
                layout="grid"
                value={browserFingerprint}
              />
              <p className="text-center text-eyebrow text-subtle">
                This page signs in on its own once approved.
              </p>
            </div>
          ) : null}

          {kind === "another_account" && signedInEmail ? (
            <div className="flex items-center justify-between gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5 text-small-body">
              <span className="text-subtle">Signed in as</span>
              <span className="min-w-0 truncate font-medium text-fg">{signedInEmail}</span>
            </div>
          ) : null}

          {kind === "needs_pro" && graceDaysLeft != null ? (
            <div className="flex items-center justify-between gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5 text-eyebrow text-muted-foreground">
              <span>Hosted address reserved</span>
              <span className="font-medium text-fg tabular-nums">
                {graceDaysLeft} {graceDaysLeft === 1 ? "day" : "days"} left
              </span>
            </div>
          ) : null}

          <div className="flex flex-col gap-2">
            {kind === "sign_in" ? (
              <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                Sign in
              </Button>
            ) : null}

            {kind === "waiting_approval" ? (
              <Button className="w-full" onClick={onSecondaryAction} size="lg" variant="ghost">
                Cancel and switch account
              </Button>
            ) : null}

            {kind === "approval_expired" ? (
              <>
                <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                  <RefreshCw className="size-4" />
                  Request approval again
                </Button>
                <Button className="w-full" onClick={onSecondaryAction} size="lg" variant="ghost">
                  Sign in with another account
                </Button>
              </>
            ) : null}

            {kind === "not_approved" ? (
              <>
                <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                  Try again
                </Button>
                <Button className="w-full" onClick={onSecondaryAction} size="lg" variant="ghost">
                  Switch account
                </Button>
              </>
            ) : null}

            {kind === "another_account" ? (
              <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                Sign out and switch account
              </Button>
            ) : null}

            {kind === "needs_pro" ? (
              <>
                <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                  Upgrade to Pro
                  <ExternalLink className="size-3.5" />
                </Button>
                <Button className="w-full" onClick={onSecondaryAction} size="lg" variant="ghost">
                  Learn more
                </Button>
              </>
            ) : null}

            {kind === "device_removed" ? (
              <Button className="w-full" onClick={onPrimaryAction} size="cta-lg">
                Sign in again
              </Button>
            ) : null}
          </div>
        </Surface>

        {kind === "sign_in" ? (
          <div className="flex items-center gap-1.5 text-eyebrow text-faint">
            <Lock aria-hidden="true" className="size-3 shrink-0" />
            <span>Connected directly to {computerName}</span>
          </div>
        ) : null}
      </div>
    </main>
  );
}
