import { AlertCircle } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Button } from "@compozy/ui";

import { formatRelativeTime } from "@/systems/automation";

/**
 * `7 automations · 6 on · next run in 14h`. While more pages exist the loaded
 * rows can't speak for the whole list: `60 automations · 50 shown`.
 */
export function AutomationListFooter({
  total,
  enabledCount,
  loadedCount,
  nextRunAt,
}: {
  /** Both kinds' totals; null while one is unknown or failed. */
  total: number | null;
  /** Null until every page is loaded. */
  enabledCount: number | null;
  loadedCount: number;
  nextRunAt: string | null;
}) {
  const next = nextRunAt ? formatRelativeTime(nextRunAt).replace(/^In /, "") : null;
  return (
    <p
      className="px-1 pt-3 text-caption text-subtle tabular-nums"
      data-testid="automations-list-footer"
    >
      {total ?? "—"} {total === 1 ? "automation" : "automations"} ·{" "}
      {enabledCount === null ? `${loadedCount} shown` : `${enabledCount} on`}
      {next ? ` · next run in ${next}` : null}
    </p>
  );
}

/**
 * A load failed while rows are on screen: one list failed (Business Rule 17),
 * or a refresh of every shown list failed and the cached rows stay.
 */
export function AutomationPartialAlert({
  failed,
  onRetry,
}: {
  failed: "schedule" | "event" | "refresh";
  onRetry: () => void;
}) {
  const title =
    failed === "refresh"
      ? "Couldn't refresh automations."
      : `Couldn't load ${failed === "schedule" ? "scheduled automations" : "event automations"}.`;
  return (
    <div className="border-b border-line px-9 py-3">
      <Alert data-testid="automations-partial-alert" variant="warning">
        <AlertCircle aria-hidden="true" className="size-4" />
        <AlertTitle>{title}</AlertTitle>
        <AlertDescription>
          <Button onClick={onRetry} size="sm" type="button" variant="secondary">
            Try again
          </Button>
        </AlertDescription>
      </Alert>
    </div>
  );
}
