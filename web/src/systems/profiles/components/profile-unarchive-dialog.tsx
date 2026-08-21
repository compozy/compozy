import { ArchiveRestore, Clock } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

export interface ProfileUnarchiveDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: string;
  /**
   * The automations the daemon reported as still paused. Only known after the
   * unarchive returns — there is no unarchive plan endpoint, so this dialog
   * confirms first and reports second rather than inventing a preview.
   */
  pausedAutomations: readonly string[] | null;
  isPending: boolean;
  error?: string | null;
  onUnarchive: () => void;
  onDone: () => void;
}

function ReactivationList({ automations }: { automations: readonly string[] }) {
  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-unarchive-paused"
    >
      {automations.map(automation => (
        <div
          key={automation}
          className="flex min-h-8 items-center gap-2 border-t border-line-soft px-3 text-small-body text-fg first:border-t-0"
        >
          <Clock aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          <span>{automation}</span>
          <span className="ml-auto shrink-0 font-mono text-micro text-subtle">paused</span>
        </div>
      ))}
    </div>
  );
}

/**
 * Unarchive a profile.
 *
 * The profile itself comes back immediately; its automations do not. Each stays
 * paused until it is turned on deliberately — silently waking a year-old digest
 * is the failure mode this guards against.
 */
export function ProfileUnarchiveDialog({
  open,
  onOpenChange,
  profile,
  pausedAutomations,
  isPending,
  error = null,
  onUnarchive,
  onDone,
}: ProfileUnarchiveDialogProps) {
  const settled = pausedAutomations !== null;
  const description = settled ? (
    pausedAutomations.length > 0 ? (
      <>
        <span>{profile} is back. These automations stay paused until you turn each one on.</span>
        <ReactivationList automations={pausedAutomations} />
      </>
    ) : (
      <span>{profile} is back. It had no paused automations.</span>
    )
  ) : (
    <span>
      Its work returns to scoped views. Any automations paused at archive stay paused until you turn
      them on.
    </span>
  );

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={settled ? onDone : onOpenChange}
      eyebrow="Profiles"
      icon={ArchiveRestore}
      iconTone="neutral"
      title={`Unarchive ${profile}`}
      description={description}
      tone="neutral"
      confirmLabel={settled ? "Done" : "Unarchive"}
      cancelLabel="Cancel"
      isPending={isPending}
      {...(error !== null ? { error } : {})}
      onConfirm={settled ? onDone : onUnarchive}
      contentProps={{ "data-testid": "profile-unarchive-dialog" }}
      confirmButtonProps={{ "data-testid": "profile-unarchive-confirm" }}
      cancelButtonProps={{
        "data-testid": "profile-unarchive-cancel",
        ...(settled ? { className: "hidden" } : {}),
      }}
    />
  );
}
