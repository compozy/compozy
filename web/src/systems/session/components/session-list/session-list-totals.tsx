import { StateGlyph, type StateGlyphState } from "@compozy/ui";

import type { SessionListTotals as Totals } from "../../hooks/use-session-list-view";

const TOTALS: readonly { key: keyof Totals; label: string; glyph: StateGlyphState }[] = [
  { key: "needsYou", label: "needs you", glyph: "attention" },
  { key: "working", label: "working", glyph: "running" },
  { key: "idle", label: "idle", glyph: "idle" },
];

/**
 * The quiet footer under a session list: how many sessions in the list's whole
 * scope need you, are working, or sit idle — the daemon's exact facet counts,
 * never a loaded page. Only non-zero states show; with none, nothing renders.
 * It reads, it does not filter.
 */
export function SessionListTotals({
  totals,
  testIdPrefix,
}: {
  totals: Totals | undefined;
  testIdPrefix: string;
}) {
  const shown = totals ? TOTALS.filter(item => totals[item.key] > 0) : [];
  if (!totals || shown.length === 0) return null;
  return (
    <footer
      aria-label="Session totals"
      data-testid={`${testIdPrefix}-totals`}
      className="flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 border-t border-line px-4 py-2 text-micro text-subtle"
    >
      {shown.map(item => (
        <span
          key={item.key}
          className="flex items-center gap-1.5"
          data-testid={`${testIdPrefix}-totals-${item.key}`}
        >
          <StateGlyph size="sm" state={item.glyph} still />
          <span>
            <span className="font-mono text-muted-foreground tabular-nums">{totals[item.key]}</span>{" "}
            {item.label}
          </span>
        </span>
      ))}
    </footer>
  );
}
