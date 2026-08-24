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
  const running = plan?.running_sessions ?? [];
  const blockers = plan?.approval_blockers ?? [];
  const leasedRuns = plan?.leased_runs ?? 0;
  const blocked = running.length > 0 || blockers.length > 0 || leasedRuns > 0;
  const automations = plan?.automations_to_pause ?? [];

  const blockingReasons = [
    running.length > 0
      ? `${running.length} session${running.length === 1 ? " is" : "s are"} still running in ${profile}. Stop ${running
          .map(name => `"${name}"`)
          .join(" and ")} to archive this profile.`
      : null,
    blockers.length > 0 ? `Resolve these approval blockers before archiving ${profile}.` : null,
    leasedRuns > 0
      ? `${leasedRuns} leased run${leasedRuns === 1 ? " is" : "s are"} still active. Wait for the lease${leasedRuns === 1 ? "" : "s"} to end before archiving ${profile}.`
      : null,
  ].filter((reason): reason is string => reason !== null);
  const description = blocked
    ? blockingReasons.join(" ")
    : "Its work leaves scoped views and its automations pause. Nothing is deleted.";
  const body =
    blockers.length > 0 ? (
      <BlockerList blockers={blockers} />
    ) : !blocked && automations.length > 0 ? (
      <PausedList automations={automations} />
    ) : null;

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      eyebrow="Profiles"
      icon={blocked ? TriangleAlert : Archive}
      iconTone="neutral"
      title={`Archive ${profile}`}
      description={description}
      body={body}
      descriptionProps={blocked ? { "data-testid": "profile-archive-blocked" } : undefined}
      tone="warning"
      confirmLabel="Archive"
      cancelLabel={blocked ? "Close" : "Cancel"}
      isPending={isPending || planLoading}
      {...(error !== null ? { error } : {})}
      {...(plan !== undefined && plan.queued_runs_to_freeze > 0
        ? {
            note: `${plan.queued_runs_to_freeze} queued run${
              plan.queued_runs_to_freeze === 1 ? "" : "s"
            } freeze with the profile and become claimable again after unarchive.`,
          }
        : {})}
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
