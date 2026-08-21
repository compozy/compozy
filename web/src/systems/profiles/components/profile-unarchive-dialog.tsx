import { useState } from "react";
import { ArchiveRestore, Clock } from "lucide-react";

import { ConfirmDialog, Switch } from "@compozy/ui";

import { parseProfileAutomationIdentity } from "../lib/profile-automation-identity";

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
  onSetAutomationEnabled: (identity: string, enabled: boolean) => Promise<void>;
  onDone: () => void;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Failed to update this automation.";
}

function ReactivationList({
  automations,
  onSetEnabled,
}: {
  automations: readonly string[];
  onSetEnabled: (identity: string, enabled: boolean) => Promise<void>;
}) {
  const [enabled, setEnabled] = useState<ReadonlySet<string>>(() => new Set());
  const [pending, setPending] = useState<ReadonlySet<string>>(() => new Set());
  const [errors, setErrors] = useState<ReadonlyMap<string, string>>(() => new Map());

  const setAutomationEnabled = async (identity: string, next: boolean) => {
    setPending(current => new Set(current).add(identity));
    setErrors(current => {
      const updated = new Map(current);
      updated.delete(identity);
      return updated;
    });
    try {
      await onSetEnabled(identity, next);
      setEnabled(current => {
        const updated = new Set(current);
        if (next) updated.add(identity);
        else updated.delete(identity);
        return updated;
      });
    } catch (error) {
      setErrors(current => new Map(current).set(identity, errorMessage(error)));
    } finally {
      setPending(current => {
        const updated = new Set(current);
        updated.delete(identity);
        return updated;
      });
    }
  };

  return (
    <div
      className="mt-2 overflow-hidden rounded-md border border-line"
      data-testid="profile-unarchive-paused"
    >
      {automations.map(automation => {
        const parsed = parseProfileAutomationIdentity(automation);
        const isEnabled = enabled.has(automation);
        const isPending = pending.has(automation);
        const rowError = errors.get(automation);
        const label = `${isEnabled ? "Pause" : "Reactivate"} ${parsed.id}`;
        return (
          <div key={automation} className="border-t border-line-soft px-3 py-2 first:border-t-0">
            <div className="flex min-h-8 items-center gap-2 text-small-body text-fg">
              <Clock aria-hidden="true" className="size-3 shrink-0 text-subtle" />
              <span className="min-w-0 flex-1 truncate">{parsed.id}</span>
              <span className="shrink-0 font-mono text-micro text-subtle">
                {isEnabled ? "active" : "paused"}
              </span>
              <Switch
                aria-label={label}
                checked={isEnabled}
                disabled={isPending}
                onCheckedChange={next => void setAutomationEnabled(automation, next)}
              />
            </div>
            {rowError ? (
              <p className="mt-1 text-micro text-danger" role="alert">
                {rowError}
              </p>
            ) : null}
          </div>
        );
      })}
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
  onSetAutomationEnabled,
  onDone,
}: ProfileUnarchiveDialogProps) {
  const settled = pausedAutomations !== null;
  const description = settled ? (
    pausedAutomations.length > 0 ? (
      <span>{profile} is back. These automations stay paused until you turn each one on.</span>
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
      body={
        settled && pausedAutomations.length > 0 ? (
          <ReactivationList automations={pausedAutomations} onSetEnabled={onSetAutomationEnabled} />
        ) : null
      }
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
