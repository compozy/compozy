import { AlertTriangle } from "lucide-react";

import { ConfirmDialog, Spinner } from "@compozy/ui";

import type { MarketplaceCatalogListing } from "../types";
import { MarketplaceTrustWarningList } from "./marketplace-trust-warning-list";

type ExtensionTrustWarning = NonNullable<
  NonNullable<MarketplaceCatalogListing["trust"]>["warnings"]
>[number];

const NO_WARNINGS: readonly ExtensionTrustWarning[] = [];

interface ExtensionTrustDialogProps {
  /** Extension identity being consented to — a catalog entry name or a source-union ref. */
  name: string;
  action: "install" | "update";
  description?: string;
  error?: string | null;
  open: boolean;
  pending?: boolean;
  warnings?: readonly ExtensionTrustWarning[];
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}

/** The single explicit-consent gate for every unverified extension install or update. */
function ExtensionTrustDialog({
  name,
  action,
  description,
  error,
  open,
  pending = false,
  warnings = NO_WARNINGS,
  onConfirm,
  onOpenChange,
}: ExtensionTrustDialogProps) {
  const updating = action === "update";
  const actionLabel = updating ? "Update" : "Install";
  return (
    <ConfirmDialog
      body={
        warnings.length > 0 ? (
          <MarketplaceTrustWarningList items={warnings} />
        ) : (
          <p className="text-small-body text-muted">
            There are no specific warnings. Check where it comes from before you continue.
          </p>
        )
      }
      bodyProps={{ className: "max-h-[min(60vh,32rem)] overflow-y-auto" }}
      cancelButtonProps={{ disabled: pending }}
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "extension-trust-confirm" }}
      confirmLabel={
        pending ? (
          <>
            <Spinner aria-hidden="true" className="size-3" />
            {actionLabel}…
          </>
        ) : (
          `${actionLabel} anyway`
        )
      }
      contentProps={{
        className: "sm:max-w-(--width-modal-sm)",
        "data-testid": "extension-trust-dialog",
      }}
      description={
        description ??
        `This extension isn't verified. Review the warnings before ${updating ? "updating" : "installing"} it.`
      }
      error={error || undefined}
      icon={AlertTriangle}
      isPending={pending}
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title={`${actionLabel} ${name}?`}
      tone="warning"
    />
  );
}

export { ExtensionTrustDialog };
export type { ExtensionTrustDialogProps };
