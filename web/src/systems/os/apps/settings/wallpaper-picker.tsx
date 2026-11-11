import { useRef } from "react";

import { cn } from "@compozy/ui";

import type { OsWallpaper } from "../../lib/os-types";

const WALLPAPERS: Array<{ id: OsWallpaper; label: string }> = [
  { id: "flat", label: "Flat" },
  { id: "ember", label: "Ember" },
  { id: "mesh", label: "Mesh" },
  { id: "carbon", label: "Carbon" },
];

const THUMB_BACKGROUND: Record<OsWallpaper, string> = {
  flat: "var(--wallpaper-thumb-flat)",
  ember: "var(--wallpaper-thumb-ember)",
  mesh: "var(--wallpaper-thumb-mesh)",
  carbon: "var(--wallpaper-thumb-carbon)",
};

/**
 * Wallpaper choice as an APG radio group: one tab stop, arrows move AND
 * select (automatic activation — switching wallpapers is cheap and instant).
 */
export function WallpaperPicker({
  value,
  onChange,
}: {
  value: OsWallpaper;
  onChange: (wallpaper: OsWallpaper) => void;
}) {
  const groupRef = useRef<HTMLDivElement>(null);

  const moveSelection = (offset: number) => {
    const index = WALLPAPERS.findIndex(option => option.id === value);
    const next = WALLPAPERS[(index + offset + WALLPAPERS.length) % WALLPAPERS.length];
    onChange(next.id);
    groupRef.current
      ?.querySelector<HTMLButtonElement>(`[data-wallpaper-option="${next.id}"]`)
      ?.focus();
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      moveSelection(1);
    } else if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      moveSelection(-1);
    }
  };

  return (
    <div
      ref={groupRef}
      role="radiogroup"
      aria-label="Wallpaper"
      className="grid w-full grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3"
      onKeyDown={handleKeyDown}
    >
      {WALLPAPERS.map(option => {
        const selected = option.id === value;
        return (
          <button
            key={option.id}
            type="button"
            role="radio"
            aria-checked={selected}
            tabIndex={selected ? 0 : -1}
            data-wallpaper-option={option.id}
            data-testid={`os-wallpaper-option-${option.id}`}
            className={cn(
              "group flex w-full flex-col overflow-hidden rounded-lg border bg-card text-left",
              "transition-colors duration-base",
              "focus-visible:shadow-focus-ring focus-visible:outline-none",
              selected ? "border-fg" : "border-line hover:border-line-strong"
            )}
            onClick={() => onChange(option.id)}
          >
            <span
              aria-hidden="true"
              className="block aspect-video w-full border-b border-line"
              style={{ backgroundImage: THUMB_BACKGROUND[option.id] }}
            />
            <span
              className={cn(
                "flex items-center justify-between px-3 py-2 text-small-body",
                selected ? "font-medium text-fg" : "text-muted-foreground"
              )}
            >
              {option.label}
              {selected ? (
                <span aria-hidden="true" className="size-1.5 rounded-full bg-fg" />
              ) : null}
            </span>
          </button>
        );
      })}
    </div>
  );
}
