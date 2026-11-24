import { BadgeCheck, PackageX, Radio } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

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
  RadioCardGroup,
  SkeletonRegion,
  SkeletonText,
  type RadioCardGroupOption,
} from "@compozy/ui";

import { useExtensionProvenance } from "../hooks/use-extensions";
import { useRemoveExtension } from "../hooks/use-extension-actions";
import { extensionInstallationScope } from "../lib/extension-installation-scope";
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
            <ProvenanceSkeleton />
          ) : query.error ? (
            <p className="text-small-body text-destructive">{query.error.message}</p>
          ) : query.data ? (
            <ProvenanceFields provenance={query.data} />
          ) : (
            <p className="text-small-body text-muted-foreground">
              No provenance data is available.
            </p>
          )}
        </div>
        <DialogFooter variant="ruled">
          <DialogClose render={<Button type="button" variant="ghost" />}>Done</DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const PROVENANCE_TERM = {
  sourceKind: "Source kind",
  source: "Source",
  checksum: "Checksum",
  archiveDigest: "Archive digest",
  trust: "Integrity and trust",
  registryTier: "Registry tier",
  installedBy: "Installed by",
} as const;

/** The provenance list while it loads: the real terms and row rhythm over placeholder values. */
function ProvenanceSkeleton() {
  return (
    <SkeletonRegion label="Loading provenance">
      <dl className="divide-y divide-line-soft">
        {Object.values(PROVENANCE_TERM).map(term => (
          <ProvenanceRow key={term} term={term}>
            <span className="text-small-body">
              <SkeletonText className="w-56" />
            </span>
          </ProvenanceRow>
        ))}
      </dl>
    </SkeletonRegion>
  );
}

function ProvenanceFields({ provenance }: { provenance: ExtensionProvenance }) {
  return (
    <dl className="divide-y divide-line-soft" data-testid="extension-provenance-fields">
      <ProvenanceRow term={PROVENANCE_TERM.sourceKind}>
        <span className="text-small-body text-fg" data-testid="extension-provenance-source-kind">
          {extensionSourceKindLabel(provenance.installed_from)}
        </span>
        <code className="break-all font-mono text-mono-id text-muted-foreground">
          {provenance.installed_from || "—"}
        </code>
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.source}>
        <code className="break-all font-mono text-eyebrow text-fg">
          {provenance.source_url ?? provenance.slug ?? "—"}
        </code>
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.checksum}>
        {provenance.checksum_sha256 ? (
          <MonoId value={provenance.checksum_sha256} />
        ) : (
          <code className="font-mono text-eyebrow text-fg">—</code>
        )}
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.archiveDigest}>
        {provenance.archive_digest_sha256 ? (
          <MonoId value={provenance.archive_digest_sha256} />
        ) : (
          <code className="font-mono text-eyebrow text-fg">—</code>
        )}
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.trust}>
        <ExtensionTrustBadges facts={extensionTrustFacts(provenance)} showRegistryTier={false} />
        {!provenance.digest_matched && !provenance.checksum_verified ? (
          <span className="text-eyebrow text-muted-foreground">
            No integrity evidence was recorded.
          </span>
        ) : null}
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.registryTier}>
        <code className="font-mono text-eyebrow text-fg">{provenance.registry_tier || "—"}</code>
      </ProvenanceRow>
      <ProvenanceRow term={PROVENANCE_TERM.installedBy}>
        <code className="font-mono text-eyebrow text-fg">{provenance.installed_by || "—"}</code>
      </ProvenanceRow>
    </dl>
  );
}

function ProvenanceRow({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 py-3 first:pt-0 last:pb-0">
      <dt className="eyebrow text-muted-foreground">{term}</dt>
      <dd className="flex min-w-0 flex-wrap items-center gap-1.5">{children}</dd>
    </div>
  );
}

type RemovalChoice = "remove" | "permanent";

const PERMANENT_REMOVAL_CONSEQUENCE = "The built-in will not return on restart or update.";

const REMOVAL_CHOICE_OPTIONS: ReadonlyArray<RadioCardGroupOption<RemovalChoice>> = [
  {
    value: "remove",
    title: "Remove",
    description: "The built-in returns at the next restart.",
    "data-testid": "remove-extension-choice-remove",
  },
  {
    value: "permanent",
    title: "Remove permanently",
    description: PERMANENT_REMOVAL_CONSEQUENCE,
    "data-testid": "remove-extension-choice-permanent",
  },
];

/**
 * Only the global bundled row can be removed for good. The daemon refuses permanent removal of a
 * marketplace or local row, of a dev overlay, and of any workspace-scoped instance.
 */
