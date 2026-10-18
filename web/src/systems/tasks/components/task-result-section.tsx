import { FileX } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { Empty, Section } from "@compozy/ui";

import type { TaskRun } from "../types";
import { TaskExternalResult } from "./task-external-result";
import { TaskInlineResult } from "./task-inline-result";
import type { TaskResultPageController } from "./task-result-types";

export type { TaskResultPageController } from "./task-result-types";

export interface TaskResultSectionProps extends Omit<
  ComponentProps<typeof Section>,
  "children" | "label"
> {
  emptyMessage: ReactNode;
  external?: TaskResultPageController;
  jsonTestId?: string;
  result: TaskRun["result"];
  resultBytes?: number;
  resultRef?: string;
}

/** Shared bounded result presentation for task and run detail surfaces. */
export function TaskResultSection({
  result,
  resultBytes = 0,
  resultRef,
  external,
  emptyMessage,
  jsonTestId,
  ...sectionProps
}: TaskResultSectionProps) {
  return (
    <Section label="Result" {...sectionProps}>
      {resultRef && external ? (
        <TaskExternalResult controller={external} resultBytes={resultBytes} resultRef={resultRef} />
      ) : result == null ? (
        <div className="rounded-lg bg-card px-4 py-5 shadow-card">
          <Empty icon={FileX} size="compact" title={emptyMessage} />
        </div>
      ) : (
        <TaskInlineResult jsonTestId={jsonTestId} result={result} />
      )}
    </Section>
  );
}
