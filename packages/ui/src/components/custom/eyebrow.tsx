import * as React from "react";

import { cn } from "../../lib/utils";

export type EyebrowProps = Omit<React.ComponentProps<"span">, "children"> & {
  children: React.ReactNode;
};

function Eyebrow({ className, children, ...props }: EyebrowProps) {
  return (
    <span data-slot="eyebrow" className={cn("eyebrow", className)} {...props}>
      {children}
    </span>
  );
}

export { Eyebrow };
