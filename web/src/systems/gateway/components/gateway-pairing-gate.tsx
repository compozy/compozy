import { useLayoutEffect, useState, type FormEvent } from "react";
import { KeyRound } from "@compozy/ui/icons";

import { Button, Empty, Field, FieldDescription, FieldLabel, Input, Logo } from "@compozy/ui";

import { useRedeemGatewayPairing } from "../hooks/use-gateway-access";
import { gatewayErrorCode, type GatewayListenerTier } from "@/lib/gateway-access-signal";
import { classifyPairingInput } from "../lib/pairing-input";
import { pairingRedeemFailureMessage } from "../lib/pairing-redeem-copy";
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
 * never from here, so this gate can redeem a code but never issue one. One
 * field takes either the six-digit code or a pasted pairing link.
 */
export function GatewayPairingGate({ reload, tier }: GatewayPairingGateProps) {
  // A scanned link delivers the artifact in the fragment. Read it once, then
  // strip it from the address bar before paint and long before the redeem
  // request, so it never reaches history, a referrer, or a server log.
  const [codeInput, setCodeInput] = useState(readPairingFragment);
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

  const canSubmit = classifyPairingInput(codeInput) !== null && name.trim() !== "";
  const redeemFailure = redeem.error
    ? pairingRedeemFailureMessage(gatewayErrorCode(redeem.error))
    : null;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const credential = classifyPairingInput(codeInput);
    const deviceName = name.trim();
    if (!credential || deviceName === "") return;
    redeem.mutate({ ...credential, name: deviceName });
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
              <FieldLabel htmlFor="gateway-pairing-code-input">Pairing code or link</FieldLabel>
              <Input
                aria-describedby="gateway-pairing-code-hint"
                autoComplete="off"
                autoFocus
                data-testid="gateway-pairing-gate-code"
                id="gateway-pairing-code-input"
                onChange={event => setCodeInput(event.target.value)}
                spellCheck={false}
                required
                value={codeInput}
              />
              <FieldDescription id="gateway-pairing-code-hint">
                Enter the six-digit code, or paste the pairing link.
              </FieldDescription>
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
            {redeemFailure ? (
              <p
                className="text-form-label text-destructive"
                data-testid="gateway-pairing-gate-error"
                role="alert"
              >
                {redeemFailure}
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
