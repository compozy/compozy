import { AlertCircle, Trash2 } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Button, ConfirmDialog } from "@compozy/ui";

import { formatRelativeTime, type AutomationView } from "@/systems/automation";

/** `7 automations · 6 on · next run in 14h`. */
export function AutomationListFooter({
  total,
  enabledCount,
  nextRunAt,
}: {
  /** Both kinds' totals; null while one is unknown or failed. */
  total: number | null;
  enabledCount: number;
  nextRunAt: string | null;
}) {
  const next = nextRunAt ? formatRelativeTime(nextRunAt).replace(/^In /, "") : null;
  return (
    <p
      className="px-1 pt-3 text-caption text-subtle tabular-nums"
      data-testid="automations-list-footer"
    >
      {total ?? "—"} {total === 1 ? "automation" : "automations"} · {enabledCount} on
      {next ? ` · next run in ${next}` : null}
    </p>
  );
}

/** One list failed: loaded rows stay under this alert (Business Rule 17). */
export function AutomationPartialAlert({
  failed,
  onRetry,
}: {
  failed: "schedule" | "event";
  onRetry: () => void;
}) {
  const noun = failed === "schedule" ? "scheduled automations" : "event automations";
  return (
    <div className="border-b border-line px-9 py-3">
      <Alert data-testid="automations-partial-alert" variant="warning">
        <AlertCircle aria-hidden="true" className="size-4" />
        <AlertTitle>Couldn&apos;t load {noun}.</AlertTitle>
        <AlertDescription>
          <Button onClick={onRetry} size="sm" type="button" variant="secondary">
            Try again
          </Button>
        </AlertDescription>
      </Alert>
    </div>
  );
}

/** Delete by typing the exact name (Business Rule 12). */
export function AutomationDeleteDialog({
  target,
  pending,
  onConfirm,
  onOpenChange,
}: {
  target: AutomationView | null;
  pending: boolean;
  onConfirm: () => Promise<void>;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "confirm-delete-automation-btn" }}
      confirmIcon={Trash2}
      confirmInputProps={{ "data-testid": "automation-delete-confirm-typing" }}
      confirmLabel={pending ? "Deleting…" : "Delete automation"}
      confirmTyping={target?.name ?? ""}
      contentProps={{ "data-testid": "automation-delete-dialog" }}
      description={
        <>
          This permanently deletes <span className="font-mono text-fg">{target?.name}</span>. Past
          runs stay.
        </>
      }
      isPending={pending}
      onConfirm={() => {
        void onConfirm().catch(() => undefined);
      }}
      onOpenChange={onOpenChange}
      open={target !== null}
      title="Delete automation?"
      tone="danger"
    />
  );
}
