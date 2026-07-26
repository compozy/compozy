import { Plus } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Input,
  Label,
  RadioCard,
} from "@agh/ui";

import type { ToolApprovalGrantSetDraft } from "../hooks/use-tool-approval-grants-panel";

export interface ToolApprovalGrantSetDialogProps {
  draft: ToolApprovalGrantSetDraft;
  open: boolean;
  isPending: boolean;
  canSubmit: boolean;
  error: string | null;
  onChange: (draft: ToolApprovalGrantSetDraft) => void;
  onOpenChange: (open: boolean) => void;
  onSubmit: () => void;
}

/** Explicit wider-decision editor. Exact input grants remain prompt-origin only. */
export function ToolApprovalGrantSetDialog({
  draft,
  open,
  isPending,
  canSubmit,
  error,
  onChange,
  onOpenChange,
  onSubmit,
}: ToolApprovalGrantSetDialogProps) {
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent
        className="grid-rows-[auto_minmax(0,1fr)_auto_auto] max-h-[min(var(--height-modal-tall),calc(100vh-2rem))] gap-0 overflow-hidden p-0 sm:max-w-xl"
        data-testid="tool-approval-grant-set-dialog"
        showCloseButton={false}
      >
        <form
          className="contents"
          onSubmit={event => {
            event.preventDefault();
            onSubmit();
          }}
        >
          <DialogHeader variant="ruled">
            <DialogTitle>Set a broader decision</DialogTitle>
            <DialogDescription>
              Remember a native-tool decision beyond one exact input. The workspace tool policy
              still decides whether a matching call is allowed.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 overflow-y-auto px-5 py-4">
            <div className="flex flex-col gap-5">
              <div className="flex flex-col gap-2">
                <Label className="eyebrow text-muted">Scope</Label>
                <div
                  aria-label="Remembered decision scope"
                  className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                  role="radiogroup"
                >
                  <RadioCard
                    data-testid="tool-approval-grant-scope-agent"
                    description="One named agent and tool, across every input"
                    onSelect={() => onChange({ ...draft, scope: "agent" })}
                    selected={draft.scope === "agent"}
                    title="Agent + tool"
                  />
                  <RadioCard
                    data-testid="tool-approval-grant-scope-tool"
                    description="This tool for every agent and input in the workspace"
                    onSelect={() => onChange({ ...draft, scope: "tool" })}
                    selected={draft.scope === "tool"}
                    title="Tool-wide"
                  />
                </div>
              </div>

              <div className="flex flex-col gap-1.5">
                <Label className="eyebrow text-muted" htmlFor="tool-approval-grant-tool-id">
                  Tool ID
                </Label>
                <Input
                  autoComplete="off"
                  className="font-mono"
                  data-testid="tool-approval-grant-tool-id"
                  id="tool-approval-grant-tool-id"
                  onChange={event => onChange({ ...draft, toolId: event.target.value })}
                  placeholder="agh__workspace_list"
                  required
                  value={draft.toolId}
                />
              </div>

              {draft.scope === "agent" ? (
                <div className="flex flex-col gap-1.5">
                  <Label className="eyebrow text-muted" htmlFor="tool-approval-grant-agent-name">
                    Agent name
                  </Label>
                  <Input
                    autoComplete="off"
                    className="font-mono"
                    data-testid="tool-approval-grant-agent-name"
                    id="tool-approval-grant-agent-name"
                    onChange={event => onChange({ ...draft, agentName: event.target.value })}
                    placeholder="claude-code"
                    required
                    value={draft.agentName}
                  />
                </div>
              ) : null}

              <div className="flex flex-col gap-2">
                <Label className="eyebrow text-muted">Decision</Label>
                <div
                  aria-label="Remembered decision"
                  className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                  role="radiogroup"
                >
                  <RadioCard
                    data-testid="tool-approval-grant-decision-allow"
                    description="Skip future prompts when the workspace policy permits the call"
                    onSelect={() => onChange({ ...draft, decision: "allow" })}
                    selected={draft.decision === "allow"}
                    title="Allow"
                  />
                  <RadioCard
                    data-testid="tool-approval-grant-decision-reject"
                    description="Reject matching calls without prompting"
                    onSelect={() => onChange({ ...draft, decision: "reject" })}
                    selected={draft.decision === "reject"}
                    title="Reject"
                  />
                </div>
              </div>
            </div>
          </div>

          {error ? (
            <div
              className="border-t border-line px-5 py-3 text-xs text-danger"
              data-testid="tool-approval-grant-set-error"
            >
              {error}
            </div>
          ) : null}

          <DialogFooter
            className="mx-0 mb-0 rounded-b-xl border-t border-line bg-transparent px-5 py-3"
            variant="ruled"
          >
            <Button
              disabled={isPending}
              onClick={() => onOpenChange(false)}
              size="sm"
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
            <Button
              data-testid="tool-approval-grant-set-confirm"
              disabled={!canSubmit || isPending}
              size="sm"
              type="submit"
            >
              <Plus aria-hidden="true" className="size-3" />
              Set decision
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
