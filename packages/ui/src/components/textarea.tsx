import * as React from "react";

import { cn } from "../lib/utils";

export type TextareaVariant = "default" | "mono";

export interface TextareaProps extends React.ComponentProps<"textarea"> {
  /** `mono` switches to `font-mono` + 12 px wave-2 / analysis §4. */
  variant?: TextareaVariant;
}

const VARIANT_CLASSNAME: Record<TextareaVariant, string> = {
  default: "text-small-body",
  mono: "font-mono text-form-label",
};

function Textarea({ className, variant = "default", ...props }: TextareaProps) {
  return (
    <textarea
      data-slot="textarea"
      data-variant={variant}
      className={cn(
        "flex field-sizing-content min-h-textarea-min w-full rounded-md border border-line bg-elevated px-3 py-2 text-fg transition-colors outline-none placeholder:text-subtle focus-visible:outline-none focus-visible:shadow-focus-ring focus-visible:border-line-strong disabled:cursor-not-allowed disabled:bg-canvas disabled:border-line-soft disabled:text-disabled disabled:opacity-100 aria-invalid:border-danger",
        VARIANT_CLASSNAME[variant],
        className
      )}
      {...props}
    />
  );
}

export { Textarea };
