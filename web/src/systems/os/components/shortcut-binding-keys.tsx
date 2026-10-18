import { Fragment } from "react";

import { Kbd, KbdGroup, cn } from "@compozy/ui";

import { primaryShortcutModifier, shortcutLabel } from "../lib/window-manager-shortcuts";

export interface ShortcutBindingKeysProps {
  bindings: readonly string[];
  overridden?: boolean;
  compact?: boolean;
  className?: string;
}

/** Shared chord glyph renderer for Settings, the cheatsheet, and shell labels. */
export function ShortcutBindingKeys({
  bindings,
  overridden = false,
  compact = false,
  className,
}: ShortcutBindingKeysProps) {
  const primaryModifier = primaryShortcutModifier(
    typeof navigator === "undefined" ? "" : navigator.platform
  );
  if (bindings.length === 0) {
    return <span className={cn("font-mono text-micro text-faint", className)}>—</span>;
  }
  return (
    <span className={cn("inline-flex flex-wrap items-center justify-end gap-1.5", className)}>
      {overridden ? (
        // An override keeps the keys' own ink; one quiet dot marks it.
        <span
          aria-label="Overridden shortcut"
          className="size-1.5 shrink-0 rounded-full bg-fg-2"
          data-slot="shortcut-override-marker"
          role="img"
        />
      ) : null}
      <KbdGroup>
        {bindings.map((binding, index) => (
          <Fragment key={binding}>
            {index > 0 ? (
              // Alternates read as "or": the separator sits between caps, never inside one.
              <span aria-hidden="true" className="text-faint">
                /
              </span>
            ) : null}
            <Kbd className={cn(compact && "px-1")} data-alternate={index > 0 ? "true" : undefined}>
              {shortcutLabel(binding, primaryModifier)}
            </Kbd>
          </Fragment>
        ))}
      </KbdGroup>
    </span>
  );
}
