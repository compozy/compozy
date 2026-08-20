import { useLayoutEffect, useState, type FormEvent } from "react";
import { KeyRound } from "lucide-react";

import { Button, Empty, Field, FieldLabel, Input, Logo } from "@compozy/ui";

import { useRedeemGatewayPairing } from "../hooks/use-gateway-access";
import type { GatewayListenerTier } from "@/lib/gateway-access-signal";
import { clearPairingFragment, readPairingFragment } from "../lib/pairing-fragment";

export interface GatewayPairingGateProps {
  /** Injected in tests so redeeming does not navigate. */
  reload?: () => void;
  tier?: GatewayListenerTier;
}

/**
 * What an unpaired browser sees, and the only thing it sees.
 *
 * Pairing codes are minted from the private overlay or the daemon machine and
 * never from here, so this gate can redeem a code but never issue one.
 */
export function GatewayPairingGate({ reload, tier }: GatewayPairingGateProps) {
  // A scanned link delivers the artifact in the fragment. Read it once, then
  // strip it from the address bar before paint and long before the redeem
  // request, so it never reaches history, a referrer, or a server log.
  const [artifact, setArtifact] = useState(readPairingFragment);
  const [name, setName] = useState("");
  const redeem = useRedeemGatewayPairing(reload);
  useLayoutEffect(() => clearPairingFragment(), []);

  if (tier === "public") {
    return (
      <main
        className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-background px-6 py-10"
        data-testid="gateway-pairing-gate"
      >
        <Logo variant="logo" />
        <Empty
          className="max-w-md"
          description="Open this machine's private address, or use the machine running CompozyOS, to pair this device."
          framed
          icon={KeyRound}
          title="Pairing is unavailable here"
          titleAs="h1"
        />
      </main>
    );
  }

  const canSubmit = artifact.trim() !== "" && name.trim() !== "";

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const code = artifact.trim();
    const deviceName = name.trim();
    if (code === "" || deviceName === "") return;
    redeem.mutate({ artifact: code, name: deviceName });
  };

  return (
    <main
      className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-background px-6 py-10"
      data-testid="gateway-pairing-gate"
    >
      <Logo variant="logo" />
      <Empty
        className="max-w-md"
        description="Enter a one-time pairing code minted from the machine running CompozyOS, or from a device you have already paired."
        framed
        icon={KeyRound}
        title="Pair this device"
        titleAs="h1"
        action={
          <form className="flex w-full flex-col gap-3" onSubmit={handleSubmit}>
            <Field>
              <FieldLabel htmlFor="gateway-pairing-code-input">Pairing code</FieldLabel>
              <Input
                autoComplete="off"
                autoFocus
                data-testid="gateway-pairing-gate-code"
                id="gateway-pairing-code-input"
                onChange={event => setArtifact(event.target.value)}
                spellCheck={false}
                required
                value={artifact}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="gateway-pairing-name-input">Name this device</FieldLabel>
              <Input
                autoComplete="off"
                data-testid="gateway-pairing-gate-name"
                id="gateway-pairing-name-input"
                onChange={event => setName(event.target.value)}
                placeholder="Work phone"
                required
                value={name}
              />
            </Field>
            {redeem.error ? (
              <p
                className="text-form-label text-danger"
                data-testid="gateway-pairing-gate-error"
                role="alert"
              >
                {redeem.error.message}
              </p>
            ) : null}
            <Button
              data-testid="gateway-pairing-gate-submit"
              disabled={redeem.isPending || !canSubmit}
              type="submit"
            >
              Pair this device
            </Button>
          </form>
        }
      />
    </main>
  );
}
