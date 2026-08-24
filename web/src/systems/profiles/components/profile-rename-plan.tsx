import { ArrowRight, CircleSlash, HardDrive, Moon } from "lucide-react";

import { Checkbox, cn } from "@compozy/ui";

import type { RenameProfilePlan } from "../types";

export interface ProfileRenamePlanProps {
  plan: RenameProfilePlan;
  newName: string;
  /** Workspace ids whose repo folder the operator accepted. */
  acceptedRepos: readonly string[];
  onToggleRepo: (workspaceId: string) => void;
}

function Tier({ title, tag, children }: { title: string; tag: string; children: React.ReactNode }) {
  return (
    <div className="border-t border-line-soft px-3 py-2.5 first:border-t-0">
      <div className="flex items-baseline gap-2">
        <span className="text-small-body font-medium text-fg">{title}</span>
        <span className="ml-auto shrink-0 text-micro font-medium text-faint">{tag}</span>
      </div>
      <div className="mt-1 flex flex-col gap-1">{children}</div>
    </div>
  );
}

const ROW_CLASS = "flex min-h-7 items-center gap-2 text-small-body text-muted";
const CODE_CLASS = "rounded-xxs bg-badge-fill px-1 py-px font-mono text-micro text-muted";

/**
 * What a rename will do, grouped by who decides.
 *
 * Machine folders move on their own and are stated, not asked. Repo folders are
 * offers — accepting one turns it into a pending change the operator commits in
 * their own project. Extension placements that point at the old name will sleep;
 * that is reported, never negotiated. An empty tier is not rendered, because an
 * empty tier is a ghost.
 */
export function ProfileRenamePlan({
  plan,
  newName,
  acceptedRepos,
  onToggleRepo,
}: ProfileRenamePlanProps) {
  const target = newName.trim();
  const accepted = new Set(acceptedRepos);
  return (
    <div
      className="overflow-hidden rounded-md border border-line"
      data-testid="profile-rename-plan"
    >
      {plan.machine_folders.length > 0 ? (
        <Tier title="Machine folders" tag="automatic">
          {plan.machine_folders.map(folder => (
            <div key={folder} className={ROW_CLASS}>
              <HardDrive aria-hidden="true" className="size-3 shrink-0 text-subtle" />
              <code className={CODE_CLASS}>{folder}</code>
              <ArrowRight aria-hidden="true" className="size-2.5 shrink-0 text-subtle" />
              <code className={CODE_CLASS}>{target}</code>
            </div>
          ))}
        </Tier>
      ) : null}
      {plan.repo_candidates.length > 0 ? (
        <Tier title="Repo folders" tag="your choice — pending changes to commit">
          {plan.repo_candidates.map(candidate => (
            <label
              key={candidate.workspace_id}
              className={cn(ROW_CLASS, "cursor-pointer")}
              data-testid={`profile-rename-repo-${candidate.workspace_id}`}
            >
              <Checkbox
                checked={accepted.has(candidate.workspace_id)}
                onCheckedChange={() => onToggleRepo(candidate.workspace_id)}
                aria-label={`Rename the folder in ${candidate.workspace}`}
              />
              <span>{candidate.workspace}</span>
              <span aria-hidden="true">·</span>
              <code className={CODE_CLASS}>{candidate.path}</code>
            </label>
          ))}
        </Tier>
      ) : null}
      {plan.dormant_placements.length > 0 ? (
        <Tier title="Extension placements" tag="informational">
          {plan.dormant_placements.map(placement => (
            <div
              key={`${placement.extension}-${placement.resource}`}
              className={cn(ROW_CLASS, "text-faint")}
              data-testid="profile-rename-dormant"
            >
              <Moon aria-hidden="true" className="size-3 shrink-0" />
              <span>
                {placement.extension} content for {placement.profile} will sleep until a profile
                uses that name again
              </span>
            </div>
          ))}
        </Tier>
      ) : null}
      {plan.vault_ref_rewrites > 0 ? (
        <Tier title="Stored references" tag="automatic">
          <div className={ROW_CLASS}>
            <CircleSlash aria-hidden="true" className="size-3 shrink-0 text-subtle" />
            <span>
              {plan.vault_ref_rewrites} credential reference
              {plan.vault_ref_rewrites === 1 ? "" : "s"} repointed
            </span>
          </div>
        </Tier>
      ) : null}
    </div>
  );
}
