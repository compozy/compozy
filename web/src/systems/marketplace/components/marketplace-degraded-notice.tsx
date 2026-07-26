import { AlertCircle } from "lucide-react";

import { Alert, AlertActions, AlertDescription, Button } from "@agh/ui";

// Discriminated on usage context so the page-level notice reads naturally ("Marketplace results may
// be out of date") while per-kind copy stays verbatim. `scope: "page"` forbids `label` by
// construction, so a doubled "Marketplace marketplace is unreachable" is impossible.
type MarketplaceDegradedNoticeProps = { onRetry: () => void } & (
  | { scope: "kind"; label: string; hasItems: boolean }
  | { scope: "page" }
);

function MarketplaceDegradedNotice(props: MarketplaceDegradedNoticeProps) {
  // A page-level notice only renders while retained data is shown, so it is always stale (warning),
  // never unreachable (danger); the "nothing to show" case is a full-page Empty state upstream.
  const isStale = props.scope === "page" || props.hasItems;
  const message =
    props.scope === "page"
      ? "Marketplace results may be out of date"
      : props.hasItems
        ? `${props.label} results may be out of date`
        : `${props.label} marketplace is unreachable`;
  return (
    <Alert variant={isStale ? "warning" : "danger"}>
      <AlertCircle aria-hidden="true" className="mt-0.5 size-3.5 shrink-0" />
      <AlertDescription className="font-medium">{message}</AlertDescription>
      <AlertActions>
        <Button onClick={props.onRetry} size="sm" type="button" variant="ghost">
          Retry
        </Button>
      </AlertActions>
    </Alert>
  );
}

export { MarketplaceDegradedNotice };
export type { MarketplaceDegradedNoticeProps };
