import type { ComponentProps, ReactNode } from "react";
import type { LucideIcon } from "@compozy/ui/icons";

import { cn } from "@compozy/ui";

interface RailSectionProps extends Omit<ComponentProps<"section">, "title"> {
  icon: LucideIcon;
  title: string;
  children: ReactNode;
}

/** One band of the one-card rail; bands are separated by hairlines (patterns §03). */
export function RailSection({
  icon: Icon,
  title,
  children,
  className,
  ...props
}: RailSectionProps) {
  return (
    <section
      className={cn("border-t border-line-soft px-4 py-3.5 first:border-t-0", className)}
      {...props}
    >
      <h3 className="mb-2 flex items-center gap-1.5 text-eyebrow text-muted">
        <Icon aria-hidden="true" className="size-3 text-muted" />
        {title}
      </h3>
      {children}
    </section>
  );
}
