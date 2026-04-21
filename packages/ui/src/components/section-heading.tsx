import type { HTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export interface SectionHeadingProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  actions?: ReactNode;
  description?: ReactNode;
  eyebrow?: ReactNode;
  title: ReactNode;
}

export function SectionHeading({
  actions,
  className,
  description,
  eyebrow,
  title,
  ...props
}: SectionHeadingProps): ReactElement {
  return (
    <header
      className={cn("flex flex-col gap-5 md:flex-row md:items-end md:justify-between", className)}
      {...props}
    >
      <div className="space-y-2">
        {eyebrow ? <p className="eyebrow text-muted-foreground">{eyebrow}</p> : null}
        <div className="space-y-2">
          <h1 className="font-display text-[clamp(2rem,4vw,3rem)] leading-none tracking-[-0.03em] text-foreground">
            {title}
          </h1>
          {description ? (
            <p className="max-w-3xl text-sm leading-7 text-muted-foreground">{description}</p>
          ) : null}
        </div>
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-3">{actions}</div> : null}
    </header>
  );
}
