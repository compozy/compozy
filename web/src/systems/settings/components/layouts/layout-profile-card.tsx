import { Trash2 } from "@compozy/ui/icons";

import { Button, Pill, cn } from "@compozy/ui";

import { layoutThumbnail } from "../../lib/window-manager-layout-thumbnail";
import type { WindowManagerLayoutResourceRecord } from "../../lib/window-manager-layout-types";

interface LayoutProfileCardProps {
  record: WindowManagerLayoutResourceRecord;
  selected: boolean;
  onSelect: () => void;
  onLoad: () => void;
  onDelete: () => void;
}

/**
 * One saved layout. The thumbnail is the default tab of its first desktop, so
 * cards are told apart by shape rather than by reading four names in a row.
 */
export function LayoutProfileCard({
  record,
  selected,
  onSelect,
  onLoad,
  onDelete,
}: LayoutProfileCardProps) {
  const slots = record.spec.participantSlots.length;
  const tabs =
    record.spec.source.kind === "current"
      ? layoutThumbnail(record.spec.source.document).tabCount
      : null;

  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden rounded-lg bg-card shadow-card",
        "transition-colors duration-base ease-out",
        selected ? "bg-selected" : "hover:bg-surface-2"
      )}
      data-selected={selected ? "true" : undefined}
      data-testid={`layout-profile-card-${record.id}`}
    >
      <button
        aria-pressed={selected}
        className="flex min-h-0 flex-1 flex-col text-left focus-visible:outline-none focus-visible:shadow-focus-ring"
        type="button"
        onClick={onSelect}
      >
        <span className="block h-23 border-b border-line-soft p-2.5">
          <LayoutProfileThumbnail record={record} />
        </span>
        <span className="block px-3 pt-2.5 pb-3">
          <span className="block truncate text-body font-medium text-fg">
            {record.spec.displayName}
          </span>
          <span className="mt-0.5 block truncate font-mono text-mono-id text-faint">
            {record.id}
          </span>
          <span className="mt-2 flex flex-wrap items-center gap-1.5">
            <Pill size="xs" tone={record.scope.kind === "user" ? "info" : "neutral"}>
              {record.scope.kind === "user" ? "Every project" : "This project"}
            </Pill>
            {record.spec.aspectVariant === "any" ? null : (
              <Pill size="xs" tone="neutral">
                {record.spec.aspectVariant === "landscape" ? "Landscape" : "Portrait"}
              </Pill>
            )}
            {tabs === null ? null : (
              <Pill size="xs" tone="neutral">
                {tabs} tab{tabs === 1 ? "" : "s"}
              </Pill>
            )}
            <Pill size="xs" tone="neutral">
              {slots} window{slots === 1 ? "" : "s"}
            </Pill>
          </span>
        </span>
      </button>
      <div className="flex items-center gap-1.5 border-t border-line-soft px-2.5 py-2">
        <Button
          data-testid={`layout-profile-load-${record.id}`}
          size="xs"
          type="button"
          variant="secondary"
          onClick={onLoad}
        >
          Load
        </Button>
        <span className="flex-1" />
        <Button
          aria-label={`Delete ${record.spec.displayName}`}
          data-testid={`layout-profile-delete-${record.id}`}
          size="icon-xs"
          type="button"
          variant="ghost"
          onClick={onDelete}
        >
          <Trash2 aria-hidden="true" className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

function LayoutProfileThumbnail({ record }: { record: WindowManagerLayoutResourceRecord }) {
  const source = record.spec.source;
  if (source.kind === "earlier") {
    // Saved before tabs and splits: the daemon converts it when it is loaded,
    // and the card does not guess at the result.
    return (
      <span
        className="flex h-full flex-col items-center justify-center gap-0.5 text-center text-form-hint text-faint"
        data-testid={`layout-profile-earlier-${record.id}`}
      >
        <span>Saved before tabs</span>
        <span>Opens as tabs when loaded</span>
      </span>
    );
  }
  const thumbnail = layoutThumbnail(source.document);

  if (thumbnail.tiles.length === 0) {
    return (
      <span className="flex h-full items-center justify-center text-form-hint text-faint">
        Empty layout
      </span>
    );
  }

  return (
    <svg
      aria-hidden="true"
      className="h-full w-full"
      preserveAspectRatio="none"
      viewBox={thumbnail.viewBox}
    >
      {thumbnail.tiles.map(tile => (
        <rect
          className="fill-bar-fill stroke-line-focus"
          height={tile.h}
          key={`${tile.x}:${tile.y}:${tile.w}:${tile.h}`}
          rx="1.6"
          strokeWidth="0.8"
          width={tile.w}
          x={tile.x}
          y={tile.y}
        />
      ))}
    </svg>
  );
}
