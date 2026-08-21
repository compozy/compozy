import { ProfileDestinationChip } from "@/systems/profiles";

import {
  WorkspaceScopeStatement,
  type WorkspaceScopeStatementProps,
} from "./workspace-scope-statement";

export interface CreateDestinationStatementProps extends WorkspaceScopeStatementProps {
  /**
   * The profile a new item lands in, supplied only while the aggregate is on.
   * Every scoped view already answers the question on screen, so the chip stays
   * absent there and the calm default is preserved.
   */
  profileDestination?: string | null;
}

/**
 * Where a new item lands, on both axes the operator can be looking through.
 *
 * Both halves are labels: scope changes happen at the menubar, never here. The
 * profile half is fixed text rather than a picker, because turning every shared
 * creation surface into a scope-aware control was rejected outright — the
 * owner-naming toast after the commit is what surfaces a misfile (ADR-005).
 *
 * Creation surfaces that opt into this composite share the same two-axis
 * guardrail; single-axis hosts may keep the bare workspace statement.
 */
export function CreateDestinationStatement({
  profileDestination,
  className,
  ...props
}: CreateDestinationStatementProps) {
  if (!profileDestination) return <WorkspaceScopeStatement className={className} {...props} />;
  return (
    <span
      data-slot="create-destination"
      className="inline-flex max-w-full flex-wrap items-center gap-1.5 align-middle"
    >
      <WorkspaceScopeStatement className={className} {...props} />
      <ProfileDestinationChip profile={profileDestination} />
    </span>
  );
}
