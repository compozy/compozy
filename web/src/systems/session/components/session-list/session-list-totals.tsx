import { StateGlyph, type StateGlyphState } from "@compozy/ui";

import type { SessionListTotals as Totals } from "../../hooks/use-session-list-view";

const TOTALS: readonly { key: keyof Totals; label: string; glyph: StateGlyphState }[] = [
  { key: "needsYou", label: "needs you", glyph: "attention" },
  { key: "working", label: "working", glyph: "running" },
  { key: "idle", label: "idle", glyph: "idle" },
];

/**
 * The quiet footer under a session list: how many sessions in the list's whole
 * scope need you, are working, or sit idle — exact facet counts across answered
 * computers, never a loaded page. Names computers that could not answer (`R9`).
 */
export function SessionListTotals({
  totals,
  uncountedNames,
  testIdPrefix,
}: {
  totals: Totals | undefined;
  /** Names of computers omitted from the aggregate total (`R9`). */
  uncountedNames?: readonly string[];
  testIdPrefix: string;
}) {
  const shown = totals ? TOTALS.filter(item => totals[item.key] > 0) : [];
  const hasUncounted = (uncountedNames?.length ?? 0) > 0;
  if (shown.length === 0 && !hasUncounted) return null;
  return (
    <>
      {totals && shown.length > 0 ? (
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
                <span className="font-mono text-muted-foreground tabular-nums">
                  {totals[item.key]}
                </span>{" "}
                {item.label}
              </span>
            </span>
          ))}
        </footer>
      ) : null}
      {hasUncounted ? (
        <p
          className="m-0 px-4 pb-2 text-micro text-faint"
          data-testid={`${testIdPrefix}-not-counted`}
        >
          Not counted: {uncountedNames!.join(", ")}
        </p>
      ) : null}
    </>
  );
}
