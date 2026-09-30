import { Archive, Clock, TriangleAlert } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import type { ArchiveProfilePlan } from "../types";

export interface ProfileArchiveDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: string;
  plan: ArchiveProfilePlan | undefined;
  planLoading: boolean;
  isPending: boolean;
  error?: string | null;
  onArchive: (planRevision: string) => void;
}

function PausedList({ automations }: { automations: readonly string[] }) {
  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-archive-paused"
    >
      {automations.map(automation => (
        <div
          key={automation}
          className="flex min-h-8 items-center gap-2 border-t border-line-soft px-3 text-small-body text-fg first:border-t-0"
        >
          <Clock aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          <span>{automation}</span>
          <span className="ml-auto shrink-0 font-mono text-micro text-subtle">pauses</span>
        </div>
      ))}
    </div>
  );
}

function BlockerList({ blockers }: { blockers: readonly string[] }) {
  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-archive-blockers"
    >
      {blockers.map(blocker => (
        <div
          className="flex min-h-8 items-center gap-2 border-t border-line-soft px-3 text-small-body text-fg first:border-t-0"
          key={blocker}
        >
          <TriangleAlert aria-hidden="true" className="size-3 shrink-0 text-warning" />
          <span>{blocker}</span>
        </div>
      ))}
    </div>
  );
}

function pluralSuffix(count: number, one: string, many: string): string {
  return count === 1 ? one : many;
}

function runningReason(running: readonly string[], profile: string): string | null {
  if (running.length === 0) return null;
  const names = running.map(name => `"${name}"`).join(" and ");
  return `${running.length} session${pluralSuffix(running.length, " is", "s are")} still running in ${profile}. Stop ${names} to archive this profile.`;
}

function leasedRunsReason(leasedRuns: number, profile: string): string | null {
  if (leasedRuns <= 0) return null;
  return `${leasedRuns} leased run${pluralSuffix(leasedRuns, " is", "s are")} still active. Wait for the lease${pluralSuffix(leasedRuns, "", "s")} to end before archiving ${profile}.`;
}

function queuedRunsNote(plan: ArchiveProfilePlan | undefined): string | undefined {
  const queued = plan?.queued_runs_to_freeze ?? 0;
  if (queued <= 0) return undefined;
  return `${queued} queued run${pluralSuffix(queued, "", "s")} freeze with the profile and become claimable again after unarchive.`;
}

/** Derives what the archive confirmation says from the daemon's archive plan. */
function archiveDialogView(plan: ArchiveProfilePlan | undefined, profile: string) {
  const running = plan?.running_sessions ?? [];
  const blockers = plan?.approval_blockers ?? [];
  const leasedRuns = plan?.leased_runs ?? 0;
  const blocked = running.length > 0 || blockers.length > 0 || leasedRuns > 0;
  const blockingReasons = [
    runningReason(running, profile),
    blockers.length > 0 ? `Resolve these approval blockers before archiving ${profile}.` : null,
    leasedRunsReason(leasedRuns, profile),
  ].filter((reason): reason is string => reason !== null);
  return {
    blocked,
    blockers,
    automations: plan?.automations_to_pause ?? [],
    description: blocked
      ? blockingReasons.join(" ")
      : "Its work leaves scoped views and its automations pause. Nothing is deleted.",
    note: queuedRunsNote(plan),
  };
}

/** The list under the description, or `null` so the dialog renders no body slot. */
function archiveDialogBody({
  blocked,
  blockers,
  automations,
}: {
  blocked: boolean;
  blockers: readonly string[];
  automations: readonly string[];
}): React.ReactNode {
  if (blockers.length > 0) return <BlockerList blockers={blockers} />;
  if (!blocked && automations.length > 0) return <PausedList automations={automations} />;
  return null;
}

/**
 * Archive a profile.
 *
 * Nothing is deleted, so nothing here is danger-styled. Running sessions block
 * the action and are named as a warning — something is in use, not broken — and
 * the way forward is to stop them.
 */
export function ProfileArchiveDialog({
  open,
  onOpenChange,
  profile,
  plan,
  planLoading,
  isPending,
  error = null,
  onArchive,
}: ProfileArchiveDialogProps) {
  const { blocked, blockers, automations, description, note } = archiveDialogView(plan, profile);
  const body = archiveDialogBody({ blocked, blockers, automations });

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      eyebrow="Profiles"
      icon={blocked ? TriangleAlert : Archive}
      iconTone={blocked ? "warning" : "neutral"}
      title={`Archive ${profile}`}
      description={description}
      body={body}
      descriptionProps={blocked ? { "data-testid": "profile-archive-blocked" } : undefined}
      tone="warning"
      confirmLabel="Archive"
      cancelLabel={blocked ? "Close" : "Cancel"}
      isPending={isPending || planLoading}
      {...(error !== null ? { error } : {})}
      {...(note !== undefined ? { note } : {})}
      onConfirm={() => {
        if (plan !== undefined && !blocked) onArchive(plan.revision);
      }}
      contentProps={{ "data-testid": "profile-archive-dialog" }}
      confirmButtonProps={{
        "data-testid": "profile-archive-confirm",
        disabled: blocked || plan === undefined || planLoading,
      }}
      cancelButtonProps={{ "data-testid": "profile-archive-cancel" }}
    />
  );
}
