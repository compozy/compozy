import type { LoopStartBinding } from "../../types";
import { MonoTag } from "../mono-tag";

interface LoopEditorStartSummaryProps {
  start: LoopStartBinding[];
}

/**
 * The read-only Start summary pinned at the canvas origin (design §4.6 / §9.14): the
 * definition's declared `start[]` kinds as chips, so the graph and the DSL tell the same
 * initiation story. There is no graph-side start editing in v1; `start[]` is authored in
 * the definition file/agent (FS-as-truth, ADR-015) and round-trips through the codec like
 * any other field. The editable DSL surface for `start[]` is a v1 deferral.
 */
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
