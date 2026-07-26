import type { ComponentProps } from "react";

import { cn } from "@agh/ui";

export interface HomePageMetaProps extends ComponentProps<"div"> {
  workspaceName: string | null;
  /** Injected for deterministic tests; defaults to the current date. */
  today?: Date;
}

const DATE_FORMAT = new Intl.DateTimeFormat(undefined, {
  weekday: "long",
  month: "short",
  day: "numeric",
});

/**
 * Demoted page meta line — the window head owns identity, so the body opens
 * with a 12px date · workspace line instead of a second H1.
 */
export function HomePageMeta({ workspaceName, today, className, ...props }: HomePageMetaProps) {
  return (
    <div
      className={cn(
        "mb-4 flex flex-wrap items-center gap-2 border-b border-line pb-3.5 text-small-body text-subtle",
        className
      )}
      data-slot="home-page-meta"
      {...props}
    >
      <span>{DATE_FORMAT.format(today ?? new Date())}</span>
      {workspaceName ? (
        <>
          <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
          <span>
            workspace <span className="font-medium text-muted">{workspaceName}</span>
          </span>
        </>
      ) : null}
    </div>
  );
}
