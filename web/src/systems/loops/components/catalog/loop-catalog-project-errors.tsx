import { RotateCcw, TriangleAlert } from "@compozy/ui/icons";

import { Alert, AlertActions, AlertDescription, Button } from "@compozy/ui";

/** One project whose definitions the All projects batch could not resolve. */
export interface LoopCatalogProjectError {
  workspaceId: string;
  /** The project's name, or "User". */
  label: string;
  message: string;
}

interface LoopCatalogProjectErrorsProps {
  errors: readonly LoopCatalogProjectError[];
  truncated: boolean;
  onRetry: () => void;
}

/**
 * Per-project honesty for the All projects definitions batch: each project
 * that failed gets its own inline row (healthy rows still render), and a batch
 * cut at its bound says so instead of passing for the whole catalog.
 */
export function LoopCatalogProjectErrors({
  errors,
  truncated,
  onRetry,
}: LoopCatalogProjectErrorsProps) {
  if (errors.length === 0 && !truncated) return null;
  return (
    <div className="flex flex-col gap-2" data-testid="loop-catalog-batch-notices">
      {errors.map(error => (
        <Alert
          data-testid="loop-catalog-project-error"
          data-workspace-id={error.workspaceId}
          key={error.workspaceId}
          role="alert"
          variant="danger"
        >
          <TriangleAlert aria-hidden="true" />
          <AlertDescription title={error.message}>Couldn't load {error.label}</AlertDescription>
          <AlertActions>
            <Button onClick={onRetry} size="sm" type="button" variant="secondary">
              <RotateCcw aria-hidden="true" />
              Retry
            </Button>
          </AlertActions>
        </Alert>
      ))}
      {truncated ? (
        <p className="text-caption text-muted" data-testid="loop-catalog-truncated">
          Showing the first 2,000 definitions
        </p>
      ) : null}
    </div>
  );
}
