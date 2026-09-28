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
  FieldLabel,
  RadioCard,
} from "@compozy/ui";

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
            <DialogTitle>Add a rule</DialogTitle>
            <DialogDescription>
              Remember a decision for a tool beyond one exact request. Your project&apos;s tool
              settings still decide what is allowed.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 overflow-y-auto px-5 py-4">
            <div className="flex flex-col gap-5">
              <div className="flex flex-col gap-2">
                <FieldLabel>Applies to</FieldLabel>
                <div
                  aria-label="Remembered decision scope"
                  className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                  role="radiogroup"
                >
                  <RadioCard
                    data-testid="tool-approval-grant-scope-agent"
                    description="This tool, for one agent, on every request"
                    onSelect={() => onChange({ ...draft, scope: "agent" })}
                    selected={draft.scope === "agent"}
                    title="One agent"
                  />
                  <RadioCard
                    data-testid="tool-approval-grant-scope-tool"
                    description="This tool, for every agent in the project"
                    onSelect={() => onChange({ ...draft, scope: "tool" })}
                    selected={draft.scope === "tool"}
                    title="Every agent"
                  />
                </div>
              </div>

              <div className="flex flex-col gap-1.5">
                <FieldLabel htmlFor="tool-approval-grant-tool-id">Tool</FieldLabel>
                <Input
                  autoComplete="off"
                  className="font-mono"
                  data-testid="tool-approval-grant-tool-id"
                  id="tool-approval-grant-tool-id"
                  onChange={event => onChange({ ...draft, toolId: event.target.value })}
                  placeholder="compozy__workspace_list"
                  required
                  value={draft.toolId}
                />
              </div>

              {draft.scope === "agent" ? (
                <div className="flex flex-col gap-1.5">
                  <FieldLabel htmlFor="tool-approval-grant-agent-name">Agent</FieldLabel>
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
                <FieldLabel>Decision</FieldLabel>
                <div
                  aria-label="Remembered decision"
                  className="grid grid-cols-1 gap-2 sm:grid-cols-2"
                  role="radiogroup"
                >
                  <RadioCard
                    data-testid="tool-approval-grant-decision-allow"
                    description="Don't ask again when your project's settings permit it"
                    onSelect={() => onChange({ ...draft, decision: "allow" })}
                    selected={draft.decision === "allow"}
                    title="Allow"
                  />
                  <RadioCard
                    data-testid="tool-approval-grant-decision-reject"
                    description="Block matching requests without asking"
                    onSelect={() => onChange({ ...draft, decision: "reject" })}
                    selected={draft.decision === "reject"}
                    title="Block"
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
              Add rule
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
