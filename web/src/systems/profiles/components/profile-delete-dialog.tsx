import { AppWindow, Bell, Blocks, Trash2, TriangleAlert, UserRound, Wrench } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import type { DeleteProfilePlan, ProfileRemovalSummary } from "../types";

export interface ProfileDeleteDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  profile: string;
  plan: DeleteProfilePlan | undefined;
  planLoading: boolean;
  /** Non-zero means the flow routes to archive instead (US-006.AC-2). */
  workItems: number;
  isPending: boolean;
  error?: string | null;
  onDelete: (planRevision: string) => void;
  onArchiveInstead: () => void;
}

interface RemovalRow {
  key: keyof ProfileRemovalSummary;
  label: string;
  icon: LucideIcon;
}

// Only the rows an operator can act on or recognise; raw counters the daemon
// keeps for its own bookkeeping stay out of the confirmation.
const REMOVAL_ROWS: readonly RemovalRow[] = [
  { key: "agents", label: "Agents", icon: UserRound },
  { key: "skills", label: "Skills", icon: Wrench },
  { key: "loops", label: "Loops", icon: Blocks },
  { key: "mcp_servers", label: "MCP servers", icon: Blocks },
  { key: "config_keys", label: "Config overrides", icon: Wrench },
  { key: "credential_overrides", label: "Credential overrides", icon: Wrench },
  { key: "memory_entries", label: "Memory entries", icon: Bell },
  { key: "desktop_partitions", label: "Saved desktops", icon: AppWindow },
];

function Enumeration({ removed }: { removed: ProfileRemovalSummary }) {
  const rows = REMOVAL_ROWS.filter(row => removed[row.key] > 0);
  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-delete-enumeration"
    >
      <div className="flex min-h-8 items-center gap-2 px-3 text-small-body text-fg">
        <UserRound aria-hidden="true" className="size-3 shrink-0 text-subtle" />
        <span>Profile identity and settings</span>
      </div>
      {rows.map(row => (
        <div
          key={row.key}
          className="flex min-h-8 items-center gap-2 border-t border-line-soft px-3 text-small-body text-fg"
        >
          <row.icon aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          <span>{row.label}</span>
          <span className="ml-auto shrink-0 font-mono text-micro text-subtle">
            {removed[row.key]}
          </span>
        </div>
      ))}
    </div>
  );
}

function BlockerList({ blockers }: { blockers: readonly string[] }) {
  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-delete-blockers"
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
 * Delete a profile.
 *
 * A profile holding work is not offered a scarier confirmation — the dialog
 * states the fact and the primary action becomes archive. When deletion is
 * genuinely available it lists exactly what goes, and danger inks the confirm
 * button and nothing else.
 */
export function ProfileDeleteDialog({
  open,
  onOpenChange,
  profile,
  plan,
  planLoading,
  workItems,
  isPending,
  error = null,
  onDelete,
  onArchiveInstead,
}: ProfileDeleteDialogProps) {
  const ownsWork = workItems > 0;
  const blockers = plan?.approval_blockers ?? [];

  if (ownsWork) {
    return (
      <ConfirmDialog
        open={open}
        onOpenChange={onOpenChange}
        eyebrow="Profiles"
        icon={Trash2}
        iconTone="neutral"
        title={`Delete ${profile}`}
        description={`${profile} holds ${workItems} item${
          workItems === 1 ? "" : "s"
        }. Archive keeps them; delete is available once the profile is empty.`}
        tone="neutral"
        confirmLabel="Archive instead"
        cancelLabel="Cancel"
        isPending={isPending}
        onConfirm={onArchiveInstead}
        contentProps={{ "data-testid": "profile-delete-dialog" }}
        confirmButtonProps={{ "data-testid": "profile-delete-archive-instead" }}
        cancelButtonProps={{ "data-testid": "profile-delete-cancel" }}
      />
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      eyebrow="Profiles"
      icon={Trash2}
      iconTone="danger"
      title={`Delete ${profile}`}
      description={
        blockers.length > 0
          ? `Resolve these approval blockers before deleting ${profile}.`
          : plan !== undefined
            ? "Profile identity, settings, and the resources listed below will be deleted."
            : null
      }
      body={
        blockers.length > 0 ? (
          <BlockerList blockers={blockers} />
        ) : plan !== undefined ? (
          <Enumeration removed={plan.removed} />
        ) : null
      }
      note="This cannot be undone."
      tone="danger"
      confirmLabel="Delete profile"
      cancelLabel={blockers.length > 0 ? "Close" : "Cancel"}
      isPending={isPending || planLoading}
      {...(error !== null ? { error } : {})}
      onConfirm={() => {
        if (plan !== undefined && blockers.length === 0) onDelete(plan.revision);
      }}
      contentProps={{ "data-testid": "profile-delete-dialog" }}
      confirmButtonProps={{
        "data-testid": "profile-delete-confirm",
        disabled: plan === undefined || blockers.length > 0 || planLoading,
      }}
      cancelButtonProps={{ "data-testid": "profile-delete-cancel" }}
    />
  );
}
