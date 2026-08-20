import { CodeBlock } from "@compozy/ui";

import type { DslLine } from "../../lib/loop-dsl";

interface LoopEditorDslViewProps {
  lines: DslLine[];
}

function dslHighlightLines(lines: DslLine[]): number[] {
  const highlightLines: number[] = [];
  for (const [index, line] of lines.entries()) {
    if (line.highlight || line.offending) {
      highlightLines.push(index + 1);
    }
  }
  return highlightLines;
}

export function LoopEditorDslView({ lines }: LoopEditorDslViewProps) {
  const code = lines.map(line => line.text).join("\n");

  return (
    <div className="min-h-0 overflow-auto p-6" data-testid="loop-editor-dsl">
      <p className="mb-3 max-w-[74ch] text-form-label leading-relaxed text-subtle">
        A read-only view of the <span className="font-medium text-fg">compozy.loop/v1</span>{" "}
        definition you&apos;re editing — persisted to disk (the source of truth) on Publish. String
        values interpolate with Go templates <span className="font-mono">{"{{ }}"}</span>;
        conditions are CEL; node ids are snake_case.
      </p>
      <CodeBlock code={code} copyable highlightLines={dslHighlightLines(lines)} language="yaml" />
    </div>
  );
}
