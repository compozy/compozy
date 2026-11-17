import type { ComponentType, ReactNode } from "react";

/** A labelled fact inside a row panel: small faint label above a value. */
export function MCPPanelFact({
  label,
  icon: Icon,
  children,
}: {
  label: string;
  icon?: ComponentType<{ className?: string; "aria-hidden"?: true }>;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1.5 text-meta text-faint">
        {Icon ? <Icon aria-hidden={true} className="size-3.5" /> : null}
        {label}
      </span>
      {children}
    </div>
  );
}

/** What a server runs or said, verbatim, in a sunken mono block. */
export function MCPCommandBlock({ children, testId }: { children: ReactNode; testId?: string }) {
  return (
    <code
      data-testid={testId}
      className="block rounded-md bg-sunken px-2.5 py-2 font-mono text-mono-id leading-relaxed break-all text-muted-foreground shadow-hairline-inset"
    >
      {children}
    </code>
  );
}
