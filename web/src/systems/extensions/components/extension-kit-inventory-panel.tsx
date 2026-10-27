import { Boxes } from "lucide-react";

import {
  Button,
  cn,
  ListingRow,
  ListingRowSkeleton,
  MonoId,
  Pill,
  Section,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

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

function KitInventoryItemRow({ item }: { item: ExtensionKitItem }) {
  return (
    <ListingRow data-testid="extension-kit-inventory-item" interactive={false}>
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
        <Pill form="plain" mono tone={item.live ? "success" : "neutral"}>
          <Pill.Dot />
          {item.live ? "live" : "shipped"}
        </Pill>
      </ListingRow.Trail>
    </ListingRow>
  );
}

function KitInventoryGroup({ group }: { group: ExtensionKitInventoryGroup }) {
  return (
    <div>
      <div className="flex items-center justify-between gap-2 px-4 pt-3 pb-1.5">
        <span className="eyebrow text-muted">{group.kind}</span>
        <span className="font-mono text-eyebrow text-faint tabular-nums">{group.items.length}</span>
      </div>
      {group.items.map(item => (
        <KitInventoryItemRow item={item} key={`${item.kind}:${item.id}`} />
      ))}
    </div>
  );
}

/** One group of placeholder rows: the group head's kind and count, then the real row layout. */
function KitInventorySkeleton() {
  return (
    <SkeletonRegion label="Loading kit inventory">
      <div className="flex items-center justify-between gap-2 px-4 pt-3 pb-1.5">
        <span className="eyebrow">
          <SkeletonText className="w-16" />
        </span>
        <span className="font-mono text-eyebrow">
          <SkeletonText className="w-4" />
        </span>
      </div>
      <ListingRowSkeleton count={3} trail="pill" />
    </SkeletonRegion>
  );
}

function KitInventoryContent({
  items,
  error,
  isLoading,
  onRetry,
  showEmptyState,
}: Omit<ExtensionKitInventoryPanelProps, "bare" | "showEmptyState"> & {
  showEmptyState: boolean;
}) {
  if (isLoading) {
    return <KitInventorySkeleton />;
  }
  if (error) {
    return (
      <div className="space-y-2 px-4 py-3">
        <p className="text-small-body font-medium text-danger">Kit inventory could not be loaded</p>
        <p className="text-eyebrow text-muted">{error.message}</p>
        <Button onClick={onRetry} size="sm" type="button" variant="secondary">
          Retry
        </Button>
      </div>
    );
  }
  if (!items || items.length === 0) {
    return showEmptyState ? (
      <p className="px-4 py-3 text-small-body text-muted">
        This extension ships no static kit resources.
      </p>
    ) : null;
  }
  return groupByKind(items).map(group => <KitInventoryGroup group={group} key={group.kind} />);
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
  const total = items?.length ?? 0;
  const body = (
    <div
      className={cn("divide-y divide-line-soft", !bare && "overflow-hidden rounded-lg bg-sunken")}
      data-testid="extension-kit-inventory"
    >
      <KitInventoryContent
        error={error}
        items={items}
        isLoading={isLoading}
        onRetry={onRetry}
        showEmptyState={showEmptyState}
      />
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
