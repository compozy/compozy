import { GitBranchIcon, GitCommitHorizontalIcon } from "lucide-react";
import { useState } from "react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  dialogShellClass,
  DropdownMenuItem,
  Empty,
  EntityDialogBody,
  EntityDialogHeader,
  Field,
  FieldError,
  FieldLabel,
  MonoId,
  SplitButton,
  Textarea,
} from "@compozy/ui";

import { isCommitScopeEmpty, type WorktreeExitLadderRow } from "../lib/worktree-exit-ladder";
import type { WorktreeExitCommitScope } from "../types";
import { AGENT_MESSAGE_PROMPT } from "./worktree-commit-dialog-copy";
import { WorktreeScopeBlock } from "./worktree-scope-block";

interface WorktreeCommitDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  worktreeName: string;
  branch?: string;
  scope: WorktreeExitCommitScope;
  /** The commit-class rows from the plan; the first is the invoked primary. */
  actions: readonly WorktreeExitLadderRow[];
  /** Daemon reason when there is nothing to commit. Rendered verbatim. */
  blockedReason?: string;
  /** Captured hook output from a failed attempt; kept with the typed message. */
  hookOutput?: string;
  hookError?: string;
  isSubmitting?: boolean;
  /** Writes a reviewable prompt into the bound session's composer. Never sends it. */
  onStagePrompt?: (message: string) => void;
  /** Offered instead when no session is bound: start one in this worktree. */
  onStartSession?: () => void;
  promptStaged?: boolean;
  onCommit: (action: WorktreeExitLadderRow, message: string) => void;
}

/**
 * The staged prompt. It asks for text and says not to run anything, because
 * staging must never turn into an agent running git on the operator's behalf.
 */
const PENDING_LABEL: Record<string, string> = {
  commit: "Committing…",
  commit_push: "Committing & pushing…",
};

type AgentStagedState = "staged" | "available" | "unbound" | undefined;

function agentStagedState(
  onStagePrompt: WorktreeCommitDialogProps["onStagePrompt"],
  onStartSession: WorktreeCommitDialogProps["onStartSession"],
  promptStaged: boolean
): AgentStagedState {
  if (onStagePrompt) return promptStaged ? "staged" : "available";
  return onStartSession ? "unbound" : undefined;
}

function commitDialogState(hookError: string | undefined, empty: boolean) {
  if (hookError) return "hook-failed";
  return empty ? "empty" : "scope";
}

function primaryLabel(primary: WorktreeExitLadderRow, isSubmitting: boolean): string {
  if (!isSubmitting) return primary.label;
  return PENDING_LABEL[primary.action] ?? primary.label;
}

/** Offers the agent path: stage a prompt in the bound session, or start one. */
function CommitAgentAction({
  onStagePrompt,
  onStartSession,
  promptStaged,
}: Pick<WorktreeCommitDialogProps, "onStagePrompt" | "onStartSession"> & {
  promptStaged: boolean;
}) {
  if (onStagePrompt && promptStaged) {
    return (
      <p className="text-badge text-subtle" data-slot="worktree-commit-agent-staged">
        Prompt staged in the session composer.
      </p>
    );
  }
  if (onStagePrompt) {
    return (
      <Button
        data-slot="worktree-commit-agent-stage"
        onClick={() => onStagePrompt(AGENT_MESSAGE_PROMPT)}
        size="sm"
        type="button"
        variant="secondary"
      >
        Have the agent write it
      </Button>
    );
  }
  if (!onStartSession) return null;
  return (
    <Button
      data-slot="worktree-commit-agent-start"
      onClick={onStartSession}
      size="sm"
      type="button"
      variant="secondary"
    >
      Start a session in this worktree
    </Button>
  );
}

