import { Kbd, cn } from "@compozy/ui";

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
    <span className={cn("inline-flex flex-wrap items-center justify-end gap-1", className)}>
      {bindings.map((binding, index) => (
        <Kbd
          className={cn(
            "border border-line bg-canvas",
            index > 0 && "border-dashed border-line-strong bg-transparent",
            overridden && "border-accent-dim text-accent-strong",
            compact && "px-1"
          )}
          data-alternate={index > 0 ? "true" : undefined}
          key={binding}
        >
          {shortcutLabel(binding, primaryModifier)}
        </Kbd>
      ))}
    </span>
  );
}
