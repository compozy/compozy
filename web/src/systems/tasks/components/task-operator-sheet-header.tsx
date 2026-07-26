import { Search } from "lucide-react";
import type { ReactNode } from "react";

import { SheetDescription, SheetHeader, SheetTitle } from "@agh/ui";

export function TaskOperatorSheetHeader({
  description,
  title,
}: {
  description: ReactNode;
  title: string;
}) {
  return (
    <SheetHeader>
      <div className="flex items-start gap-3">
        <span
          aria-hidden="true"
          className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-tint text-accent-strong"
        >
          <Search className="size-4" />
        </span>
        <div className="min-w-0">
          <span className="eyebrow font-mono text-subtle">Operator</span>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </div>
      </div>
    </SheetHeader>
  );
}
