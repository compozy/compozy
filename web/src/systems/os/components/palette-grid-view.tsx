import { useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject } from "react";

import { Pill } from "@compozy/ui";
import { ImageIcon } from "lucide-react";

import { statusTone } from "@/lib/status-tone";
import { cn } from "@/lib/utils";

import { cmdPaletteIconRegistry } from "../lib/cmd-palette-icons";
import {
  visibleGridSections,
  visibleGridTiles,
  virtualGridRows,
  type PaletteGridTile,
} from "../lib/cmd-palette-grid";
import type { CmdPaletteViewAction, CmdPaletteViewGrid } from "../lib/cmd-palette-types";
import { OsPaletteProgramBand } from "./os-palette-program-status";
import { PALETTE_VIEW_VIRTUAL_THRESHOLD } from "./os-palette-virtual-rows-constants";
import { usePaletteGridVirtualizer } from "../hooks/use-palette-grid-virtualizer";

const GRID_VIEWPORT_CLASS =
  "max-h-72 overflow-y-auto p-3 outline-none focus-visible:shadow-focus-ring";

export function PaletteGridView({
  columns = 3,
  empty,
  filterLocally = false,
  grid,
  loading = false,
  onAction,
  onSelectionChange,
  query = "",
}: {
  columns?: number;
  empty?: { title: string; hint?: string } | null;
  filterLocally?: boolean;
  grid: CmdPaletteViewGrid;
  loading?: boolean;
  onAction: (action: CmdPaletteViewAction) => void;
  onSelectionChange?: (tileId: string) => void;
  query?: string;
}) {
  const [selected, setSelected] = useState(0);
  const [failedImages, setFailedImages] = useState<ReadonlySet<string>>(() => new Set());
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const tiles = visibleGridTiles(grid, query, filterLocally);
  const safeColumns = Math.max(1, Math.min(columns, 6));
  const virtualized = tiles.length > PALETTE_VIEW_VIRTUAL_THRESHOLD;
  const selectedId = tiles[selected]?.id ?? "";

  useLayoutEffect(() => {
    if (tiles.length === 0 || virtualized || selectedId === "") return;
    viewportRef.current
      ?.querySelector(`[data-testid="palette-grid-tile-${selectedId}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [selectedId, tiles.length, virtualized]);

  if (tiles.length === 0) {
    if (loading) {
      return <OsPaletteProgramBand phase="busy" onRetry={() => undefined} />;
    }
    return (
      <div className="px-3 py-8 text-center" data-testid="palette-grid-empty">
        <p className="text-card-title text-fg">{empty?.title ?? "No items yet"}</p>
        {empty?.hint ? <p className="mt-1 text-small-body text-muted">{empty.hint}</p> : null}
      </div>
    );
  }

  const select = (index: number) => {
    setSelected(index);
    const tile = tiles[index];
    if (tile) onSelectionChange?.(tile.id);
  };
  const activate = (tile: PaletteGridTile | undefined) => {
    const action = primaryAction(tile);
    if (action) onAction(action);
  };
  const keyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    let next = selected;
    switch (event.key) {
      case "ArrowRight":
        next = Math.min(selected + 1, tiles.length - 1);
        break;
      case "ArrowLeft":
        next = Math.max(selected - 1, 0);
        break;
      case "ArrowDown":
        next = Math.min(selected + safeColumns, tiles.length - 1);
        break;
      case "ArrowUp":
        next = Math.max(selected - safeColumns, 0);
        break;
      case "Enter":
        activate(tiles[selected]);
        break;
      default:
        return;
    }
    event.preventDefault();
    select(next);
  };

  if (virtualized) {
    return (
      <VirtualGrid
        columns={safeColumns}
        failedImages={failedImages}
        grid={grid}
        query={query}
        filterLocally={filterLocally}
        selected={selected}
        viewportRef={viewportRef}
        onAction={onAction}
        onActivate={activate}
        onImageError={tileID => setFailedImages(current => new Set([...current, tileID]))}
        onKeyDown={keyDown}
        onSelect={index => {
          select(index);
        }}
      />
    );
  }

  const sections = visibleGridSections(grid.sections, query, filterLocally);
  return (
    <GridViewport ref={viewportRef} onKeyDown={keyDown}>
      {sections.map(section => (
        <section key={section.key} className="mb-4 last:mb-0">
          {section.title ? <h3 className="eyebrow mb-2 text-muted">{section.title}</h3> : null}
          <div
            className="grid gap-2"
            style={{ gridTemplateColumns: `repeat(${safeColumns}, minmax(0, 1fr))` }}
          >
            {section.tiles.map(({ tile, index }) => (
              <GridTileButton
                key={tile.id}
                failed={failedImages.has(tile.id)}
                selected={selected === index}
                tile={tile}
                onAction={onAction}
                onClick={() => {
                  select(index);
                  activate(tile);
                }}
                onImageError={() => setFailedImages(current => new Set([...current, tile.id]))}
              />
            ))}
          </div>
        </section>
      ))}
    </GridViewport>
  );
}

function VirtualGrid({
  columns,
  failedImages,
  filterLocally,
  grid,
  query,
  selected,
  viewportRef,
  onAction,
  onActivate,
  onImageError,
  onKeyDown,
  onSelect,
}: {
  columns: number;
  failedImages: ReadonlySet<string>;
  filterLocally: boolean;
  grid: CmdPaletteViewGrid;
  query: string;
  selected: number;
  viewportRef: RefObject<HTMLDivElement | null>;
  onAction: (action: CmdPaletteViewAction) => void;
  onActivate: (tile: PaletteGridTile | undefined) => void;
  onImageError: (tileID: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
  onSelect: (index: number) => void;
}) {
  const tiles = visibleGridTiles(grid, query, filterLocally);
  const rows = virtualGridRows(grid.sections, columns, tiles);
  const virtualizer = usePaletteGridVirtualizer(rows, viewportRef);
  const selectedRow = rows.findIndex(row => row.tiles.some(entry => entry.index === selected));
  useLayoutEffect(() => {
    if (selectedRow >= 0) virtualizer.scrollToIndex(selectedRow, { align: "auto" });
  }, [selectedRow, virtualizer]);
  return (
    <GridViewport ref={viewportRef} virtualized onKeyDown={onKeyDown}>
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map(item => {
          const row = rows[item.index];
          if (!row) return null;
          return (
            <section
              key={row.key}
              ref={virtualizer.measureElement}
              className="absolute top-0 left-0 w-full pb-2"
              data-index={item.index}
              data-row-key={row.key}
              style={{ transform: `translateY(${item.start}px)` }}
            >
              {row.title ? <h3 className="eyebrow mb-2 text-muted">{row.title}</h3> : null}
              <div
                className="grid gap-2"
                style={{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }}
              >
                {row.tiles.map(({ tile, index }) => (
                  <GridTileButton
                    key={tile.id}
                    failed={failedImages.has(tile.id)}
                    selected={selected === index}
                    tile={tile}
                    onAction={onAction}
                    onClick={() => {
                      onSelect(index);
                      onActivate(tile);
                    }}
                    onImageError={() => onImageError(tile.id)}
                  />
                ))}
              </div>
            </section>
          );
        })}
      </div>
    </GridViewport>
  );
}

function GridViewport({
  children,
  ref,
  virtualized = false,
  onKeyDown,
}: {
  children: React.ReactNode;
  ref: RefObject<HTMLDivElement | null>;
  virtualized?: boolean;
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
}) {
  return (
    <div
      ref={ref}
      className={GRID_VIEWPORT_CLASS}
      data-testid="palette-grid-view"
      {...(virtualized ? { "data-virtualized": "true" } : {})}
      role="grid"
      tabIndex={0}
      onKeyDown={onKeyDown}
    >
      {children}
    </div>
  );
}

function GridTileButton({
  failed,
  onAction,
  selected,
  tile,
  onClick,
  onImageError,
}: {
  failed: boolean;
  onAction: (action: CmdPaletteViewAction) => void;
  selected: boolean;
  tile: PaletteGridTile;
  onClick: () => void;
  onImageError: () => void;
}) {
  const extras = secondaryActions(tile);
  return (
    <div
      aria-selected={selected}
      className={cn(
        "min-w-0 rounded-md border border-line bg-canvas-tint p-2",
        selected && "border-line-strong bg-elevated shadow-focus-ring"
      )}
      data-action-count={tile.actions?.length ?? 0}
      data-testid={`palette-grid-tile-${tile.id}`}
      role="gridcell"
    >
      <button
        type="button"
        className="w-full text-left outline-none"
        tabIndex={-1}
        onClick={onClick}
      >
        <TileImage failed={failed} tile={tile} onError={onImageError} />
        <div className="mt-2 flex min-w-0 items-center gap-2">
          <span className="min-w-0 flex-1 truncate text-card-title text-fg">{tile.title}</span>
          {tile.badge ? (
            <Pill size="xs" tone={statusTone(tile.badge.tone)}>
              {tile.badge.label}
            </Pill>
          ) : null}
        </div>
      </button>
      {extras.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-1">
          {extras.map(action => (
            <button
              key={action.title}
              type="button"
              className="rounded-sm border border-line px-1.5 py-0.5 text-micro text-muted outline-none hover:bg-elevated"
              onClick={() => onAction(action)}
            >
              {action.title}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function primaryAction(tile: PaletteGridTile | undefined): CmdPaletteViewAction | undefined {
  return tile?.actions?.find(action => action.primary) ?? tile?.actions?.[0];
}

function secondaryActions(tile: PaletteGridTile): readonly CmdPaletteViewAction[] {
  const actions = tile.actions ?? [];
  const primary = primaryAction(tile);
  return actions.filter(action => action !== primary);
}

function TileImage({
  failed,
  tile,
  onError,
}: {
  failed: boolean;
  tile: PaletteGridTile;
  onError: () => void;
}) {
  if (!failed && tile.image.url) {
    return (
      <img
        alt=""
        className="aspect-video w-full rounded-sm bg-canvas object-cover"
        src={tile.image.url}
        onError={onError}
      />
    );
  }
  const token = tile.image.token;
  const TokenIcon = token ? cmdPaletteIconRegistry[token] : undefined;
  return (
    <div className="grid aspect-video w-full place-items-center rounded-sm bg-canvas text-subtle">
      {tile.image.emoji && !failed ? (
        <span aria-hidden="true" className="text-title">
          {tile.image.emoji}
        </span>
      ) : TokenIcon && !failed ? (
        <TokenIcon aria-hidden="true" className="size-5" data-icon-token={token} />
      ) : (
        <ImageIcon aria-hidden="true" className="size-5" />
      )}
    </div>
  );
}
