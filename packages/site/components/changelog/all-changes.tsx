"use client";

/** "Every merged change": the full record of a release, folded until asked for. */
import { Button, cn } from "@compozy/ui";
import { ChevronDown } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { HAIRLINE } from "@/components/site/layout";

export function AllChanges({
  total,
  startOpen,
  children,
}: {
  total: number;
  startOpen: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(startOpen);
  return (
    <section id="all-changes" className={cn("mt-14 scroll-mt-28 border-t pt-10", HAIRLINE)}>
      <Button variant="outline" aria-expanded={open} onClick={() => setOpen(value => !value)}>
        Every merged change · {total}
        <ChevronDown className={cn("transition-transform", open && "rotate-180")} />
      </Button>
      {open ? <div className="mt-8 flex flex-col gap-9">{children}</div> : null}
    </section>
  );
}
