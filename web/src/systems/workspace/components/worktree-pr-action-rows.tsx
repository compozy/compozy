import { ExternalLinkIcon, GitPullRequestDraftIcon, GitPullRequestIcon } from "lucide-react";
import type { ReactNode } from "react";

import { DropdownMenuItem, Spinner, SplitButton } from "@compozy/ui";

export type WorktreePrActionKey = "draft" | "create" | "view" | "browser";

export interface WorktreePrActionRow {
  key: WorktreePrActionKey;
  label: string;
  primary?: boolean;
  blocked?: boolean;
  /** Verbatim daemon reason. */
  reason?: string;
  url?: string;
}

interface WorktreePrActionRowsProps {
  rows: readonly WorktreePrActionRow[];
  submittingKey?: WorktreePrActionKey;
  onSelect: (row: WorktreePrActionRow) => void;
}

const ICON: Record<WorktreePrActionKey, ReactNode> = {
  draft: <GitPullRequestDraftIcon aria-hidden="true" />,
  create: <GitPullRequestIcon aria-hidden="true" />,
  view: <ExternalLinkIcon aria-hidden="true" />,
  browser: <ExternalLinkIcon aria-hidden="true" />,
};

const PENDING_LABEL: Record<WorktreePrActionKey, string> = {
  draft: "Opening draft…",
  create: "Opening…",
  view: "Opening…",
  browser: "Opening…",
};

/**
 * The PR step's footer control. The primary verb is the split action; draft
 * and the compare link sit in the menu — the same pattern as commit. An empty
 * menu collapses to a single button. Every item exists because the payload
 * offers it; nothing is speculative.
 */
export function WorktreePrActionRows({ rows, submittingKey, onSelect }: WorktreePrActionRowsProps) {
  const primary = rows.find(row => row.primary) ?? rows[0];
  if (!primary) return null;
  const alternatives = rows.filter(row => row.key !== primary.key);

  return (
    <SplitButton
      blocked={primary.blocked}
      blockedReason={primary.reason}
      data-action={primary.key}
      data-slot="worktree-pr-action"
      disabled={submittingKey !== undefined}
      icon={submittingKey === primary.key ? <Spinner className="size-3" /> : ICON[primary.key]}
      label={submittingKey === primary.key ? PENDING_LABEL[primary.key] : primary.label}
      menuLabel="PR actions"
      onAction={() => onSelect(primary)}
    >
      {alternatives.length === 0
        ? undefined
        : alternatives.map(row => (
            <DropdownMenuItem
              data-action={row.key}
              data-slot="worktree-pr-action"
              disabled={row.blocked || submittingKey !== undefined}
              key={row.key}
              onClick={() => onSelect(row)}
            >
              {ICON[row.key]}
              <span>{row.label}</span>
              {row.blocked && row.reason ? (
                <span className="ml-auto min-w-0 flex-1 truncate text-right text-micro text-faint">
                  {row.reason}
                </span>
              ) : null}
            </DropdownMenuItem>
          ))}
    </SplitButton>
  );
}
