import { useRef } from "react";

import { Field, FieldDescription, FieldLabel, FieldTitle, PageShell, Switch, cn } from "@agh/ui";

import { useSettingsTopbar } from "@/systems/settings";

import { useAppearanceSettingsPane } from "../../hooks/use-appearance-settings-pane";
import type { OsWallpaper } from "../../lib/os-types";

const WALLPAPERS: Array<{ id: OsWallpaper; label: string }> = [
  { id: "ember", label: "Ember" },
  { id: "mesh", label: "Mesh" },
  { id: "carbon", label: "Carbon" },
];

const THUMB_BACKGROUND: Record<OsWallpaper, string> = {
  ember: "var(--wallpaper-thumb-ember)",
  mesh: "var(--wallpaper-thumb-mesh)",
  carbon: "var(--wallpaper-thumb-carbon)",
};

/**
 * Wallpaper choice as an APG radio group: one tab stop, arrows move AND
 * select (automatic activation — switching wallpapers is cheap and instant).
 */
function WallpaperPicker({
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
              "group flex w-full flex-col overflow-hidden rounded-lg border text-left",
              "transition-colors duration-base",
              "focus-visible:shadow-focus-ring focus-visible:outline-none",
              selected ? "border-accent" : "border-line-strong hover:border-line-focus"
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
                selected ? "font-semibold text-fg-strong" : "text-muted"
              )}
            >
              {option.label}
              {selected ? (
                <span aria-hidden="true" className="size-1.5 rounded-full bg-accent" />
              ) : null}
            </span>
          </button>
        );
      })}
    </div>
  );
}

/**
 * The Appearance pane (US-015): shell-session wallpaper, Dock magnification,
 * and the in-product reduced-motion preference. The system reduced-motion
 * preference always wins over the toggle (US-015.EC-1).
 */
export function AppearanceSettingsPane() {
  useSettingsTopbar("appearance");
  const appearance = useAppearanceSettingsPane();

  return (
    <PageShell slug="appearance">
      <div className="flex max-w-2xl flex-col gap-8" data-testid="os-appearance-pane">
        <Field>
          <FieldTitle>Wallpaper</FieldTitle>
          <FieldDescription>Backdrop for the current shell session.</FieldDescription>
          <WallpaperPicker value={appearance.wallpaper} onChange={appearance.setWallpaper} />
        </Field>
        <Field>
          <FieldLabel htmlFor="os-appearance-magnify">Dock magnification</FieldLabel>
          <FieldDescription>Dock icons lift and scale under the pointer.</FieldDescription>
          <div className="min-w-0">
            <Switch
              id="os-appearance-magnify"
              data-testid="os-appearance-magnify"
              checked={appearance.dockMagnify}
              onCheckedChange={checked => appearance.setDockMagnify(checked === true)}
            />
          </div>
        </Field>
        <Field>
          <FieldLabel htmlFor="os-appearance-reduce-motion">Reduce motion</FieldLabel>
          <FieldDescription>
            {appearance.systemReducedMotion
              ? "Your system already prefers reduced motion — that preference wins while it is on."
              : "Window, dock, and minimize animations become instant."}
          </FieldDescription>
          <div className="min-w-0">
            <Switch
              id="os-appearance-reduce-motion"
              data-testid="os-appearance-reduce-motion"
              checked={appearance.reduceMotion}
              onCheckedChange={checked => appearance.setReduceMotion(checked === true)}
            />
          </div>
        </Field>
      </div>
    </PageShell>
  );
}