function CommitHookFeedback({
  hookError,
  hookOutput,
}: Pick<WorktreeCommitDialogProps, "hookError" | "hookOutput">) {
  return (
    <>
      {hookError ? <FieldError>{hookError}</FieldError> : null}
      {hookOutput ? (
        <pre
          className="mt-1.5 max-h-24 overflow-y-auto rounded-md bg-sunken px-2.5 py-2 font-mono text-eyebrow leading-relaxed whitespace-pre-wrap text-subtle"
          data-slot="worktree-commit-hook-output"
        >
          {hookOutput}
        </pre>
      ) : null}
    </>
  );
}

function CommitBranch({ branch }: { branch?: string }) {
  if (!branch) return null;
  return (
    <span
      className="mr-auto flex items-center gap-1.5 [&_svg]:size-3 [&_svg]:text-muted"
      data-slot="worktree-commit-branch"
    >
      <GitBranchIcon aria-hidden="true" />
      <MonoId copy={false} preserveCase size="sm" value={branch} />
    </span>
  );
}

/**
 * Confirms a commit and shows exactly what it will stage.
 *
 * The message field promises nothing it cannot keep: leaving it blank uses the
 * daemon's default message — there is no generation path in this release, so
 * the placeholder says "default", not "generated". A failed hook keeps the typed
 * message and shows the command's own output rather than a summary of it.
 */
export function WorktreeCommitDialog({
  open,
  onOpenChange,
  worktreeName,
  branch,
  scope,
  actions,
  blockedReason,
  hookOutput,
  hookError,
  isSubmitting = false,
  onStagePrompt,
  onStartSession,
  promptStaged = false,
  onCommit,
}: WorktreeCommitDialogProps) {
  const [message, setMessage] = useState("");
  const primary = actions[0];
  const alternatives = actions.slice(1);
  const empty = isCommitScopeEmpty(scope);

  if (!primary) return null;

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("sm")}`}
        data-agent-staged={agentStagedState(onStagePrompt, onStartSession, promptStaged)}
        data-message={message === "" ? "empty" : undefined}
        data-slot="worktree-commit-dialog"
        data-state={commitDialogState(hookError, empty)}
        data-submitting={isSubmitting ? "" : undefined}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow={worktreeName}
          icon={GitCommitHorizontalIcon}
          onClose={() => onOpenChange(false)}
          title="Commit your changes"
        />
        <EntityDialogBody className="flex flex-col gap-4">
          {empty ? (
            <Empty framed role="status" title={blockedReason ?? "Nothing to commit."} />
          ) : (
            <WorktreeScopeBlock scope={scope} />
          )}
          <Field>
            <FieldLabel htmlFor="worktree-commit-message">Commit message</FieldLabel>
            <Textarea
              autoFocus
              disabled={isSubmitting}
              id="worktree-commit-message"
              onChange={event => setMessage(event.target.value)}
              placeholder="Leave blank to use a default message."
              rows={4}
              value={message}
            />
            <CommitAgentAction
              onStagePrompt={onStagePrompt}
              onStartSession={onStartSession}
              promptStaged={promptStaged}
            />
            <CommitHookFeedback hookError={hookError} hookOutput={hookOutput} />
          </Field>
        </EntityDialogBody>
        <DialogFooter
          className="min-h-editor-footer items-center gap-3 max-[760px]:flex-col max-[760px]:items-stretch"
          variant="ruled"
        >
          <CommitBranch branch={branch} />
          <Button
            disabled={isSubmitting}
            onClick={() => onOpenChange(false)}
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
          <SplitButton
            blocked={empty || !primary.enabled}
            blockedReason={empty ? blockedReason : primary.blockedReason}
            disabled={isSubmitting}
            label={primaryLabel(primary, isSubmitting)}
            menuLabel="Commit actions"
            onAction={() => onCommit(primary, message)}
          >
            {alternatives.map(row => (
              <DropdownMenuItem
                data-action={row.action}
                disabled={!row.enabled}
                key={row.action}
                onClick={() => onCommit(row, message)}
                title={row.blockedReason}
              >
                {row.label}
              </DropdownMenuItem>
            ))}
          </SplitButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
