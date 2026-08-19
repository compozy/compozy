import { Pill, cn } from "@compozy/ui";

export const SHORTCUT_SOURCE_ALL = "__all__";

export interface ShortcutSourceOption {
  /** `core` or `ext.<name>`, exactly as the registry reports it. */
  id: string;
  label: string;
  count: number;
}

export interface ShortcutSourceFilterProps {
  options: readonly ShortcutSourceOption[];
  selected: string;
  onSelect: (source: string) => void;
}

/**
 * Narrows the shortcut table to one contribution source (S12).
 *
 * The options come from the registry's own source list, so an extension appears
 * here the moment it contributes commands and disappears when it is disabled —
 * there is no separate list of known sources to keep in step.
 */
export function ShortcutSourceFilter({ options, selected, onSelect }: ShortcutSourceFilterProps) {
  if (options.length <= 1) return null;
  const total = options.reduce((sum, option) => sum + option.count, 0);
  const entries = [{ id: SHORTCUT_SOURCE_ALL, label: "All", count: total }, ...options];
  return (
    <div
      aria-label="Filter shortcuts by source"
      className="flex flex-wrap items-center gap-1.5 border-b border-line-soft px-4 py-2.5"
      data-testid="shortcut-source-filter"
      role="group"
    >
      {entries.map(entry => (
        <button
          aria-pressed={entry.id === selected}
          className={cn(
            "rounded-full transition-colors duration-base ease-out",
            "focus-visible:outline-none focus-visible:shadow-focus-ring"
          )}
          data-testid={`shortcut-source-${entry.id}`}
          key={entry.id}
          type="button"
          onClick={() => onSelect(entry.id)}
        >
          <Pill size="sm" tone={entry.id === selected ? "accent" : "neutral"}>
            {entry.label}
            <span className="ml-1 font-mono text-badge text-faint">{entry.count}</span>
          </Pill>
        </button>
      ))}
    </div>
  );
}
