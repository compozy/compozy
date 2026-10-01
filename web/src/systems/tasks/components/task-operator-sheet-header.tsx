import { Search } from "lucide-react";
import type { ReactNode } from "react";

import { KindIcon, SheetDescription, SheetHeader, SheetTitle } from "@compozy/ui";

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
        <KindIcon aria-hidden="true" icon={Search} tone="well" />
        <div className="min-w-0">
          <span className="eyebrow font-mono text-subtle">Operator</span>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </div>
      </div>
    </SheetHeader>
  );
}
