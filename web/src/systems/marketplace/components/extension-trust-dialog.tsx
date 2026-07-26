import { AlertTriangle } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Pill,
  Spinner,
} from "@agh/ui";

import type { MarketplaceListing } from "../types";

interface ExtensionTrustDialogProps {
  entry: MarketplaceListing;
  error?: string | null;
  open: boolean;
  pending?: boolean;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}

function ExtensionTrustDialog({
  entry,
  error,
  open,
  pending = false,
  onConfirm,
  onOpenChange,
}: ExtensionTrustDialogProps) {
  const warnings = entry.trust?.warnings ?? [];
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        aria-describedby="extension-trust-description"
        className="sm:max-w-(--width-modal-sm)"
        data-testid="extension-trust-dialog"
        unframed
      >
        <DialogHeader className="pr-10" variant="ruled">
          <div className="flex items-start gap-3">
            <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-warning-tint text-warning">
              <AlertTriangle aria-hidden="true" className="size-4" />
            </span>
            <div className="flex flex-col gap-1.5">
              <DialogTitle>Install {entry.name}?</DialogTitle>
              <DialogDescription id="extension-trust-description">
                This extension is not verified. Review the daemon warnings before installing it.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex max-h-[min(60vh,32rem)] flex-col gap-3 overflow-y-auto px-5 py-4">
          {warnings.map(warning => (
            <div
              className="rounded-md border border-line bg-warning-tint px-3 py-2.5"
              key={warning.id}
            >
              <div className="flex items-center gap-2">
                <Pill mono tone={warning.severity === "error" ? "danger" : "warning"}>
                  {warning.severity}
                </Pill>
                <p className="text-small-body font-medium text-fg-strong">{warning.title}</p>
              </div>
              <p className="mt-1.5 text-small-body text-muted">{warning.message}</p>
              {warning.suggested_command ? (
                <code className="mt-2 inline-flex rounded bg-canvas px-2 py-1 font-mono text-form-hint text-fg">
                  {warning.suggested_command}
                </code>
              ) : null}
            </div>
          ))}
          {warnings.length === 0 ? (
            <p className="text-small-body text-muted">
              The registry did not provide a specific warning. Verify the source before continuing.
            </p>
          ) : null}
          {error ? (
            <p
              className="rounded-md bg-danger-tint px-3 py-2 text-small-body text-danger"
              role="alert"
            >
              {error}
            </p>
          ) : null}
        </div>

        <DialogFooter variant="ruled">
          <Button
            disabled={pending}
            onClick={() => onOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <Button
            className="bg-warning-tint text-warning hover:bg-warning-tint hover:opacity-90"
            data-testid="extension-trust-confirm"
            disabled={pending}
            onClick={onConfirm}
            type="button"
            variant="secondary"
          >
            {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {pending ? "Installing…" : "Install anyway"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export { ExtensionTrustDialog };
export type { ExtensionTrustDialogProps };
