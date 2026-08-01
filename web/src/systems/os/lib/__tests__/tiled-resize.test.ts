// Suite: tiled free-edge resize geometry
// Invariant: a tiled unit's free edges are exactly the sides no other unit abuts, and an
// edge/corner drag clamps to the nearest obstacle, the layout area, and the unit's minimums.
// Boundary IN: pure zone geometry from the layout projection.
// Boundary OUT: rnd gesture wiring and the daemon window.resize command.
// New standalone suite: tiled-resize is a new pure module no existing suite owns.
import { describe, expect, it } from "vitest";

import {
  frameResizableEdges,
  resizedTiledZone,
  tiledZoneLimits,
  zoneOverlapsAny,
} from "../tiled-resize";
import type { NormalizedRect } from "../window-manager-types";

const LEFT_HALF: NormalizedRect = { x: 0, y: 0, w: 0.5, h: 1 };
const RIGHT_HALF: NormalizedRect = { x: 0.5, y: 0, w: 0.5, h: 1 };
const TOP_RIGHT: NormalizedRect = { x: 0.5, y: 0, w: 0.5, h: 0.5 };

describe("frameResizableEdges", () => {
  it("Should free every edge of a solo island and block only abutting sides", () => {
    expect(frameResizableEdges(LEFT_HALF, [])).toEqual({
      left: true,
      right: true,
      top: true,
      bottom: true,
    });
    expect(frameResizableEdges(LEFT_HALF, [RIGHT_HALF])).toEqual({
      left: true,
      right: false,
      top: true,
      bottom: true,
    });
    expect(frameResizableEdges(RIGHT_HALF, [LEFT_HALF])).toEqual({
      left: false,
      right: true,
      top: true,
      bottom: true,
    });
  });

  it("Should ignore neighbours whose spans only corner-touch the edge", () => {
    const bottomRight: NormalizedRect = { x: 0.5, y: 0.5, w: 0.5, h: 0.5 };
    const topLeft: NormalizedRect = { x: 0, y: 0, w: 0.5, h: 0.5 };
    expect(frameResizableEdges(topLeft, [bottomRight])).toEqual({
      left: true,
      right: true,
      top: true,
      bottom: true,
    });
    expect(frameResizableEdges(topLeft, [TOP_RIGHT])).toEqual({
      left: true,
      right: false,
      top: true,
      bottom: true,
    });
  });
});

describe("resizedTiledZone", () => {
  it("Should shrink a free edge and leave the opposite edge anchored", () => {
    const next = resizedTiledZone({
      zone: LEFT_HALF,
      direction: "bottom",
      deltaW: 0,
      deltaH: -0.4,
      limits: tiledZoneLimits(LEFT_HALF, [RIGHT_HALF]),
      minSize: { w: 0.1, h: 0.1 },
    });
    expect(next).toEqual({ x: 0, y: 0, w: 0.5, h: 0.6 });
  });

  it("Should clamp growth at the nearest obstacle and shrink at the minimum", () => {
    const obstacle: NormalizedRect = { x: 0.7, y: 0, w: 0.3, h: 1 };
    const grown = resizedTiledZone({
      zone: LEFT_HALF,
      direction: "right",
      deltaW: 0.4,
      deltaH: 0,
      limits: tiledZoneLimits(LEFT_HALF, [obstacle]),
      minSize: { w: 0.1, h: 0.1 },
    });
    expect(grown).toEqual({ x: 0, y: 0, w: 0.7, h: 1 });

    const floored = resizedTiledZone({
      zone: LEFT_HALF,
      direction: "top",
      deltaW: 0,
      deltaH: -0.95,
      limits: tiledZoneLimits(LEFT_HALF, []),
      minSize: { w: 0.1, h: 0.2 },
    });
    expect(floored?.x).toBe(0);
    expect(floored?.w).toBe(0.5);
    expect(floored?.y).toBeCloseTo(0.8, 10);
    expect(floored?.h).toBeCloseTo(0.2, 10);
  });

  it("Should report no-ops as null and flag diagonal overlap", () => {
    expect(
      resizedTiledZone({
        zone: LEFT_HALF,
        direction: "right",
        deltaW: 0,
        deltaH: 0,
        limits: tiledZoneLimits(LEFT_HALF, []),
        minSize: { w: 0.1, h: 0.1 },
      })
    ).toBeNull();
    expect(zoneOverlapsAny({ x: 0, y: 0, w: 0.6, h: 1 }, [RIGHT_HALF])).toBe(true);
    expect(zoneOverlapsAny(LEFT_HALF, [RIGHT_HALF])).toBe(false);
  });
});
