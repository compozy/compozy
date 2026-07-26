import { AlertTriangle, BadgeCheck, PackageX } from "lucide-react";
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
} from "@agh/ui";

import { useExtensionProvenance } from "../hooks/use-extensions";
import { useRemoveExtension } from "../hooks/use-extension-actions";
import type { BundleActivation, ExtensionEntry, ExtensionProvenance } from "../types";

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
    <dl className="divide-y divide-line-soft">
      <ProvenanceRow term="Installed from">
        <code className="break-all font-mono text-xs text-fg">
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
  activeBundles,
  dependencyError,
  dependencyLoading,
  open,
  onOpenChange,
  onRemoved,
  onRetryDependencies,
}: {
  extension: ExtensionEntry | null;
  activeBundles: BundleActivation[] | undefined;
  dependencyError: Error | null;
  dependencyLoading: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRemoved?: () => void;
  onRetryDependencies: () => void;
}) {
  const remove = useRemoveExtension();
  const blockers = extension
    ? (activeBundles ?? []).filter(activation => activation.extension_name === extension.name)
    : [];
  const dependenciesReady =
    !dependencyLoading && dependencyError === null && activeBundles !== undefined;
  const blocked = !extension || !dependenciesReady || blockers.length > 0;
  const capabilityCount = extension?.capabilities?.length ?? 0;
  const actionCount = extension?.actions?.length ?? 0;
  const permissions = extension?.provenance?.permissions ?? [];
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={
        blocked
          ? { disabled: true, "data-testid": "remove-extension-confirm" }
          : { "data-testid": "remove-extension-confirm" }
      }
      confirmLabel="Remove extension"
      confirmTyping={extension?.name}
      description={`Removes the installed package and registered runtime resources for ${extension?.name ?? "this extension"}.`}
      error={remove.error?.message}
      isPending={remove.isPending}
      note={
        <div className="space-y-1">
          <p>
            This deletes local extension files known to provenance and unregisters {capabilityCount}
            capabilities and {actionCount} actions.
          </p>
          <p>
            Revoked permissions: {permissions.length ? permissions.join(", ") : "none declared"}.
          </p>
          {dependencyLoading ? (
            <p className="flex items-center gap-2" role="status">
              <Spinner className="size-3.5" />
              Checking active bundles before removal.
            </p>
          ) : dependencyError ? (
            <div className="space-y-2">
              <p>
                Bundle activity could not be loaded. {dependencyError.message} Removal stays blocked
                until this check succeeds.
              </p>
              <Button onClick={onRetryDependencies} size="sm" type="button" variant="outline">
                Retry bundle activity
              </Button>
            </div>
          ) : blockers.length > 0 ? (
            <p>
              The daemon returns 409 while an active bundle depends on this extension. Deactivate
              {` ${blockers.map(item => item.bundle_name).join(", ")}`} first.
            </p>
          ) : null}
        </div>
      }
      noteTone={!dependenciesReady || blockers.length > 0 ? "warning" : "neutral"}
      onConfirm={async () => {
        if (!extension || blocked) return;
        await remove.mutateAsync(extension.name);
        onOpenChange(false);
        onRemoved?.();
      }}
      onOpenChange={onOpenChange}
      open={open}
      title={`Remove ${extension?.name ?? "extension"}`}
      tone="danger"
      confirmIcon={PackageX}
      contentProps={{ "data-testid": "remove-extension-dialog" }}
    />
  );
}

export function DeactivateBundleDialog({
  activation,
  open,
  pending,
  error,
  onConfirm,
  onOpenChange,
}: {
  activation: BundleActivation | null;
  open: boolean;
  pending: boolean;
  error?: string;
  onConfirm: () => Promise<void>;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmIcon={AlertTriangle}
      confirmLabel="Deactivate"
      description="Stops this activation. Installed skills and extensions stay on disk; only the activation binding is removed."
      error={error}
      isPending={pending}
      note={`Stops the ${activation?.profile_name ?? "selected"} profile (${activation?.scope ?? "current scope"}). Installed capabilities stay on disk.`}
      noteTone="neutral"
      onConfirm={onConfirm}
      onOpenChange={onOpenChange}
      open={open}
      title={`Deactivate ${activation?.bundle_name ?? "bundle"}`}
      tone="danger"
      contentProps={{ "data-testid": "deactivate-bundle-dialog" }}
    />
  );
}

export function VerifiedMark({ verified }: { verified: boolean }) {
  return verified ? (
    <BadgeCheck aria-label="Checksum verified" className="size-3.5 text-success" />
  ) : null;
}
