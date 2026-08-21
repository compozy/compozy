import type { CmdPaletteViewGrid } from "./cmd-palette-types";

export type PaletteGridTile = CmdPaletteViewGrid["sections"][number]["tiles"][number];
export type PaletteGridSection = CmdPaletteViewGrid["sections"][number];

export interface VisibleGridSection {
  readonly key: string;
  readonly title?: string;
  readonly tiles: readonly { tile: PaletteGridTile; index: number }[];
}

export interface VirtualGridRow {
  readonly key: string;
  readonly title?: string;
  readonly tiles: readonly { tile: PaletteGridTile; index: number }[];
}

function tileMatches(tile: PaletteGridTile, query: string, filterLocally: boolean): boolean {
  if (!filterLocally) return true;
  const needle = query.trim().toLocaleLowerCase();
  return needle === "" || tile.title.toLocaleLowerCase().includes(needle);
}

export function visibleGridTiles(
  grid: CmdPaletteViewGrid,
  query: string,
  filterLocally: boolean
): PaletteGridTile[] {
  const tiles: PaletteGridTile[] = [];
  for (const section of grid.sections) {
    for (const tile of section.tiles) {
      if (tileMatches(tile, query, filterLocally)) tiles.push(tile);
    }
  }
  return tiles;
}

export function visibleGridSections(
  sections: readonly PaletteGridSection[],
  query: string,
  filterLocally: boolean
): VisibleGridSection[] {
  const result: VisibleGridSection[] = [];
  let index = 0;
  for (const [sectionIndex, section] of sections.entries()) {
    const tiles: { tile: PaletteGridTile; index: number }[] = [];
    for (const tile of section.tiles) {
      if (!tileMatches(tile, query, filterLocally)) continue;
      tiles.push({ tile, index });
      index += 1;
    }
    result.push({
      key: `${sectionIndex}:${section.title ?? ""}`,
      ...(section.title ? { title: section.title } : {}),
      tiles,
    });
  }
  return result;
}

export function virtualGridRows(
  sections: readonly PaletteGridSection[],
  columns: number,
  tiles: readonly PaletteGridTile[]
): VirtualGridRow[] {
  const indexById = new Map(tiles.map((tile, index) => [tile.id, index] as const));
  const rows: VirtualGridRow[] = [];
  for (const [sectionIndex, section] of sections.entries()) {
    const visible = section.tiles.filter(tile => indexById.has(tile.id));
    for (let start = 0; start < visible.length; start += columns) {
      rows.push({
        key: `${sectionIndex}:${start}`,
        ...(start === 0 && section.title ? { title: section.title } : {}),
        tiles: visible.slice(start, start + columns).map(tile => ({
          tile,
          index: indexById.get(tile.id) ?? 0,
        })),
      });
    }
  }
  return rows;
}