function canRemoveBuiltinPermanently(extension: ExtensionEntry | null): boolean {
  return extension?.source === "bundled" && extension.dev !== true && !extension.workspace_id;
}

/**
 * `canRemovePermanently` is the viewer's write-permission signal. The web has no per-actor
 * authority field today, so callers pass it for the operator surface; the daemon enforces the
 * same operator check on every permanent request regardless of what the UI shows.
 */
export function RemoveExtensionDialog({
  extension,
  open,
  onOpenChange,
  onRemoved,
  canRemovePermanently = false,
}: {
  extension: ExtensionEntry | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onRemoved?: () => void;
  canRemovePermanently?: boolean;
}) {
  const remove = useRemoveExtension();
  const [removal, setRemoval] = useState<RemovalChoice>("remove");
  // A dev overlay is a workspace link over the published row: unlinking it never deletes the
  // published installation.
  const isDevOverlay = extension?.dev === true;
  const offersPermanent = canRemovePermanently && canRemoveBuiltinPermanently(extension);
  const permanent = offersPermanent && removal === "permanent";
  const subject = extension?.name ?? "this extension";
  // The choice is per open: a reopened dialog starts from the non-permanent default.
  const closeDialog = (nextOpen: boolean) => {
    if (!nextOpen) setRemoval("remove");
    onOpenChange(nextOpen);
  };
  return (
    <ConfirmDialog
      body={
        offersPermanent ? (
          <RadioCardGroup
            aria-label="Removal type"
            onValueChange={setRemoval}
            options={REMOVAL_CHOICE_OPTIONS}
            value={removal}
          />
        ) : undefined
      }
      cancelLabel="Cancel"
      confirmButtonProps={
        !extension
          ? { disabled: true, "data-testid": "remove-extension-confirm" }
          : { "data-testid": "remove-extension-confirm" }
      }
      confirmLabel={
        isDevOverlay ? "Unlink local copy" : permanent ? "Remove permanently" : "Remove extension"
      }
      confirmTyping={extension?.name}
      description={
        isDevOverlay
          ? `Unlinks the local development copy of ${subject} in this project. The published installation stays in place.`
          : `Removes ${subject} and everything it added.`
      }
      error={remove.error?.message}
      isPending={remove.isPending}
      note={<RemoveExtensionNote extension={extension} isDevOverlay={isDevOverlay} />}
      noteTone="neutral"
      onConfirm={async () => {
        if (!extension) return;
        // Permanent removal is global: the daemon refuses a workspace or profile with it, so the
        // installation scope is left out of that request.
        await remove.mutateAsync({
          ...(permanent ? {} : extensionInstallationScope(extension)),
          dev: isDevOverlay,
          name: extension.name,
          ...(permanent ? { permanent: true } : {}),
        });
        closeDialog(false);
        onRemoved?.();
      }}
      onOpenChange={closeDialog}
      open={open}
      title={`${isDevOverlay ? "Unlink" : "Remove"} ${extension?.name ?? "extension"}`}
      tone="destructive"
      confirmIcon={PackageX}
      contentProps={{ "data-testid": "remove-extension-dialog" }}
    />
  );
}

function RemoveExtensionNote({
  extension,
  isDevOverlay,
}: {
  extension: ExtensionEntry | null;
  isDevOverlay: boolean;
}) {
  const permissions = extension?.permissions ?? [];
  return (
    <div className="space-y-1">
      {isDevOverlay ? (
        <p>
          This project goes back to the published extension. Your local files are left untouched.
        </p>
      ) : (
        <RemoveExtensionImpact extension={extension} />
      )}
      <p>Permissions removed: {permissions.length ? permissions.join(", ") : "none"}.</p>
    </div>
  );
}

function RemoveExtensionImpact({ extension }: { extension: ExtensionEntry | null }) {
  const capabilityCount = extension?.capabilities?.length ?? 0;
  const keepsProfiles =
    (extension?.declared_profiles?.length ?? 0) > 0 ||
    extension?.placements?.some(placement => Boolean(placement.profile?.trim()));
  return (
    <>
      <p>
        This deletes the extension's files and removes the {capabilityCount}{" "}
        {capabilityCount === 1 ? "capability" : "capabilities"} it provides.
      </p>
      {keepsProfiles ? <p>Declared profiles and their work stay.</p> : null}
    </>
  );
}

/**
 * One affordance for both enable and update: the daemon refuses either until the operator ratifies
 * the exact digest it returned, so the digest is shown rather than summarised.
 */
export function VerifiedMark({ verified }: { verified: boolean }) {
  return verified ? (
    <BadgeCheck aria-label="Checksum verified" className="size-3.5 text-success" />
  ) : null;
}
