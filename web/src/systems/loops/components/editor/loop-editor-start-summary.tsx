import type { LoopStartBinding } from "../../types";
import { MonoTag } from "../mono-tag";

interface LoopEditorStartSummaryProps {
  start: LoopStartBinding[];
}

export function LoopEditorStartSummary({ start }: LoopEditorStartSummaryProps) {
  if (start.length === 0) return null;
  return (
    <div
      className="pointer-events-none absolute left-3.5 top-3 z-10 flex items-center gap-1.5 rounded-md border border-line-soft bg-canvas-soft px-2.5 py-1.5"
      data-testid="loop-editor-start-summary"
      title="Declared in start[]; authored in the definition file/agent (read-only here)"
    >
      <MonoTag className="text-pill-group-badge tracking-[0.07em] text-faint">start</MonoTag>
      {start.map(binding => (
        <MonoTag
          key={JSON.stringify(binding)}
          className="rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge tracking-[0.04em] text-subtle"
        >
          {binding.kind}
        </MonoTag>
      ))}
      <span className="ml-1 text-badge text-faint">declared in start[]</span>
    </div>
  );
}
