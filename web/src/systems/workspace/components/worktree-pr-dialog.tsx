import { GitBranchIcon, GitPullRequestIcon } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  dialogShellClass,
  Empty,
  EntityDialogBody,
  EntityDialogHeader,
  Field,
  FieldGroup,
  FieldLabel,
  Input,
  MonoId,
  Pill,
  Textarea,
} from "@compozy/ui";

import type { WorktreeExitLadder } from "../lib/worktree-exit-ladder";
import type { WorktreeExitPlanPayload } from "../types";
import { toWorktreePrDialogModel } from "../lib/worktree-pr-rows";
import {
  WorktreePrActionRows,
  type WorktreePrActionKey,
  type WorktreePrActionRow,
} from "./worktree-pr-action-rows";

interface WorktreePrDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  worktreeName: string;
  branch?: string;
  ladder: WorktreeExitLadder;
  /**
   * Daemon-supplied starting text. The web never composes it and never reads
   * the repository — template resolution happens server-side at action time.
   */
  prefill?: NonNullable<WorktreeExitPlanPayload["pr_prefill"]>;
  submittingKey?: WorktreePrActionKey;
  onSubmit: (row: WorktreePrActionRow, fields: { title: string; body: string }) => void;
}

/**
 * The pull-request step.
 *
 * Title and description start from whatever the daemon supplied and are
 * otherwise empty — template resolution happens server-side at action time, so
 * there is nothing honest for the browser to prefill on its own. With no forge
 * serving the remote, the create path is absent entirely and the compare link
 * is the whole step.
 */
export function WorktreePrDialog({
  open,
  onOpenChange,
  worktreeName,
  branch,
  ladder,
  prefill,
  submittingKey,
  onSubmit,
}: WorktreePrDialogProps) {
  const model = toWorktreePrDialogModel(ladder);
  const [title, setTitle] = useState(prefill?.title ?? "");
  const [body, setBody] = useState(prefill?.body ?? "");

  if (model.rows.length === 0) return null;

  const prNumber = ladder.forgeStatus?.pr_number;
  const requestNoun = ladder.forge?.request_noun ?? "pull request";
  const showBody = model.mode === "create" || Boolean(model.blockedReason);
  const headerTitle =
    model.mode === "existing" && prNumber != null
      ? `${requestNoun} #${prNumber} is already open`
      : (ladder.forge?.open_action_label ?? "Open in browser");

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`${showBody ? "grid-rows-[auto_minmax(0,1fr)_auto]" : "grid-rows-[auto_auto]"} ${dialogShellClass("sm")}`}
        data-blocked={model.blockedReason ? "" : undefined}
        data-mode={model.mode}
        data-slot="worktree-pr-dialog"
        data-submitting={submittingKey ? "" : undefined}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow={worktreeName}
          icon={GitPullRequestIcon}
          onClose={() => onOpenChange(false)}
          title={headerTitle}
        />
        {showBody ? (
          <EntityDialogBody className="flex flex-col gap-4">
            {model.mode === "create" ? (
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="worktree-pr-title">Title</FieldLabel>
                  <Input
                    disabled={Boolean(submittingKey)}
                    id="worktree-pr-title"
                    maxLength={300}
                    onChange={event => setTitle(event.target.value)}
                    placeholder="Title"
                    value={title}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="worktree-pr-body">Description</FieldLabel>
                  <Textarea
                    disabled={Boolean(submittingKey)}
                    id="worktree-pr-body"
                    onChange={event => setBody(event.target.value)}
                    placeholder="Description"
                    rows={5}
                    value={body}
                  />
                </Field>
              </FieldGroup>
            ) : null}
            {model.blockedReason ? (
              <Empty
                data-slot="worktree-pr-blocked"
                framed
                role="status"
                title={model.blockedReason}
              />
            ) : null}
          </EntityDialogBody>
        ) : null}
        <DialogFooter
          className="min-h-editor-footer items-center gap-3 max-[760px]:flex-col max-[760px]:items-stretch"
          variant="ruled"
        >
          <span
            className="mr-auto flex min-w-0 items-center gap-1.5 overflow-hidden [&_svg]:size-3 [&_svg]:shrink-0 [&_svg]:text-muted"
            data-slot="worktree-pr-base"
          >
            <GitBranchIcon aria-hidden="true" />
            {branch ? <MonoId copy={false} preserveCase size="sm" value={branch} /> : null}
            {ladder.base ? (
              <>
                <span aria-hidden="true" className="shrink-0 text-faint">
                  →
                </span>
                <span data-slot="worktree-pr-base-target">
                  <MonoId copy={false} preserveCase size="sm" value={ladder.base} />
                </span>
              </>
            ) : null}
            {prNumber != null ? (
              <Pill className="shrink-0" mono size="xs" tone="info">
                {`${requestNoun} #${prNumber}`}
              </Pill>
            ) : null}
          </span>
          <Button
            disabled={Boolean(submittingKey)}
            onClick={() => onOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <WorktreePrActionRows
            onSelect={row => onSubmit(row, { title, body })}
            rows={model.rows}
            submittingKey={submittingKey}
          />
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
