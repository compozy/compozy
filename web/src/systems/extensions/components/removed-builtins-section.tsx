import { Puzzle } from "@compozy/ui/icons";

import { Button, ListingRow, ListingRowSkeleton, Section, SkeletonRegion, Time } from "@compozy/ui";

import { useRestoreBuiltinExtension } from "../hooks/use-extension-actions";
import { useRemovedBuiltins } from "../hooks/use-extensions";
import { isExtensionExistsConflict } from "../lib/extension-restore-conflict";
import type { ExtensionRemovedBuiltin } from "../types";

const SECTION_LABEL = "Removed built-ins";

type RestoreMutation = ReturnType<typeof useRestoreBuiltinExtension>;

interface RemovedBuiltinsSectionProps {
  /**
   * Whether the viewer may restore built-ins. The web has no per-actor authority field, so the
   * operator surface passes it. Without it the section and its actions are absent, not disabled,
   * and the removed list is never requested.
   */
  canRestore: boolean;
}

/** The conflict names the operator's next step; any other refusal shows the daemon's reason. */
function restoreFailureMessage(error: Error, name: string): string {
  return isExtensionExistsConflict(error)
    ? `${name} is already installed. Remove the installed copy first.`
    : error.message;
}

function RemovedBuiltinRow({
  item,
  restore,
}: {
  item: ExtensionRemovedBuiltin;
  restore: RestoreMutation;
}) {
  const isThisRow = restore.variables === item.name;
  const pending = restore.isPending && isThisRow;
  const failure = restore.isError && isThisRow ? restore.error : null;

  return (
    <ListingRow data-testid="removed-builtin-row" interactive={false}>
      <ListingRow.Icon>
        <Puzzle aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title>{item.name}</ListingRow.Title>
        </ListingRow.Name>
        <ListingRow.Meta>
          Removed <Time iso={item.removed_at} mode="absolute" />
        </ListingRow.Meta>
        {failure ? (
          <p
            className="text-eyebrow text-destructive"
            data-testid="removed-builtin-error"
            role="alert"
          >
            {restoreFailureMessage(failure, item.name)}
          </p>
        ) : null}
      </ListingRow.Main>
      <ListingRow.Trail>
        <Button
          aria-label={`Restore ${item.name}`}
          data-testid="removed-builtin-restore"
          disabled={restore.isPending}
          onClick={() => {
            // A pending restore blocks every other trigger, not only its own row.
            if (restore.isPending) return;
            restore.mutate(item.name);
          }}
          size="sm"
          type="button"
          variant="secondary"
        >
          {pending ? "Restoring…" : "Restore"}
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  );
}

function RemovedBuiltinsSkeleton() {
  return (
    <SkeletonRegion label="Loading removed built-ins">
      <ListingRowSkeleton count={1} />
    </SkeletonRegion>
  );
}

function RemovedBuiltinsError({
  message,
  onRetry,
  retrying,
}: {
  message: string;
  onRetry: () => void;
  retrying: boolean;
}) {
  return (
    <div className="space-y-2 px-4 py-3" data-testid="removed-builtins-error">
      <p className="text-small-body font-medium text-destructive">
        Couldn't load removed built-ins
      </p>
      <p className="text-eyebrow text-muted-foreground">{message}</p>
      <Button disabled={retrying} onClick={onRetry} size="sm" type="button" variant="secondary">
        Retry
      </Button>
    </div>
  );
}

/**
 * Permanently removed built-ins, each with its removal time and a restore action. Hidden when no
 * built-in is removed, so the installed inventory shows nothing it does not need.
 */
function RemovedBuiltinsSection({ canRestore }: RemovedBuiltinsSectionProps) {
  const removed = useRemovedBuiltins(canRestore);
  const restore = useRestoreBuiltinExtension();

  if (!canRestore) return null;

  if (removed.isError) {
    return (
      <Section data-testid="removed-builtins-section" label={SECTION_LABEL}>
        <div className="divide-y divide-line-soft overflow-hidden rounded-lg bg-sunken">
          <RemovedBuiltinsError
            message={removed.error.message}
            onRetry={() => void removed.refetch()}
            retrying={removed.isFetching}
          />
        </div>
      </Section>
    );
  }

  if (removed.isPending) {
    return (
      <Section data-testid="removed-builtins-section" label={SECTION_LABEL}>
        <div className="divide-y divide-line-soft overflow-hidden rounded-lg bg-sunken">
          <RemovedBuiltinsSkeleton />
        </div>
      </Section>
    );
  }

  // The daemon's order is the display order; the list is never re-sorted here.
  const items = removed.data ?? [];
  if (items.length === 0) return null;

  return (
    <Section count={items.length} data-testid="removed-builtins-section" label={SECTION_LABEL}>
      <div className="divide-y divide-line-soft overflow-hidden rounded-lg bg-sunken">
        {items.map(item => (
          <RemovedBuiltinRow item={item} key={item.name} restore={restore} />
        ))}
      </div>
    </Section>
  );
}

export { RemovedBuiltinsSection };
export type { RemovedBuiltinsSectionProps };
