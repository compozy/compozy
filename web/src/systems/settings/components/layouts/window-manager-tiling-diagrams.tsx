import type { WindowPlacementId } from "@/systems/os";

const PLACEMENT: Record<WindowPlacementId, readonly [number, number, number, number]> = {
  left: [0, 0, 11, 16],
  right: [11, 0, 11, 16],
  top: [0, 0, 22, 8],
  bottom: [0, 8, 22, 8],
  "top-left": [0, 0, 11, 8],
  "top-right": [11, 0, 11, 8],
  "bottom-left": [0, 8, 11, 8],
  "bottom-right": [11, 8, 11, 8],
};

/** Which half or quarter of the screen a tiling shortcut claims. */
export function WindowTilingDiagram({ placement }: { placement: WindowPlacementId }) {
  const [x, y, width, height] = PLACEMENT[placement];

  return (
    <svg aria-hidden="true" className="h-4 w-6 text-subtle" fill="none" viewBox="0 0 24 18">
      <rect height="16" rx="2" stroke="currentColor" strokeOpacity="0.38" width="22" x="1" y="1" />
      <rect
        fill="currentColor"
        fillOpacity="0.55"
        height={height}
        rx="1.4"
        width={width}
        x={x + 1}
        y={y + 1}
      />
    </svg>
  );
}
