import type * as React from "react";

import { cn } from "../../lib/utils";
import { Eyebrow } from "./eyebrow";

export interface UntrustedFrameProps extends Omit<React.ComponentProps<"aside">, "children"> {
  /** Provenance stamp. Rendered as `<Eyebrow>` — no product copy is baked in. */
  stamp: React.ReactNode;
  children: React.ReactNode;
}

/**
 * Dashed hairline frame for quoted untrusted text. The stamp names the source;
 * the body stays plain, selectable prose. Consumers pass inert text — never
 * markdown, HTML, or a control.
 */
function UntrustedFrame({ stamp, children, className, ...props }: UntrustedFrameProps) {
  return (
    <aside
      data-slot="untrusted-frame"
      className={cn(
        "rounded-md border border-dashed border-line-strong bg-canvas-soft px-3 py-2",
        className
      )}
      {...props}
    >
      <Eyebrow className="text-muted">{stamp}</Eyebrow>
      <div
        data-slot="untrusted-frame-body"
        className="mt-1.5 whitespace-pre-wrap break-words text-small-body text-fg"
      >
        {children}
      </div>
    </aside>
  );
}

export { UntrustedFrame };
