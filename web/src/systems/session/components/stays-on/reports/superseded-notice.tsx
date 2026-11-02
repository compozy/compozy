import { History } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

/** The read-only foot of a conversation that moved on; the pointer says where to. */
export function SupersededNotice({ onOpenCurrent }: { onOpenCurrent: () => void }) {
  return (
    <div className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5">
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg bg-surface-2 px-3 py-2.5 shadow-hairline-inset">
        <History aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        <span className="min-w-0 flex-1 text-small-body text-muted-foreground">
          This conversation moved on. You can read it here.
        </span>
        <Button onClick={onOpenCurrent} size="sm" type="button" variant="ghost">
          Open the current one
        </Button>
      </div>
    </div>
  );
}
