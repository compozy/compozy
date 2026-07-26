import type { PixelRect } from "./window-manager-types";

export type SnapEdge = "left" | "right";
export type SnapCorner = "top-left" | "top-right" | "bottom-left" | "bottom-right";
export type SnapSide = "left" | "right" | "top" | "bottom";

export interface TileSnapTarget {
  kind: "tile";
  id: string;
  edge: SnapSide | SnapCorner;
  ratio: number;
  rect: PixelRect;
}

function finite(value: number, fallback = 0): number {
  return Number.isFinite(value) ? value : fallback;
}

export function progressiveSnapRatio(cycleStep: number, repeatRatios: readonly number[]): number {
  if (repeatRatios.length === 0) {
    throw new Error("Snap target config requires at least one repeat ratio.");
  }
  const firstRatio = repeatRatios[0];
  if (firstRatio === undefined) {
    throw new Error("Snap target config requires at least one repeat ratio.");
  }
  const index =
    ((Math.trunc(finite(cycleStep)) % repeatRatios.length) + repeatRatios.length) %
    repeatRatios.length;
  return repeatRatios[index] ?? firstRatio;
}

function roundedEdge(origin: number, span: number, ratio: number): number {
  return Math.round(origin + span * ratio);
}

function leadingSpan(origin: number, span: number, ratio: number): [number, number] {
  const edge = roundedEdge(origin, span, ratio);
  return [origin, edge];
}

function trailingSpan(origin: number, span: number, ratio: number): [number, number] {
  const end = origin + span;
  const edge = roundedEdge(origin, span, 1 - ratio);
  return [edge, end];
}

function sideRect(area: PixelRect, edge: SnapEdge, ratio: number): PixelRect {
  const [start, end] =
    edge === "left" ? leadingSpan(area.x, area.w, ratio) : trailingSpan(area.x, area.w, ratio);
  return { x: start, y: area.y, w: end - start, h: area.h };
}

function verticalRect(area: PixelRect, edge: "top" | "bottom", ratio: number): PixelRect {
  const [start, end] =
    edge === "top" ? leadingSpan(area.y, area.h, ratio) : trailingSpan(area.y, area.h, ratio);
  return { x: area.x, y: start, w: area.w, h: end - start };
}

function cornerRect(area: PixelRect, corner: SnapCorner): PixelRect {
  const horizontal = corner.endsWith("right") ? "right" : "left";
  const vertical = corner.startsWith("bottom") ? "bottom" : "top";
  const xRect = sideRect(area, horizontal, 0.5);
  const yRect = verticalRect(area, vertical, 0.5);
  return { x: xRect.x, y: yRect.y, w: xRect.w, h: yRect.h };
}

function isSnapCorner(edge: SnapSide | SnapCorner): edge is SnapCorner {
  return edge.includes("-");
}

export function createTileSnapTarget(
  area: PixelRect,
  edge: SnapSide | SnapCorner,
  repeatRatios: readonly number[],
  cycleStep = 0
): TileSnapTarget {
  if (isSnapCorner(edge)) {
    return { kind: "tile", id: `corner:${edge}`, edge, ratio: 0.5, rect: cornerRect(area, edge) };
  }
  const ratio = progressiveSnapRatio(cycleStep, repeatRatios);
  const rect =
    edge === "left" || edge === "right"
      ? sideRect(area, edge, ratio)
      : verticalRect(area, edge, ratio);
  return { kind: "tile", id: `edge:${edge}:${ratio}`, edge, ratio, rect };
}
