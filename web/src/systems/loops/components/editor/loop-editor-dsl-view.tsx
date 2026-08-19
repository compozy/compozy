import { cn } from "@compozy/ui";

import type { DslLine } from "../../lib/loop-dsl";

interface LoopEditorDslViewProps {
  lines: DslLine[];
}

export function LoopEditorDslView({ lines }: LoopEditorDslViewProps) {
  return (
    <div className="min-h-0 overflow-auto p-6" data-testid="loop-editor-dsl">
      <p className="mb-3 max-w-[74ch] text-form-label leading-relaxed text-subtle">
        A read-only view of the <span className="font-medium text-fg">compozy.loop/v1</span>{" "}
        definition you&apos;re editing — persisted to disk (the source of truth) on Publish. String
        values interpolate with Go templates <span className="font-mono">{"{{ }}"}</span>;
        conditions are CEL; node ids are snake_case.
      </p>
      <pre className="overflow-x-auto rounded-md border border-line-soft bg-rail p-4 font-mono text-small-body leading-relaxed text-fg">
        {lines.map((line, index) => (
          <div
            key={index}
            className={cn(
              "whitespace-pre px-1",
              line.offending && "rounded-xs bg-danger-tint text-danger",
              !line.offending && line.highlight && "bg-warning-tint"
            )}
            data-offending={line.offending ? "true" : undefined}
          >
            {line.text || " "}
          </div>
        ))}
      </pre>
    </div>
  );
}
