import { isStalePlan, lifecycleErrorMessage } from "../hooks/use-profile-lifecycle";
import { useProfileLifecycleDialogs } from "../hooks/use-profile-lifecycle-dialogs";
import { symbolPatch } from "../lib/profile-identity";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileLens, ProfilePayload } from "../types";
import { ProfileArchiveDialog } from "./profile-archive-dialog";
import { ProfileCreateDialog } from "./profile-create-dialog";
import { ProfileDeleteDialog } from "./profile-delete-dialog";
import { ProfileIdentityDialog } from "./profile-identity-dialog";
import { ProfileRenameDialog } from "./profile-rename-dialog";
import { ProfileUnarchiveDialog } from "./profile-unarchive-dialog";

export interface ProfileLifecycleDialogsProps {
  profiles: readonly ProfilePayload[];
  lens: ProfileLens;
}

/**
 * The one host for every profile lifecycle dialog.
 *
 * Settings and the command palette both raise dialogs through the shared intent
 * store and land here, so there is exactly one place that reads a plan, quotes
 * its revision, and submits — the palette starts a flow, this owns it.
 */
export function ProfileLifecycleDialogs({ profiles, lens }: ProfileLifecycleDialogsProps) {
  const model = useProfileLifecycleDialogs(profiles);
  const { lifecycle, target } = model;
  const intent = lifecycle.intent;

  if (intent === null) return null;
  const dismiss = (open: boolean) => {
    if (!open) lifecycle.close();
  };

  if (intent.flow === "create") {
    return (
      <ProfileCreateDialog
        open
        onOpenChange={dismiss}
        existingCount={profiles.length}
        lens={lens}
        isPending={model.create.isPending}
        nameError={lifecycleErrorMessage(model.create.error)}
        initialName={intent.profile}
        onCreate={input =>
          model.create.mutate(
            {
              name: input.name,
              color: input.color,
              ...symbolPatch(input.symbol),
              activate: input.activate,
            },
            { onSuccess: lifecycle.close }
          )
        }
      />
    );
  }

  if (intent.flow === "update") {
    if (model.profile === undefined) return null;
    return (
      <ProfileIdentityDialog
        key={model.profile.id}
        open
        onOpenChange={dismiss}
        profile={model.profile}
        isPending={model.update.isPending}
        error={lifecycleErrorMessage(model.update.error)}
        onSave={patch =>
          model.update.mutate({ name: target, patch }, { onSuccess: lifecycle.close })
        }
      />
    );
  }

  if (intent.flow === "rename") {
    return (
      <ProfileRenameDialog
        open
        onOpenChange={dismiss}
        profile={target}
        newName={lifecycle.renameName}
        onNewNameChange={lifecycle.setRenameName}
        plan={model.renamePlan.data}
        planLoading={model.renamePlan.isFetching}
        acceptedRepos={lifecycle.acceptedRepos}
        onToggleRepo={lifecycle.toggleRepo}
        isPending={model.rename.isPending}
        error={
          lifecycleErrorMessage(model.rename.error) ?? lifecycleErrorMessage(model.renamePlan.error)
        }
        onRename={planRevision =>
          model.rename.mutate(
            {
              name: target,
              newName: lifecycle.renameName.trim(),
              planRevision,
              repos: lifecycle.acceptedRepos,
            },
            {
              onSuccess: lifecycle.close,
              onError: error => {
                if (isStalePlan(error)) model.renamePlan.refetch();
              },
            }
          )
        }
      />
    );
  }

  if (intent.flow === "archive") {
    return (
      <ProfileArchiveDialog
        open
        onOpenChange={dismiss}
        profile={target}
        plan={model.archivePlan.data}
        isPending={model.archive.isPending}
        error={
          lifecycleErrorMessage(model.archive.error) ??
          lifecycleErrorMessage(model.archivePlan.error)
        }
        onArchive={planRevision =>
          model.archive.mutate(
            { name: target, planRevision },
            {
              onSuccess: lifecycle.close,
              onError: error => {
                if (isStalePlan(error)) model.archivePlan.refetch();
              },
            }
          )
        }
      />
    );
  }

  if (intent.flow === "unarchive") {
    return (
      <ProfileUnarchiveDialog
        open
        onOpenChange={dismiss}
        profile={target}
        pausedAutomations={lifecycle.unarchiveResult?.paused_automations ?? null}
        isPending={model.unarchive.isPending}
        error={lifecycleErrorMessage(model.unarchive.error)}
        onUnarchive={() =>
          model.unarchive.mutate(target, { onSuccess: lifecycle.setUnarchiveResult })
        }
        onDone={lifecycle.close}
      />
    );
  }

  return (
    <ProfileDeleteDialog
      open
      onOpenChange={dismiss}
      profile={target}
      plan={model.deletePlan.data}
      workItems={model.workItems}
      isPending={model.remove.isPending}
      error={
        lifecycleErrorMessage(model.remove.error) ?? lifecycleErrorMessage(model.deletePlan.error)
      }
      onDelete={planRevision =>
        model.remove.mutate(
          { name: target, planRevision },
          {
            onSuccess: lifecycle.close,
            onError: error => {
              if (isStalePlan(error)) model.deletePlan.refetch();
            },
          }
        )
      }
      onArchiveInstead={() => openProfileDialog({ flow: "archive", profile: target })}
    />
  );
}
