import { Trash2 } from "lucide-react";

import { Button, Pill, cn } from "@agh/ui";

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
 * One saved layout. The thumbnail is the record's own geometry, so cards are
 * told apart by shape rather than by reading four names in a row.
 */
export function LayoutProfileCard({
  record,
  selected,
  onSelect,
  onLoad,
  onDelete,
}: LayoutProfileCardProps) {
  const slots = record.spec.participantSlots.length;

  return (
    <div
      className={cn(
        "flex flex-col overflow-hidden rounded-lg border border-line bg-canvas-soft",
        "transition-colors duration-base ease-out hover:border-line-strong",
        selected && "border-accent-dim bg-accent-tint"
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
          <span className="block truncate text-small-body font-semibold text-fg-strong">
            {record.spec.displayName}
          </span>
          <span className="mt-0.5 block truncate font-mono text-mono-id text-faint">
            {record.id}
          </span>
          <span className="mt-2 flex flex-wrap items-center gap-1.5">
            <Pill size="xs" tone={record.scope.kind === "global" ? "info" : "neutral"}>
              {record.scope.kind === "global" ? "Every workspace" : "This workspace"}
            </Pill>
            {record.spec.aspectVariant === "any" ? null : (
              <Pill size="xs" tone="neutral">
                {record.spec.aspectVariant === "landscape" ? "Landscape" : "Portrait"}
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
          variant="outline"
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
  const thumbnail = layoutThumbnail(record.spec.document);

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
        <g key={`${tile.x}:${tile.y}:${tile.w}:${tile.h}`}>
          {tile.stacked ? (
            <rect
              className="fill-surface-glaze stroke-line-strong"
              height={tile.h}
              rx="1.6"
              strokeWidth="0.8"
              width={Math.max(0, tile.w - 2.4)}
              x={tile.x + 2.4}
              y={Math.max(0, tile.y - 1.6)}
            />
          ) : null}
          <rect
            className="fill-bar-fill stroke-line-focus"
            height={tile.h}
            rx="1.6"
            strokeWidth="0.8"
            width={tile.w}
            x={tile.x}
            y={tile.y}
          />
        </g>
      ))}
    </svg>
  );
}
