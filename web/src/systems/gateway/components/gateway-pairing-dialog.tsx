import { RefreshCw } from "@compozy/ui/icons";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Empty,
  QrCode,
  Spinner,
  Time,
} from "@compozy/ui";

import { buildPairingLink } from "../lib/pairing-fragment";
import type { GatewayPairingArtifact } from "../types";
import { GatewayManualCode } from "./gateway-manual-code";
import { GatewayPairingCopyRow } from "./gateway-pairing-copy-row";

export interface GatewayPairingDialogProps {
  open: boolean;
  isMinting: boolean;
  error?: string | null;
  artifact: GatewayPairingArtifact | null;
  /** Verified address a new device should open, when one exists. */
  pairingAddress?: string;
  onOpenChange: (open: boolean) => void;
  onMint: () => void;
}

/**
 * One-time issuance view for a pairing artifact.
 *
 * The artifact is single-use, TTL-bounded, and returned exactly once, so this
 * dialog is the only place it may ever appear. The six-digit manual code is the
 * primary path because it needs no camera and no long copy; the QR code and the
 * long link or artifact remain as secondary paths for the devices that use them.
 */
export function GatewayPairingDialog({
  open,
  isMinting,
  error,
  artifact,
  pairingAddress,
  onOpenChange,
  onMint,
}: GatewayPairingDialogProps) {
  // The link keeps the artifact in its fragment, so scanning it never puts the
  // secret in a request line, a proxy log, or a referrer.
  const pairingUrl =
    artifact && pairingAddress ? buildPairingLink(pairingAddress, artifact.artifact) : null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto"
        data-testid="gateway-pairing-dialog"
        showCloseButton={false}
      >
        <DialogHeader>
          <DialogTitle>Pair a device</DialogTitle>
          <DialogDescription>
            This code works once. Create a new one after it expires or is used.
          </DialogDescription>
        </DialogHeader>

        {isMinting ? (
          <div className="flex items-center justify-center py-8" role="status">
            <Spinner aria-label="Creating a pairing code" className="size-5 text-subtle" />
          </div>
        ) : null}

        {!isMinting && error ? (
          <p
            className="text-form-label text-destructive"
            data-testid="gateway-pairing-error"
            role="alert"
          >
            {error}
          </p>
        ) : null}

        {!isMinting && !error && !artifact ? (
          <Empty
            description="Mint a one-time code, then open it on the device you want to pair."
            title="No pairing code yet"
          />
        ) : null}

        {!isMinting && artifact ? (
          <div className="flex min-w-0 flex-col gap-4" data-testid="gateway-pairing-artifact">
            <GatewayManualCode code={artifact.manual_code} />
            <p className="text-form-label text-muted-foreground">
              Expires <Time iso={artifact.expires_at} mode="relative" />.
            </p>
            {pairingUrl ? (
              <>
                <div className="flex justify-center">
                  <QrCode
                    data-testid="gateway-pairing-qr"
                    label="Scan to open the pairing gate on another device"
                    value={pairingUrl}
                  />
                </div>
                <GatewayPairingCopyRow
                  copyLabel="Copy link"
                  label="Pairing link"
                  testId="gateway-pairing-link"
                  value={pairingUrl}
                />
              </>
            ) : (
              <p
                className="text-form-label text-muted-foreground"
                data-testid="gateway-pairing-no-address"
              >
                There is no verified private address yet, so the other device has nowhere to open.
                Turn on the private overlay and wait for its address to be verified, then mint a new
                code — the current one may expire while you wait.
              </p>
            )}
            <GatewayPairingCopyRow
              copyLabel="Copy long code"
              label="Long pairing code"
              testId="gateway-pairing-artifact-value"
              value={artifact.artifact}
            />
          </div>
        ) : null}

        <DialogFooter>
          <Button
            className="w-full sm:w-auto"
            onClick={() => onOpenChange(false)}
            size="cta-lg"
            type="button"
            variant="ghost"
          >
            Close
          </Button>
          <Button
            className="w-full sm:w-auto"
            data-testid="gateway-pairing-mint"
            disabled={isMinting}
            onClick={onMint}
            size="cta-lg"
            type="button"
          >
            <RefreshCw aria-hidden="true" />
            {artifact ? "Create a new code" : "Create a code"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
