import { RefreshCw } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Empty,
  MonoId,
  QrCode,
  Spinner,
  Time,
} from "@compozy/ui";

import { buildPairingLink } from "../lib/pairing-fragment";
import type { GatewayPairingArtifact } from "../types";

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
 * dialog is the only place it may ever appear. Every scannable code has an
 * equally usable text path beside it: a phone camera is one way in, a screen
 * reader, a remote terminal, and a machine without a camera are the others.
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
      <DialogContent data-testid="gateway-pairing-dialog">
        <DialogHeader>
          <DialogTitle>Pair a device</DialogTitle>
          <DialogDescription>
            This code works once and cannot be shown again. Mint a new one if it expires.
          </DialogDescription>
        </DialogHeader>

        {isMinting ? (
          <div className="flex items-center justify-center py-8" role="status">
            <Spinner aria-label="Minting a pairing code" className="size-5 text-subtle" />
          </div>
        ) : null}

        {!isMinting && error ? (
          <p
            className="text-form-label text-danger"
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
          <div className="flex flex-col gap-4" data-testid="gateway-pairing-artifact">
            {pairingUrl ? (
              <div className="flex flex-col items-center gap-2">
                <QrCode
                  data-testid="gateway-pairing-qr"
                  label="Scan to open the pairing gate on another device"
                  value={pairingUrl}
                />
                <MonoId
                  copy
                  copyLabel="Copy pairing link"
                  data-testid="gateway-pairing-link"
                  preserveCase
                  value={pairingUrl}
                />
              </div>
            ) : null}
            <div className="flex flex-col gap-1">
              <span className="eyebrow text-muted">Pairing code</span>
              <MonoId
                copy
                copyLabel="Copy pairing code"
                data-testid="gateway-pairing-code"
                preserveCase
                value={artifact.artifact}
              />
            </div>
            <p className="text-form-label text-muted">
              Expires <Time iso={artifact.expires_at} mode="relative" />.
            </p>
            {pairingUrl ? null : (
              <p className="text-form-label text-muted" data-testid="gateway-pairing-no-address">
                There is no verified private address yet, so the other device has nowhere to open.
                Turn on the private overlay and wait for its address to be verified, then mint a new
                code — the current one may expire while you wait.
              </p>
            )}
          </div>
        ) : null}

        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} type="button" variant="ghost">
            Close
          </Button>
          <Button
            data-testid="gateway-pairing-mint"
            disabled={isMinting}
            onClick={onMint}
            type="button"
          >
            <RefreshCw aria-hidden="true" className="size-3" />
            {artifact ? "Mint a new code" : "Mint a code"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
