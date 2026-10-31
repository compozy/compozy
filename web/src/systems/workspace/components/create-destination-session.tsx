import { cn } from "@compozy/ui";

import type { DestinationKind } from "../lib/workspace-scope-copy";
import { CreateDestinationPicker, type CreateDestinationOption } from "./create-destination-picker";
import { WorkspaceScopeStatement } from "./workspace-scope-statement";

/** One aggregated axis: its picker options, current value, and change handler. */
export interface SessionDestinationAxis {
  options: readonly CreateDestinationOption[];
  value: string;
  onChange: (value: string) => void;
}

export interface SessionDestinationRowProps extends React.ComponentProps<"div"> {
  /** Present only under All projects (ADR-003). */
  project?: SessionDestinationAxis;
  /** Present only under All profiles (ADR-003). */
  profile?: SessionDestinationAxis;
  /** The bounded destination, used when no axis is aggregated. */
  scope: DestinationKind;
  destination: string;
  root?: string;
  disabled?: boolean;
}

/**
 * Where a new session lands, for session surfaces only: "In **project ▾** · as
 * **profile ▾**", each segment present only while its axis is aggregated. With
 * neither aggregated it is today's read-only statement. Worktree, Loop, and
 * automation creation keep the profile chip (`CreateDestinationStatement`).
 */
export function SessionDestinationRow({
  project,
  profile,
  scope,
  destination,
  root,
  disabled,
  className,
  ...props
}: SessionDestinationRowProps) {
  if (!project && !profile) {
    return (
      <WorkspaceScopeStatement
        className={className}
        destination={destination}
        kind="session"
        root={root}
        scope={scope}
        variant="note"
      />
    );
  }
  return (
    <div
      data-slot="session-destination-row"
      data-testid="session-destination-row"
      className={cn(
        "inline-flex max-w-full flex-wrap items-center gap-x-1 text-form-hint text-muted-foreground",
        className
      )}
      {...props}
    >
      {/* Literal spaces keep the sentence readable as text; flex gaps lay it out. */}
      <span>In</span>{" "}
      {project ? (
        <CreateDestinationPicker
          axis="Project"
          disabled={disabled}
          onChange={project.onChange}
          options={project.options}
          testId="session-destination-project"
          value={project.value}
        />
      ) : (
        <span className="font-medium text-fg">{destination}</span>
      )}
      {profile ? (
        <>
          {" "}
          <span aria-hidden="true">·</span> <span>as</span>{" "}
          <CreateDestinationPicker
            axis="Profile"
            disabled={disabled}
            onChange={profile.onChange}
            options={profile.options}
            testId="session-destination-profile"
            value={profile.value}
          />
        </>
      ) : null}
    </div>
  );
}
