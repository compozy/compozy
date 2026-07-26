import type { ComponentProps, ReactNode } from "react";

import { cn, Eyebrow } from "@agh/ui";

interface LoopRunSectionProps extends ComponentProps<"section"> {
  label: string;
  /** Right slot of the label row (e.g. the one Live badge per surface). */
  right?: ReactNode;
}

/**
 * The redesigned run page's section shell: an eyebrow label row with an
 * optional right slot above the section body (prototype `.section__label`).
 */
export function LoopRunSection({
  label,
  right,
  className,
  children,
  ...props
}: LoopRunSectionProps) {
  return (
    <section className={cn("flex min-w-0 flex-col", className)} {...props}>
      <div className="mb-2.5 flex items-center gap-2">
        <Eyebrow className="text-subtle">{label}</Eyebrow>
        {right ? (
          <>
            <span className="flex-1" />
            {right}
          </>
        ) : null}
      </div>
      {children}
    </section>
  );
}
