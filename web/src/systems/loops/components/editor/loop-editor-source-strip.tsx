import { GitFork, Lock } from "lucide-react";

import { Button } from "@compozy/ui";

import type { LoopSource } from "../../types";

interface LoopEditorSourceStripProps {
  source: LoopSource;
  forking: boolean;
  onFork: () => void;
}

/**
 * The read-only notice for a non-workspace definition: state the reason, offer the one legal
 * verb. The strip is the NOTICE — enforcement lives in `definitionEditable`, which closes every
 * mutation path in the view-model. Validate stays legal here because it never writes.
 */
export function LoopEditorSourceStrip({ source, forking, onFork }: LoopEditorSourceStripProps) {
  return (
    <div
      className="flex flex-none items-center gap-2.5 border-b border-line bg-info-tint px-3.5 py-2 text-small-body text-fg"
      data-testid="loop-editor-readonly-strip"
    >
      <Lock aria-hidden="true" className="size-3.5 shrink-0 text-info" />
      <span>Read-only {source} source — fork before publishing.</span>
      <Button
        className="ml-auto"
        data-testid="loop-editor-fork"
        disabled={forking}
        onClick={onFork}
        size="sm"
        title="Creates a workspace copy you can edit and publish."
        type="button"
        variant="neutral"
      >
        <GitFork aria-hidden="true" className="size-3.5" />
        Fork &amp; edit
      </Button>
    </div>
  );
}
