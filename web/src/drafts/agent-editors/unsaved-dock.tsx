/** The pill that floats up while there is something to save, and is gone when there isn't. */
import { Button, cn } from "@compozy/ui";

export function UnsavedDock({
  count,
  blocked,
  onDiscard,
  onSave,
}: {
  count: number;
  /** Why Save can't run yet, in place of the count. */
  blocked?: string;
  onDiscard: () => void;
  onSave: () => void;
}) {
  if (count === 0) return null;
  return (
    <div className="pointer-events-none sticky bottom-5 z-10 flex justify-center">
      <div
        className={cn(
          "pointer-events-auto flex items-center gap-3 rounded-pill bg-card py-1.5 pr-1.5 pl-4 shadow-overlay",
          "animate-in fade-in slide-in-from-bottom-2 duration-base motion-reduce:animate-none"
        )}
        role="status"
      >
        <span className={cn("text-small-body", blocked ? "text-danger" : "text-fg-2")}>
          {blocked ?? `${count} unsaved ${count === 1 ? "change" : "changes"}`}
        </span>
        <div className="flex items-center gap-1">
          <Button onClick={onDiscard} size="sm" type="button" variant="ghost">
            Discard
          </Button>
          <Button disabled={Boolean(blocked)} onClick={onSave} size="sm" type="button">
            Save
          </Button>
        </div>
      </div>
    </div>
  );
}
