import type { ComponentProps, ReactNode } from "react";

import { JsonViewer, Markdown, Section } from "@agh/ui";

import type { TaskRun } from "../types";

export interface TaskResultSectionProps extends Omit<
  ComponentProps<typeof Section>,
  "children" | "label"
> {
  result: TaskRun["result"];
  emptyMessage: ReactNode;
  jsonTestId?: string;
}

/** Shared result presentation for task and run detail surfaces. */
export function TaskResultSection({
  result,
  emptyMessage,
  jsonTestId,
  ...sectionProps
}: TaskResultSectionProps) {
  return (
    <Section label="Result" {...sectionProps}>
      {result == null ? (
        <p className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5 text-small-body text-muted">
          {emptyMessage}
        </p>
      ) : typeof result === "string" ? (
        <div className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5">
          <Markdown>{result}</Markdown>
        </div>
      ) : (
        <JsonViewer data-testid={jsonTestId} value={result} />
      )}
    </Section>
  );
}
