// Manual section: full-width band with number, file label, status, title (one <em> word) and intro.
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

import type { Status } from "../chapters";
import { band } from "../cls";
import { StatusTag } from "./status-tag";

type Band = "ink" | "ink2" | "paper" | "black";

interface SectionProps {
  id: string;
  n: string;
  file: string;
  title: ReactNode;
  bg?: Band;
  status?: Status;
  intro?: ReactNode;
  children?: ReactNode;
}

export function Section({ id, n, file, title, bg = "ink", status, intro, children }: SectionProps) {
  return (
    <section id={id} className={cn(band, `band-${bg}`)}>
      <div className="wrap">
        <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 mono-label font-medium text-(--b-muted)">
          <b className="font-medium text-(--b-accent)">{n}</b>
          <span>{file}</span>
          {status && <StatusTag status={status} className="ml-1" />}
        </div>
        <h2 className="mt-4 mb-5 text-title text-(--b-fg)">{title}</h2>
        {intro && (
          <div className="mb-10 max-w-[62ch] text-lead text-(--b-muted) [&_p]:m-0 [&_p+p]:mt-3 [&_strong]:text-(--b-fg)">
            {intro}
          </div>
        )}
        {children}
      </div>
    </section>
  );
}
