import {
  WorktreeCommitDialog,
  WorktreeExitProgressSurface,
  WorktreePrDialog,
  type WorktreeDetailModel,
} from "@/systems/workspace";

/** The exit flow's own dialogs and progress for an action started from the PR chip. */
export function SessionPullRequestExitSurface({ model }: { model: WorktreeDetailModel | null }) {
  if (!model) return null;
  return (
    <>
      {model.commitDialog ? <WorktreeCommitDialog {...model.commitDialog} /> : null}
      {model.prDialog ? <WorktreePrDialog {...model.prDialog} /> : null}
      <WorktreeExitProgressSurface model={model} />
    </>
  );
}
