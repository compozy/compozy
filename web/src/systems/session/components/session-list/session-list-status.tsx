import { TriangleAlert, WifiOff } from "@compozy/ui/icons";

import { Alert, AlertActions, AlertDescription, Button } from "@compozy/ui";

import type { SessionListViewModel } from "../../hooks/use-session-list-view";

/** Why the rows below may be stale or flat: the stream, the catalog, or the folder list. */
export function SessionListStatus({
  disconnected,
  view,
}: {
  disconnected: boolean;
  view: SessionListViewModel;
}) {
  return (
    <>
      {disconnected ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <WifiOff aria-hidden="true" />
          <AlertDescription>
            Can&apos;t get session updates right now. The sessions below may be out of date.
          </AlertDescription>
        </Alert>
      ) : null}
      {view.catalog?.failed && (view.catalog.sessions.length ?? 0) === 0 ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertDescription>Couldn&rsquo;t load sessions.</AlertDescription>
          <AlertActions>
            <Button size="xs" type="button" variant="secondary" onClick={view.catalog.retry}>
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
      {view.layoutError ? (
        <Alert className="mx-3 my-1 w-auto px-2.5 py-2" role="status" variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertDescription>Couldn&rsquo;t load folders.</AlertDescription>
          <AlertActions>
            <Button size="xs" type="button" variant="secondary" onClick={view.layoutError.retry}>
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ) : null}
    </>
  );
}
