import { Boxes } from "lucide-react";

import { Button, cn, ListingRow, MonoId, Pill, Section, Spinner } from "@compozy/ui";

import type { ExtensionKitItem } from "../types";

interface ExtensionKitInventoryPanelProps {
  items: readonly ExtensionKitItem[] | undefined;
  error: Error | null;
  isLoading: boolean;
  onRetry: () => void;
  showEmptyState?: boolean;
  /**
   * Drop the section chrome and card frame so the panel can nest inside a
   * surface that already frames and labels itself.
   */
  bare?: boolean;
}

/**
 * Shipped-vs-live truth for one extension's static kit. `live` is the daemon's own word for a
 * resource that is currently published; everything else is declared in the package and inert, so
 * the two never render as the same thing.
 */
function ExtensionKitInventoryPanel({
  items,
  error,
  isLoading,
  onRetry,
  bare = false,
  showEmptyState = true,
}: ExtensionKitInventoryPanelProps) {
  const groups = groupByKind(items ?? []);
  const total = items?.length ?? 0;
  const body = (
    <div
      className={cn(
        "divide-y divide-line-soft",
        !bare && "overflow-hidden rounded-lg bg-canvas-soft"
      )}
      data-testid="extension-kit-inventory"
    >
      {isLoading ? (
        <div className="flex items-center gap-2 px-4 py-3 text-small-body text-muted" role="status">
          <Spinner className="size-3.5" />
          Loading kit inventory
        </div>
      ) : error ? (
        <div className="space-y-2 px-4 py-3">
          <p className="text-small-body font-medium text-danger">
            Kit inventory could not be loaded
          </p>
          <p className="text-xs text-muted">{error.message}</p>
          <Button onClick={onRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      ) : total === 0 && showEmptyState ? (
        <p className="px-4 py-3 text-small-body text-muted">
          This extension ships no static kit resources.
        </p>
      ) : total === 0 ? null : (
        groups.map(group => (
          <div key={group.kind}>
            <div className="flex items-center justify-between gap-2 px-4 pt-3 pb-1.5">
              <span className="eyebrow text-muted">{group.kind}</span>
              <span className="font-mono text-xs text-faint tabular-nums">
                {group.items.length}
              </span>
            </div>
            {group.items.map(item => (
              <ListingRow
                data-testid="extension-kit-inventory-item"
                interactive={false}
                key={`${item.kind}:${item.id}`}
              >
                <ListingRow.Icon>
                  <Boxes aria-hidden="true" className="size-4" />
                </ListingRow.Icon>
                <ListingRow.Main>
                  <ListingRow.Name mono>
                    <ListingRow.Title>{item.name}</ListingRow.Title>
                  </ListingRow.Name>
                  <ListingRow.Meta>
                    <MonoId value={item.id} />
                  </ListingRow.Meta>
                </ListingRow.Main>
                <ListingRow.Trail>
                  <Pill mono size="xs" tone={item.live ? "success" : "neutral"}>
                    {item.live ? "live" : "shipped"}
                  </Pill>
                </ListingRow.Trail>
              </ListingRow>
            ))}
          </div>
        ))
      )}
    </div>
  );
  if (bare) return body;
  return (
    <Section count={isLoading || error ? undefined : total} label="Kit inventory">
      {body}
    </Section>
  );
}

interface ExtensionKitInventoryGroup {
  kind: string;
  items: ExtensionKitItem[];
}

/** Stable, name-sorted grouping so a reorder from the daemon never reshuffles the panel. */
function groupByKind(items: readonly ExtensionKitItem[]): ExtensionKitInventoryGroup[] {
  const byKind = new Map<string, ExtensionKitItem[]>();
  for (const item of items) {
    const bucket = byKind.get(item.kind);
    if (bucket) bucket.push(item);
    else byKind.set(item.kind, [item]);
  }
  return [...byKind.entries()]
    .map(([kind, group]) => ({
      kind,
      items: [...group].sort((left, right) => left.name.localeCompare(right.name)),
    }))
    .sort((left, right) => left.kind.localeCompare(right.kind));
}

export { ExtensionKitInventoryPanel };
export type { ExtensionKitInventoryPanelProps };
