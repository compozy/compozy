import { AlertCircle } from "lucide-react";

import { Button, ConfirmDialog, Pill, Spinner } from "@compozy/ui";

import { useMarketplaceSources } from "@/systems/marketplace";

import { MarketplaceSourceRow } from "./marketplace-source-row";
import { SettingsGroup } from "./settings-group";
import { useMarketplaceSourceActions } from "./use-marketplace-source-actions";

const TEST_ID = "settings-page-marketplace-sources";

export interface SettingsMarketplaceSourcesSectionProps {
  /** Opens the shared Add plugin marketplace dialog (the page owns it). */
  onAdd: () => void;
}

/**
 * Plugin marketplaces in the daemon's authoritative order: the CompozyOS catalog first and always
 * on, presets with a switch, custom sources with a `custom` pill and Remove inside the fold. The
 * group title carries the surface's stability label for this release.
 */
export function SettingsMarketplaceSourcesSection({
  onAdd,
}: SettingsMarketplaceSourcesSectionProps) {
  const query = useMarketplaceSources();
  const actions = useMarketplaceSourceActions();
  const sources = query.data?.sources ?? [];

  return (
    <SettingsGroup
      action={
        <Button
          data-testid={`${TEST_ID}-add`}
          onClick={onAdd}
          size="sm"
          type="button"
          variant="outline"
        >
          Add plugin marketplace…
        </Button>
      }
      data-testid={TEST_ID}
      description="Marketplaces you turn on list their plugins in the Marketplace. Nothing installs until you choose to."
      title={
        <span className="inline-flex items-center gap-2">
          Plugin marketplaces
          <Pill data-testid={`${TEST_ID}-experimental`} form="hollow" size="xs">
            experimental
          </Pill>
        </span>
      }
    >
      {query.isPending ? (
        <div
          aria-label="Loading plugin marketplaces"
          className="flex min-h-11 items-center gap-2 px-3 text-form-hint text-subtle"
          data-testid={`${TEST_ID}-loading`}
          role="status"
        >
          <Spinner aria-hidden="true" className="size-3" />
          Reading sources…
        </div>
      ) : query.isError ? (
        <div
          className="flex min-h-11 flex-wrap items-center gap-2 px-3 py-2 text-form-hint text-danger"
          data-testid={`${TEST_ID}-error`}
          role="alert"
        >
          <AlertCircle aria-hidden="true" className="size-3.5 shrink-0" />
          <span className="min-w-0 flex-1">{query.error.message}</span>
          <Button onClick={() => void query.refetch()} size="sm" type="button" variant="ghost">
            Retry
          </Button>
        </div>
      ) : (
        <div className="flex min-w-0 flex-col" data-testid={`${TEST_ID}-list`}>
          {sources.map(source => (
            <MarketplaceSourceRow
              key={source.name}
              onRefresh={() => actions.refresh(source.name)}
              onRemove={source.kind === "custom" ? () => actions.askRemove(source.name) : undefined}
              onToggle={
                source.kind === "feed" ? undefined : enabled => actions.toggle(source.name, enabled)
              }
              pending={actions.isPending(source.name)}
              source={source}
            />
          ))}
        </div>
      )}
      <ConfirmDialog
        cancelLabel="Cancel"
        confirmButtonProps={{ "data-testid": `${TEST_ID}-remove-confirm` }}
        confirmLabel="Remove"
        contentProps={{ "data-testid": `${TEST_ID}-remove-dialog` }}
        description="Its section leaves the Marketplace. Extensions you installed from it stay installed."
        error={actions.removeError}
        isPending={actions.isRemoving}
        onConfirm={actions.confirmRemove}
        onOpenChange={open => {
          if (!open) actions.cancelRemove();
        }}
        open={actions.removing !== null}
        title={actions.removing ? `Remove ${actions.removing}?` : "Remove marketplace"}
        tone="danger"
      />
    </SettingsGroup>
  );
}
