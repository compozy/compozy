import { PillGroup, type PillGroupItem } from "@compozy/ui";

import { shortcutSourceLabel } from "@/systems/os";

export const SHORTCUT_SOURCE_ALL = "__all__";

export interface ShortcutSourceFilterProps {
  /** Sources present in the registry, in reading order. */
  sources: readonly string[];
  counts: ReadonlyMap<string, number>;
  selected: string;
  onSelect: (source: string) => void;
}

/**
 * Narrows the table to one contribution source (S12).
 *
 * Options come from the registry's own source list, so an extension appears the
 * moment it contributes commands and is gone when it is disabled — there is no
 * second list of known sources to keep in step. It hides itself while core is
 * the only contributor: a single-choice filter is chrome, not a control.
 */
export function ShortcutSourceFilter({
  sources,
  counts,
  selected,
  onSelect,
}: ShortcutSourceFilterProps) {
  if (sources.length <= 1) return null;
  const total = [...counts.values()].reduce((sum, count) => sum + count, 0);
  const items: PillGroupItem[] = [
    {
      value: SHORTCUT_SOURCE_ALL,
      label: "All sources",
      badge: total,
      testId: `shortcut-source-${SHORTCUT_SOURCE_ALL}`,
    },
    ...sources.map(source => ({
      value: source,
      label: shortcutSourceLabel(source),
      badge: counts.get(source) ?? 0,
      testId: `shortcut-source-${source}`,
    })),
  ];
  return (
    <PillGroup
      aria-label="Filter shortcuts by source"
      data-testid="shortcut-source-filter"
      items={items}
      onChange={onSelect}
      size="sm"
      value={selected}
    />
  );
}
