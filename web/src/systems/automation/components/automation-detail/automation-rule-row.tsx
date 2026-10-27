import type { ComponentProps, ReactNode } from "react";

import { cn, Eyebrow } from "@compozy/ui";

type RuleRowProps = Omit<ComponentProps<"div">, "children"> & {
  label: string;
  children: ReactNode;
};

export function RuleRow({ label, children, className, ...props }: RuleRowProps) {
  return (
    <div
      className={cn(
        "grid grid-cols-[56px_minmax(0,1fr)] gap-3 border-t border-line-soft px-3 py-3 first:border-t-0 md:grid-cols-[84px_minmax(0,1fr)] md:gap-3.5 md:px-4 md:py-3.5",
        className
      )}
      {...props}
    >
      <Eyebrow className="pt-0.5 text-subtle">{label}</Eyebrow>
      <div className="flex min-w-0 flex-col">{children}</div>
    </div>
  );
}
