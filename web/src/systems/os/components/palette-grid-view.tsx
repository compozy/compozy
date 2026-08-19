import { useRef, useState, type KeyboardEvent } from "react";

import { Pill } from "@compozy/ui";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ImageIcon } from "lucide-react";

import { statusTone } from "@/lib/status-tone";
import { cn } from "@/lib/utils";

import type { CmdPaletteViewAction, CmdPaletteViewGrid } from "../lib/cmd-palette-types";

type GridTile = CmdPaletteViewGrid["sections"][number]["tiles"][number];
type GridSection = CmdPaletteViewGrid["sections"][number];

const GRID_VIRTUAL_THRESHOLD = 150;
const GRID_ROW_ESTIMATE = 156;

export function PaletteGridView({
  columns = 3,
  empty,
  grid,
  onAction,
}: {
  columns?: number;
  empty?: { title: string; hint?: string } | null;
  grid: CmdPaletteViewGrid;
  onAction: (action: CmdPaletteViewAction) => void;
}) {
  const [selected, setSelected] = useState(0);
  const [failedImages, setFailedImages] = useState<ReadonlySet<string>>(() => new Set());
  const tiles = grid.sections.flatMap(section => section.tiles);
  const safeColumns = Math.max(1, Math.min(columns, 6));
  if (tiles.length === 0) {
    return (
      <div className="px-3 py-8 text-center" data-testid="palette-grid-empty">
        <p className="text-card-title text-fg">{empty?.title ?? "No items yet"}</p>
        {empty?.hint ? <p className="mt-1 text-small-body text-muted">{empty.hint}</p> : null}
      </div>
    );
  }

  const activate = (tile: GridTile | undefined) => {
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
    setSelected(next);
  };

  if (tiles.length > GRID_VIRTUAL_THRESHOLD) {
    return (
      <VirtualGrid
        columns={safeColumns}
        failedImages={failedImages}
        grid={grid}
        selected={selected}
        onActivate={activate}
        onImageError={tileID => setFailedImages(current => new Set([...current, tileID]))}
        onKeyDown={keyDown}
        onSelect={setSelected}
      />
    );
  }

  return (
    <div
      className="max-h-72 overflow-y-auto p-3 outline-none focus-visible:shadow-focus-ring"
      data-testid="palette-grid-view"
      role="grid"
      tabIndex={0}
      onKeyDown={keyDown}
    >
      {grid.sections.map((section, sectionIndex) => {
        const sectionOffset = grid.sections
          .slice(0, sectionIndex)
          .reduce((total, prior) => total + prior.tiles.length, 0);
        return (
          <section
            key={`${section.title ?? "section"}:${sectionOffset}`}
            className="mb-4 last:mb-0"
          >
            {section.title ? <h3 className="eyebrow mb-2 text-muted">{section.title}</h3> : null}
            <div
              className="grid gap-2"
              style={{ gridTemplateColumns: `repeat(${safeColumns}, minmax(0, 1fr))` }}
            >
              {section.tiles.map((tile, tileIndex) => {
                const index = sectionOffset + tileIndex;
                return (
                  <GridTileButton
                    key={tile.id}
                    failed={failedImages.has(tile.id)}
                    selected={selected === index}
                    tile={tile}
                    onClick={() => {
                      setSelected(index);
                      activate(tile);
                    }}
                    onImageError={() => setFailedImages(current => new Set([...current, tile.id]))}
                  />
                );
              })}
            </div>
          </section>
        );
      })}
    </div>
  );
}

interface VirtualGridRow {
  readonly key: string;
  readonly title?: string;
  readonly tiles: readonly { tile: GridTile; index: number }[];
}

function virtualGridRows(sections: readonly GridSection[], columns: number): VirtualGridRow[] {
  const rows: VirtualGridRow[] = [];
  let offset = 0;
  for (const section of sections) {
    for (let start = 0; start < section.tiles.length; start += columns) {
      rows.push({
        key: `${section.title ?? "section"}:${start}`,
        ...(start === 0 && section.title ? { title: section.title } : {}),
        tiles: section.tiles.slice(start, start + columns).map((tile, localIndex) => ({
          tile,
          index: offset + start + localIndex,
        })),
      });
    }
    offset += section.tiles.length;
  }
  return rows;
}

function VirtualGrid({
  columns,
  failedImages,
  grid,
  selected,
  onActivate,
  onImageError,
  onKeyDown,
  onSelect,
}: {
  columns: number;
  failedImages: ReadonlySet<string>;
  grid: CmdPaletteViewGrid;
  selected: number;
  onActivate: (tile: GridTile | undefined) => void;
  onImageError: (tileID: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
  onSelect: (index: number) => void;
}) {
  "use no memo";

  const viewportRef = useRef<HTMLDivElement | null>(null);
  const rows = virtualGridRows(grid.sections, columns);
  // oxlint-disable-next-line react/incompatible-library -- virtualizer state is isolated inside this compiler boundary.
  const virtualizer = useVirtualizer<HTMLDivElement, HTMLDivElement>({
    count: rows.length,
    getScrollElement: () => viewportRef.current,
    getItemKey: index => rows[index]?.key ?? index,
    estimateSize: () => GRID_ROW_ESTIMATE,
    overscan: 4,
    useFlushSync: false,
  });
  return (
    <div
      ref={viewportRef}
      className="max-h-72 overflow-y-auto p-3 outline-none focus-visible:shadow-focus-ring"
      data-testid="palette-grid-view"
      data-virtualized="true"
      role="grid"
      tabIndex={0}
      onKeyDown={onKeyDown}
    >
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
    </div>
  );
}

function GridTileButton({
  failed,
  selected,
  tile,
  onClick,
  onImageError,
}: {
  failed: boolean;
  selected: boolean;
  tile: GridTile;
  onClick: () => void;
  onImageError: () => void;
}) {
  return (
    <button
      type="button"
      aria-selected={selected}
      className={cn(
        "min-w-0 rounded-md border border-line bg-canvas-tint p-2 text-left outline-none",
        "hover:bg-elevated",
        selected && "border-line-strong bg-elevated shadow-focus-ring"
      )}
      data-action-count={tile.actions?.length ?? 0}
      data-testid={`palette-grid-tile-${tile.id}`}
      role="gridcell"
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
  );
}

function primaryAction(tile: GridTile | undefined): CmdPaletteViewAction | undefined {
  return tile?.actions?.find(action => action.primary) ?? tile?.actions?.[0];
}

function TileImage({
  failed,
  tile,
  onError,
}: {
  failed: boolean;
  tile: GridTile;
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
  return (
    <div className="grid aspect-video w-full place-items-center rounded-sm bg-canvas text-subtle">
      {tile.image.emoji && !failed ? (
        <span aria-hidden="true" className="text-title">
          {tile.image.emoji}
        </span>
      ) : (
        <ImageIcon aria-hidden="true" className="size-5" />
      )}
    </div>
  );
}
