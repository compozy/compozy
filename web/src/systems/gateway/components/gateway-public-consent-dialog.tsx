import { useState } from "react";

import {
  Button,
  Checkbox,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@compozy/ui";

import { PUBLIC_OPERATOR_CONSENT_DISCLOSURE } from "../lib/gateway-copy";

export interface GatewayPublicConsentDialogProps {
  open: boolean;
  isSubmitting: boolean;
  error?: string | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}

/**
 * The consent step for putting the operator UI on the public address.
 *
 * The daemon takes consent as a per-call boolean and requires it again on every
 * enable, so this dialog holds no memory: the acknowledgement resets whenever
 * it closes, and there is no "don't ask again".
 */
export function GatewayPublicConsentDialog({
  open,
  isSubmitting,
  error,
  onOpenChange,
  onConfirm,
}: GatewayPublicConsentDialogProps) {
  const [acknowledged, setAcknowledged] = useState(false);

  const handleOpenChange = (next: boolean) => {
    if (!next) setAcknowledged(false);
    onOpenChange(next);
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent data-testid="gateway-public-consent-dialog">
        <DialogHeader>
          <DialogTitle>Serve the operator UI on your public address?</DialogTitle>
          <DialogDescription>
            Confirm what becomes publicly reachable before this is turned on.
          </DialogDescription>
        </DialogHeader>
        <ul className="flex list-disc flex-col gap-2 pl-5 text-form-label text-muted">
          {PUBLIC_OPERATOR_CONSENT_DISCLOSURE.map(line => (
            <li key={line}>{line}</li>
          ))}
        </ul>
        <label className="flex items-start gap-2.5 text-form-label text-fg">
          <Checkbox
            checked={acknowledged}
            data-testid="gateway-public-consent-acknowledge"
            onCheckedChange={checked => setAcknowledged(checked === true)}
          />
          <span>I understand what becomes publicly reachable.</span>
        </label>
        {error ? (
          <p className="text-form-label text-danger" role="alert">
            {error}
          </p>
        ) : null}
        <DialogFooter>
          <Button onClick={() => handleOpenChange(false)} type="button" variant="ghost">
            Cancel
          </Button>
          <Button
            data-testid="gateway-public-consent-confirm"
            disabled={!acknowledged || isSubmitting}
            onClick={onConfirm}
            type="button"
          >
            Serve on the public address
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
