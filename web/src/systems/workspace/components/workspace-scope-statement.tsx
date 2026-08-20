import { cn } from "@compozy/ui";

import type { WorkspaceScopeMode } from "../lib/workspace-scope-mode";

export type WorkspaceScopeStatementVariant = "chip" | "note";
export type WorkspaceScopeStatementKind = "create" | "install" | "session" | "edit" | "subscribe";

export interface WorkspaceScopeStatementProps extends React.ComponentProps<"span"> {
  variant: WorkspaceScopeStatementVariant;
  scope: WorkspaceScopeMode;
  destination: string;
  kind: WorkspaceScopeStatementKind;
  /** Session root or operator-home path. */
  root?: string;
}

function statementParts(
  kind: WorkspaceScopeStatementKind,
  isGlobal: boolean,
  root: string | undefined
): { prefix: string; suffix: string } {
  switch (kind) {
    case "install":
      return {
        prefix: isGlobal ? "Installs at " : "Installs to ",
        suffix: isGlobal ? " — available in every workspace." : ".",
      };
    case "session":
      return {
        prefix: "Runs in ",
        suffix: root && root.length > 0 ? ` — ${root}` : "",
      };
    case "edit":
      return { prefix: "Lives in ", suffix: "" };
    case "subscribe":
      return {
        prefix: isGlobal ? "Subscribes at " : "Subscribes in ",
        suffix: isGlobal ? " — available in every workspace." : ".",
      };
    default:
      return {
        prefix: "Creates in ",
        suffix: isGlobal ? " — visible to every workspace." : ".",
      };
  }
}

/**
 * Read-only landing statement. Scope changes happen at the menubar — this
 * never opens a picker.
 */
export function WorkspaceScopeStatement({
  variant,
  scope,
  destination,
  kind,
  root,
  className,
  ref,
  ...props
}: WorkspaceScopeStatementProps) {
  const { prefix, suffix } = statementParts(kind, scope === "global", root);
  const chip = variant === "chip";
  const text = (
    <>
      {prefix}
      <span className="font-medium text-fg-strong">{destination}</span>
      {chip && suffix === "." ? null : suffix}
    </>
  );

  return (
    <span
      {...props}
      ref={ref}
      className={cn(
        chip
          ? "inline-block h-7 max-w-full truncate rounded-md border border-line bg-canvas-tint px-2.5 align-middle text-form-hint leading-7 text-muted"
          : "text-form-hint text-muted",
        className
      )}
      data-slot="workspace-scope-statement"
      data-testid="workspace-scope-statement"
      data-variant={variant}
    >
      {text}
    </span>
  );
}
