import { Eyebrow } from "@compozy/ui";

export function InventoryHeadRow() {
  return (
    <div className="flex items-center gap-4 border-b border-line px-4 py-2">
      <Eyebrow className="min-w-0 flex-1 text-muted-foreground">Step</Eyebrow>
      <Eyebrow className="hidden min-w-0 flex-1 text-muted-foreground lg:block">Loop</Eyebrow>
      <Eyebrow className="hidden min-w-0 flex-1 text-muted-foreground md:block">Reason</Eyebrow>
      <Eyebrow className="w-24 shrink-0 text-right text-muted-foreground">Time in state</Eyebrow>
      <span aria-hidden="true" className="size-3.5 shrink-0" />
    </div>
  );
}
