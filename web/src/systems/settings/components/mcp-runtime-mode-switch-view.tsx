import { useId } from "react";

import { cn, Switch } from "@compozy/ui";

export interface MCPRuntimeModeSwitchViewProps {
  /** Runtime display name ("Claude Code"). */
  runtimeName: string;
  checked: boolean;
  pending?: boolean;
  error?: string | null;
  onCheckedChange: (checked: boolean) => void;
  /** Visible caption beside the switch; the accessible name always names the runtime. */
  label?: string;
  className?: string;
  "data-testid"?: string;
}

/** "Use in CompozyOS sessions": whether CompozyOS sessions also load the runtime's own servers. */
export function MCPRuntimeModeSwitchView({
  runtimeName,
  checked,
  pending = false,
  error,
  onCheckedChange,
  label = "Use in CompozyOS sessions",
  className,
  "data-testid": testId,
}: MCPRuntimeModeSwitchViewProps) {
  const errorId = useId();
  return (
    <span className={cn("flex shrink-0 flex-col items-end gap-1", className)}>
      <label className="flex items-center gap-2 text-small-body text-muted-foreground">
        {label}
        <Switch
          aria-busy={pending || undefined}
          aria-describedby={error ? errorId : undefined}
          aria-label={`Use ${runtimeName}'s own MCP servers in CompozyOS sessions`}
          checked={checked}
          data-testid={testId}
          disabled={pending}
          onCheckedChange={onCheckedChange}
        />
      </label>
      {error ? (
        <span className="text-meta text-destructive" id={errorId} role="alert">
          {error}
        </span>
      ) : null}
    </span>
  );
}
