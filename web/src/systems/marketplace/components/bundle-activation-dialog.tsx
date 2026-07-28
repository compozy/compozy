import { Box, RefreshCw } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Pill,
  RadioCard,
  Spinner,
  Switch,
} from "@compozy/ui";

import { usePreviewMarketplaceBundle } from "../hooks/use-marketplace-actions";
import type { MarketplaceEntryResponse } from "../types";
import { marketplaceErrorMessage } from "./marketplace-ui";
import { useBundleActivationDialog } from "./use-bundle-activation-dialog";

interface BundleActivationDialogProps {
  data: MarketplaceEntryResponse;
  workspaceId?: string | null;
  onOpenChange: (open: boolean) => void;
}

function BundleActivationDialog({ data, workspaceId, onOpenChange }: BundleActivationDialogProps) {
  const bundle = data.bundle;
  const {
    activateBundle,
    confirmNetworkRequirement,
    error,
    phase,
    preview,
    profile,
    rerunPreview,
    scope,
    setConfirmNetworkRequirement,
    setProfile,
    setScope,
  } = useBundleActivationDialog({ data, onOpenChange, workspaceId });

  if (!bundle) return null;
  const isActivating = phase === "activating";
  const isPreviewing = phase === "preview-pending" || phase === "previewing";
  const handleOpenChange = (open: boolean) => {
    if (!open && isActivating) return;
    onOpenChange(open);
  };
  const requiresNetworkConfirmation = Boolean(preview.data?.activation.network_requirement_digest);
  return (
    <Dialog onOpenChange={handleOpenChange} open>
      <DialogContent
        aria-describedby="bundle-preview-description"
        className="sm:max-w-(--width-modal-md)"
        data-testid="bundle-activation-dialog"
        unframed
      >
        <DialogHeader className="pr-10" variant="ruled">
          <div className="flex items-start gap-3">
            <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-surface-glaze text-muted">
              <Box aria-hidden="true" className="size-4" />
            </span>
            <div className="flex flex-col gap-1.5">
              <p className="eyebrow text-muted">Bundle preview</p>
              <DialogTitle>Activate {data.entry.name}</DialogTitle>
              <DialogDescription id="bundle-preview-description">
                Choose a profile and scope. The preview is recalculated before activation.
              </DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="flex max-h-[min(68vh,42rem)] flex-col gap-5 overflow-y-auto px-5 py-4">
          {bundle.profiles.length > 1 ? (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-1 text-form-label font-medium text-fg-strong">Profile</legend>
              <div
                aria-label="Bundle profile"
                className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                role="radiogroup"
              >
                {bundle.profiles.map(candidate => (
                  <RadioCard
                    description={candidate.description}
                    key={candidate.name}
                    onSelect={() => setProfile(candidate.name)}
                    selected={profile === candidate.name}
                    title={candidate.name}
                  />
                ))}
              </div>
            </fieldset>
          ) : (
            <p className="text-small-body text-muted">
              Profile <span className="font-medium text-fg-strong">{profile}</span>
              {bundle.profiles[0]?.description ? ` · ${bundle.profiles[0].description}` : ""}
            </p>
          )}

          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-form-label font-medium text-fg-strong">Scope</legend>
            <div
              aria-label="Bundle activation scope"
              className="grid grid-cols-1 gap-2 sm:grid-cols-2"
              role="radiogroup"
            >
              <RadioCard
                description="Activate in this workspace only."
                disabled={!workspaceId}
                onSelect={() => setScope("workspace")}
                selected={scope === "workspace"}
                title="Workspace"
              />
              <RadioCard
                description="Activate across workspaces on this daemon."
                onSelect={() => setScope("global")}
                selected={scope === "global"}
                title="Global"
              />
            </div>
          </fieldset>

          {requiresNetworkConfirmation ? (
            <div className="flex items-start gap-3 rounded-md bg-canvas px-3 py-3">
              <Switch
                aria-describedby="marketplace-network-confirmation-description"
                checked={confirmNetworkRequirement}
                id="marketplace-network-confirmation"
                onCheckedChange={setConfirmNetworkRequirement}
                size="sm"
              />
              <span className="flex flex-col gap-0.5">
                <label
                  className="text-small-body font-medium text-fg-strong"
                  htmlFor="marketplace-network-confirmation"
                >
                  Confirm Live network participation
                </label>
                <span
                  className="text-form-hint text-muted"
                  id="marketplace-network-confirmation-description"
                >
                  CompozyOS records this extension’s current participation requirement with the
                  activation.
                </span>
              </span>
            </div>
          ) : null}

          <section className="flex flex-col gap-2.5" aria-live="polite">
            <div className="flex items-center gap-2">
              <h3 className="text-small-body font-medium text-fg-strong">What changes</h3>
              {isPreviewing ? <Spinner aria-hidden="true" className="size-3" /> : null}
            </div>
            {preview.data ? <BundlePreviewInventory data={preview.data} /> : null}
            {isPreviewing && !preview.data ? (
              <div className="flex min-h-32 items-center justify-center rounded-md bg-canvas text-small-body text-muted">
                Loading preview
              </div>
            ) : null}
            {error ? (
              <div
                className="flex items-start gap-2 rounded-md bg-danger-tint px-3 py-2.5 text-small-body text-danger"
                data-testid="bundle-preview-error"
                role="alert"
              >
                <span className="flex-1">
                  {marketplaceErrorMessage(error, "The bundle preview could not be loaded")}
                </span>
                <Button
                  aria-label="Retry bundle preview"
                  onClick={rerunPreview}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  <RefreshCw aria-hidden="true" className="size-3" />
                  Retry
                </Button>
              </div>
            ) : null}
          </section>
        </div>

        <DialogFooter variant="ruled">
          <Button
            disabled={isActivating}
            onClick={() => handleOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <Button
            data-testid="bundle-activate-confirm"
            disabled={
              !preview.data ||
              isPreviewing ||
              isActivating ||
              Boolean(error) ||
              (requiresNetworkConfirmation && !confirmNetworkRequirement)
            }
            onClick={() => void activateBundle()}
            type="button"
          >
            {isActivating ? <Spinner aria-hidden="true" className="size-3" /> : null}
            {isActivating ? "Activating…" : "Activate"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function BundlePreviewInventory({
  data,
}: {
  data: NonNullable<ReturnType<typeof usePreviewMarketplaceBundle>["data"]>;
}) {
  const { activation } = data;
  const rows = [
    ...(activation.inventory ?? []).map(item => ({
      id: `${item.resource_kind}:${item.resource_id}`,
      kind: item.resource_kind,
      name: item.resource_name,
    })),
    ...(activation.agents ?? []).map(item => ({
      id: `agent:${item.id}`,
      kind: "agent",
      name: item.name,
    })),
    ...(activation.jobs ?? []).map(item => ({
      id: `job:${item.id}`,
      kind: "job",
      name: item.name,
    })),
    ...(activation.triggers ?? []).map(item => ({
      id: `trigger:${item.id}`,
      kind: "trigger",
      name: item.name,
    })),
    ...(activation.bridges ?? []).map(item => ({
      id: `bridge:${item.id}`,
      kind: "bridge",
      name: item.display_name,
    })),
    ...(activation.channels ?? []).map(item => ({
      id: `channel:${item.name}`,
      kind: "channel",
      name: item.name,
    })),
  ];
  const uniqueRows = [...new Map(rows.map(row => [row.id, row])).values()];
  return (
    <div className="overflow-hidden rounded-md bg-canvas">
      {uniqueRows.length === 0 ? (
        <p className="px-3 py-4 text-small-body text-muted">No resource changes were returned.</p>
      ) : (
        uniqueRows.map(row => (
          <div
            className="grid grid-cols-[7rem_minmax(0,1fr)] items-center gap-3 border-b border-line px-3 py-2.5 last:border-b-0"
            key={row.id}
          >
            <Pill mono>{row.kind}</Pill>
            <span className="truncate text-small-body text-fg">{row.name}</span>
          </div>
        ))
      )}
    </div>
  );
}

export { BundleActivationDialog };
export type { BundleActivationDialogProps };
