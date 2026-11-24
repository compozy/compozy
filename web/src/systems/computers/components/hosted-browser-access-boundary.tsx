import type { ReactNode } from "react";
import { Spinner } from "@compozy/ui";

import { isHostedOriginCandidate } from "@/lib/hosted-browser-auth";

import { HostedBrowserContext } from "../contexts/hosted-browser-context";
import {
  useHostedBrowserAccess,
  type UseHostedBrowserAccessOptions,
} from "../hooks/use-hosted-browser-access";
import { useIsWideBrowser } from "../hooks/use-is-wide-browser";
import { BrowserSignInGate } from "./browser-sign-in-gate";
import { HostedPhoneApp } from "./hosted-phone-app";

export interface HostedBrowserAccessBoundaryProps {
  children: ReactNode;
  /** Boundary rendered on non-hosted (local / legacy gateway) origins. */
  fallbackBoundary?: ReactNode;
  /** Test seam for navigation / approval polling options. */
  accessOptions?: Omit<UseHostedBrowserAccessOptions, "enabled">;
  /** Optional phone app override (tests only). */
  phoneApp?: ReactNode;
}

function HostedBrowserAccessActive({
  children,
  accessOptions,
  phoneApp,
}: Pick<HostedBrowserAccessBoundaryProps, "children" | "accessOptions" | "phoneApp">) {
  const access = useHostedBrowserAccess({ enabled: true, ...accessOptions });
  const isWide = useIsWideBrowser();

  if (access.state.kind === "checking") {
    return (
      <div
        className="flex min-h-dvh items-center justify-center bg-background text-muted-foreground"
        data-testid="hosted-browser-checking"
      >
        <div className="flex items-center gap-2 text-small-body">
          <Spinner aria-hidden="true" className="size-4" />
          <span>Connecting to {access.computerName}…</span>
        </div>
      </div>
    );
  }

  if (access.state.kind !== "ready") {
    return (
      <BrowserSignInGate
        browserFingerprint={
          access.state.kind === "waiting_approval" ? access.browserFingerprint : undefined
        }
        computerName={access.computerName}
        deviceName={access.deviceName}
        errorMessage={access.state.kind === "sign_in" ? access.state.errorMessage : undefined}
        graceDaysLeft={access.state.kind === "needs_pro" ? access.state.graceDaysLeft : undefined}
        hostname={access.hostname}
        kind={access.state.kind}
        onPrimaryAction={access.primaryGateAction}
        onSecondaryAction={access.secondaryGateAction}
        onSignInProvider={access.signInWithProvider}
        signedInEmail={
          access.state.kind === "another_account" ? access.state.signedInEmail : undefined
        }
      />
    );
  }

  if (!access.contextValue) {
    return (
      <div
        className="flex min-h-dvh items-center justify-center bg-background text-muted-foreground"
        data-testid="hosted-browser-checking"
      >
        <div className="flex items-center gap-2 text-small-body">
          <Spinner aria-hidden="true" className="size-4" />
          <span>Connecting to {access.computerName}…</span>
        </div>
      </div>
    );
  }

  return (
    <HostedBrowserContext value={access.contextValue}>
      {isWide ? children : (phoneApp ?? <HostedPhoneApp />)}
    </HostedBrowserContext>
  );
}

/**
 * Root access boundary for hosted computer origins (`https://<slug>.compozy.run`, U8).
 *
 * On hosted origins, owns the Sign in to CompozyOS gate (WebCrypto P-256 + PKCE + DPoP),
 * renders the desktop web shell at wide widths (`>= 1024 px`), and switches to the
 * compact phone layout below `1024 px`. On non-hosted origins, delegates transparently
 * to `fallbackBoundary` (legacy gateway boundary until S14) or renders `children`.
 */
export function HostedBrowserAccessBoundary({
  children,
  fallbackBoundary,
  accessOptions,
  phoneApp,
}: HostedBrowserAccessBoundaryProps) {
  if (!isHostedOriginCandidate()) {
    return <>{fallbackBoundary ?? children}</>;
  }
  return (
    <HostedBrowserAccessActive accessOptions={accessOptions} phoneApp={phoneApp}>
      {children}
    </HostedBrowserAccessActive>
  );
}
