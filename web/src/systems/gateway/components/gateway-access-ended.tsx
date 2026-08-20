import { ShieldOff } from "lucide-react";

import { Empty, Logo } from "@compozy/ui";

/**
 * Where a revoked device lands.
 *
 * Deliberately terminal and data-free: the operator revoked this device, so it
 * shows no workspace, session, or posture detail — only what happened and the
 * one way back, which is a new pairing code from a device that is still
 * trusted.
 */
export function GatewayAccessEnded() {
  return (
    <main
      className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-background px-6 py-10"
      data-testid="gateway-access-ended"
    >
      <Logo variant="logo" />
      <Empty
        className="max-w-md"
        description="This device was revoked, so its session and live streams were closed. Mint a new pairing code from the machine running CompozyOS, or from another paired device, to get back in."
        framed
        icon={ShieldOff}
        title="Access ended"
        titleAs="h1"
      />
    </main>
  );
}
