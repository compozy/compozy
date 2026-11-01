import { cn } from "@compozy/ui";

import type { DestinationKind } from "../lib/workspace-scope-copy";

export type WorkspaceScopeStatementVariant = "chip" | "note";
export type WorkspaceScopeStatementKind = "create" | "install" | "session" | "edit" | "subscribe";

export interface WorkspaceScopeStatementProps extends React.ComponentProps<"span"> {
  variant: WorkspaceScopeStatementVariant;
  /** `user` lands outside any project. */
  scope: DestinationKind;
  destination: string;
  kind: WorkspaceScopeStatementKind;
  /** Session root or operator-home path. */
  root?: string;
}

function statementParts(
  kind: WorkspaceScopeStatementKind,
  isUser: boolean,
  root: string | undefined
): { prefix: string; suffix: string } {
  switch (kind) {
    case "install":
      return {
        prefix: isUser ? "Installs at " : "Installs to ",
        suffix: isUser ? " — available in every project." : ".",
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
        prefix: isUser ? "Subscribes at " : "Subscribes in ",
        suffix: isUser ? " — available in every project." : ".",
      };
    default:
      return { prefix: "Creates in ", suffix: "." };
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
  const { prefix, suffix } = statementParts(kind, scope === "user", root);
  const chip = variant === "chip";
  const text = (
    <>
      {prefix}
      <span className="font-medium text-fg">{destination}</span>
      {chip && suffix === "." ? null : suffix}
    </>
  );

  return (
    <span
      {...props}
      ref={ref}
      className={cn(
        chip
          ? "inline-block h-7 max-w-full truncate rounded-pill bg-surface-2 px-2.5 align-middle text-form-hint leading-7 text-muted-foreground"
          : "text-form-hint text-muted-foreground",
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
