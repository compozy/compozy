import { BadgeCheck, PackageX, Radio } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  MonoId,
  Spinner,
} from "@compozy/ui";

import { useExtensionProvenance } from "../hooks/use-extensions";
import { useRemoveExtension } from "../hooks/use-extension-actions";
import { extensionSourceKindLabel } from "../lib/extension-source-kind";
import { extensionTrustFacts } from "../lib/extension-trust-facts";
import type { ExtensionEntry, ExtensionProvenance } from "../types";
import { ExtensionTrustBadges } from "./extension-trust-badges";

export function ExtensionProvenanceDialog({
  extension,
  open,
  onOpenChange,
}: {
  extension: ExtensionEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const query = useExtensionProvenance(extension?.name ?? "", open);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" unframed>
        <DialogHeader variant="ruled">
          <Eyebrow>Provenance</Eyebrow>
          <DialogTitle>Provenance · {extension?.name}</DialogTitle>
          <DialogDescription>
            Where this extension came from and how it was verified.
          </DialogDescription>
        </DialogHeader>
        <div className="px-5 py-4" data-testid="extension-provenance-content">
          {query.isLoading ? (
            <div className="flex items-center gap-2 text-sm text-muted">
              <Spinner className="size-4" />
              Loading provenance
            </div>
          ) : query.error ? (
            <p className="text-sm text-danger">{query.error.message}</p>
          ) : query.data ? (
            <ProvenanceFields provenance={query.data} />
          ) : (
            <p className="text-sm text-muted">No provenance data is available.</p>
          )}
        </div>
        <DialogFooter variant="ruled">
          <DialogClose render={<Button size="sm" type="button" variant="ghost" />}>
            Done
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ProvenanceFields({ provenance }: { provenance: ExtensionProvenance }) {
  return (
    <dl className="divide-y divide-line-soft" data-testid="extension-provenance-fields">
      <ProvenanceRow term="Source kind">
        <span className="text-xs text-fg" data-testid="extension-provenance-source-kind">
          {extensionSourceKindLabel(provenance.installed_from)}
        </span>
        <code className="break-all font-mono text-mono-id text-muted">
          {provenance.installed_from || "—"}
        </code>
      </ProvenanceRow>
      <ProvenanceRow term="Source">
        <code className="break-all font-mono text-xs text-fg">
          {provenance.source_url ?? provenance.slug ?? "—"}
        </code>
      </ProvenanceRow>
      <ProvenanceRow term="Checksum">
        {provenance.checksum_sha256 ? (
          <MonoId value={provenance.checksum_sha256} />
        ) : (
          <code className="font-mono text-xs text-fg">—</code>
        )}
      </ProvenanceRow>
      <ProvenanceRow term="Archive digest">
        {provenance.archive_digest_sha256 ? (
          <MonoId value={provenance.archive_digest_sha256} />
        ) : (
          <code className="font-mono text-xs text-fg">—</code>
        )}
      </ProvenanceRow>
      <ProvenanceRow term="Integrity and trust">
        <ExtensionTrustBadges facts={extensionTrustFacts(provenance)} showRegistryTier={false} />
        {!provenance.digest_matched && !provenance.checksum_verified ? (
          <span className="text-xs text-muted">No integrity evidence was recorded.</span>
        ) : null}
      </ProvenanceRow>
      <ProvenanceRow term="Registry tier">
        <code className="font-mono text-xs text-fg">{provenance.registry_tier || "—"}</code>
      </ProvenanceRow>
      <ProvenanceRow term="Installed by">
        <code className="font-mono text-xs text-fg">{provenance.installed_by || "—"}</code>
      </ProvenanceRow>
    </dl>
  );
}

function ProvenanceRow({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 py-3 first:pt-0 last:pb-0">
      <dt className="eyebrow text-muted">{term}</dt>
      <dd className="flex min-w-0 flex-wrap items-center gap-1.5">{children}</dd>
    </div>
  );
}

export function RemoveExtensionDialog({
  extension,
  open,
  onOpenChange,
  onRemoved,
}: {
  extension: ExtensionEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRemoved?: () => void;
}) {
  const remove = useRemoveExtension();
  // A dev overlay is a workspace link over the published row: unlinking it never deletes the
  // published installation.
  const isDevOverlay = extension?.dev === true;
  const capabilityCount = extension?.capabilities?.length ?? 0;
  const permissions = extension?.permissions ?? [];
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={
        !extension
          ? { disabled: true, "data-testid": "remove-extension-confirm" }
          : { "data-testid": "remove-extension-confirm" }
      }
      confirmLabel={isDevOverlay ? "Unlink dev overlay" : "Remove extension"}
      confirmTyping={extension?.name}
      description={
        isDevOverlay
          ? `Unlinks the workspace dev overlay for ${extension?.name ?? "this extension"}. The published installation stays in place.`
          : `Removes the installed package and registered runtime resources for ${extension?.name ?? "this extension"}.`
      }
      error={remove.error?.message}
      isPending={remove.isPending}
      note={
        <div className="space-y-1">
          {isDevOverlay ? (
            <p>
              This workspace stops running the linked generation and falls back to the published
              extension. Files under the origin path are left untouched.
            </p>
          ) : (
            <>
              <p>
                This deletes local extension files known to provenance and unregisters{" "}
                {capabilityCount} capabilities.
              </p>
              {(extension?.declared_profiles?.length ?? 0) > 0 ? (
                <p>Declared profiles and their work stay.</p>
              ) : null}
            </>
          )}
          <p>
            Revoked permissions: {permissions.length ? permissions.join(", ") : "none declared"}.
          </p>
        </div>
      }
      noteTone="neutral"
      onConfirm={async () => {
        if (!extension) return;
        await remove.mutateAsync({ dev: isDevOverlay, name: extension.name });
        onOpenChange(false);
        onRemoved?.();
      }}
      onOpenChange={onOpenChange}
      open={open}
      title={`${isDevOverlay ? "Unlink" : "Remove"} ${extension?.name ?? "extension"}`}
      tone="danger"
      confirmIcon={PackageX}
      contentProps={{ "data-testid": "remove-extension-dialog" }}
    />
  );
}

/**
 * One affordance for both enable and update: the daemon refuses either until the operator ratifies
 * the exact digest it returned, so the digest is shown rather than summarised.
 */
export function ExtensionNetworkConfirmDialog({
  action,
  digest,
  error,
  extensionName,
  onConfirm,
  onOpenChange,
  open,
  pending,
}: {
  action: "enable" | "update";
  digest: string;
  error?: string;
  extensionName: string;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
  open: boolean;
  pending: boolean;
}) {
  const verb = action === "enable" ? "Enabling" : "Updating";
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "extension-network-confirm-accept" }}
      confirmIcon={Radio}
      confirmLabel="Confirm and continue"
      contentProps={{ "data-testid": "extension-network-confirm-dialog" }}
      description={`${verb} ${extensionName} applies the Live Compozy Network participation it declares. CompozyOS records this decision against the digest below.`}
      error={error}
      isPending={pending}
      note={
        <div className="space-y-1">
          <p>Requirement digest</p>
          <MonoId data-testid="extension-network-confirm-digest" value={digest} />
        </div>
      }
      noteTone="warning"
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title={`Confirm network participation for ${extensionName}`}
      tone="warning"
    />
  );
}

export function VerifiedMark({ verified }: { verified: boolean }) {
  return verified ? (
    <BadgeCheck aria-label="Checksum verified" className="size-3.5 text-success" />
  ) : null;
}
