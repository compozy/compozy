import { FilesIcon } from "lucide-react";

import { Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow, MonoId } from "@compozy/ui";

import type { WorktreeExitCommitScope } from "../types";
import { WorktreeDirtySignal } from "./worktree-signal-parts";

interface WorktreeScopeBlockProps {
  scope: WorktreeExitCommitScope;
}

/**
 * What a commit would actually stage.
 *
 * Untracked additions are listed by name rather than counted, because "3 files"
 * hides which three a stage-all is about to capture. When the daemon bounded
 * that list, the block says so instead of implying it is complete.
 */
export function WorktreeScopeBlock({ scope }: WorktreeScopeBlockProps) {
  const untracked = scope.untracked_files ?? [];

  return (
    <section
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-slot="worktree-scope-block"
      data-truncated={scope.untracked_truncated ? "" : undefined}
    >
      <header
        className="flex min-h-[30px] items-center gap-[7px] border-b border-line-soft px-[11px] text-small-body font-semibold text-fg [&_svg]:size-3 [&_svg]:text-muted"
        data-slot="worktree-scope-head"
      >
        <FilesIcon aria-hidden="true" />
        {`${scope.changed_files} files`}
        <span className="ml-auto">
          <WorktreeDirtySignal deletions={scope.deletions} dirty insertions={scope.insertions} />
        </span>
      </header>
      {untracked.length > 0 ? (
        <Collapsible defaultOpen>
          <CollapsibleTrigger
            className="flex w-full items-center gap-2 px-[11px] py-1.5 text-start"
            data-slot="worktree-scope-untracked-trigger"
          >
            <Eyebrow>Untracked additions</Eyebrow>
            <span className="text-badge text-subtle">{scope.untracked_total}</span>
          </CollapsibleTrigger>
          <CollapsibleContent>
            <ul className="max-h-40 overflow-y-auto p-1" data-slot="worktree-scope-list">
              {untracked.map(path => (
                <li
                  className="flex min-h-[25px] items-center gap-2.5 rounded-sm px-1.5 hover:bg-row-hover"
                  key={path}
                >
                  <MonoId copy={false} preserveCase size="sm" value={path} />
                </li>
              ))}
            </ul>
            {scope.untracked_truncated ? (
              <p
                className="px-1.5 pb-1 text-badge text-faint"
                data-slot="worktree-scope-truncation"
              >
                {`Showing ${untracked.length} of ${scope.untracked_total}`}
              </p>
            ) : null}
          </CollapsibleContent>
        </Collapsible>
      ) : null}
    </section>
  );
}
