import { Pill } from "@compozy/ui";

import type { SnapTarget } from "../lib/snap-targets";

function previewLabel(kind: SnapTarget["kind"]): string {
  switch (kind) {
    case "tile":
      return "Tile";
    case "zoom":
      return "Zoom";
    case "insert":
      return "Insert";
    case "split":
      return "Split";
    case "swap":
      return "Swap windows";
  }
}

/**
 * One ephemeral structural preview; it never mutates the authoritative
 * snapshot. Flat like the tiles it previews: an accent hairline over an accent
 * tint, labelled with the solid accent pill.
 */
export function OsSnapOverlay({ preview }: { preview: SnapTarget | null }) {
  if (preview === null) return null;
  return (
    <div
      aria-hidden="true"
      data-slot="window-manager-command-preview"
      className="pointer-events-none absolute z-50 border border-accent bg-accent-tint"
      style={{
        left: preview.rect.x,
        top: preview.rect.y,
        width: preview.rect.w,
        height: preview.rect.h,
      }}
    >
      <Pill tone="accent" solid size="sm" className="absolute top-2 left-2">
        {previewLabel(preview.kind)}
      </Pill>
    </div>
  );
}
