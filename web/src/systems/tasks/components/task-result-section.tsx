import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import { useState, type ComponentProps, type ReactNode } from "react";

import {
  Button,
  CodeBlock,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  JsonViewer,
  Markdown,
  Section,
  Skeleton,
} from "@compozy/ui";

import type { TaskRun, TaskRunResultPage } from "../types";

const NUMBER_FORMATTER = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });

export interface TaskResultPageController {
  canGoNext: boolean;
  canGoPrevious: boolean;
  copyState: "idle" | "copying" | "copied" | "error";
  errorMessage: string | null;
  isLoading: boolean;
  onCopy: () => Promise<void>;
  onNextPage: () => void;
  onOpenChange: (open: boolean) => void;
  onPreviousPage: () => void;
  onRetry: () => void;
  open: boolean;
  page: TaskRunResultPage | undefined;
  pageText: string;
}

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
        <ExternalTaskResult controller={external} resultBytes={resultBytes} resultRef={resultRef} />
      ) : result == null ? (
        <p className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5 text-small-body text-muted">
          {emptyMessage}
        </p>
      ) : (
        <InlineTaskResult jsonTestId={jsonTestId} result={result} />
      )}
    </Section>
  );
}

function InlineTaskResult({
  jsonTestId,
  result,
}: {
  jsonTestId?: string;
  result: NonNullable<TaskRun["result"]>;
}) {
  const [open, setOpen] = useState(false);
  return (
    <Collapsible onOpenChange={setOpen} open={open}>
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {!open ? (
          <div className="max-h-44 overflow-hidden px-4 py-3.5">
            <InlineResultValue jsonTestId={jsonTestId} result={result} />
          </div>
        ) : null}
        <CollapsibleContent>
          <div className="max-h-96 overflow-auto px-4 py-3.5">
            <InlineResultValue jsonTestId={jsonTestId} result={result} />
          </div>
        </CollapsibleContent>
        <div className="flex justify-end border-t border-line-soft px-2 py-1.5">
          <CollapsibleTrigger
            className="group/result-trigger"
            render={
              <Button size="sm" type="button" variant="ghost">
                {open ? "Collapse result" : "Expand result"}
                <ChevronDown
                  aria-hidden="true"
                  className="transition-transform duration-fast group-data-panel-open/result-trigger:rotate-180 motion-reduce:transition-none"
                />
              </Button>
            }
          />
        </div>
      </div>
    </Collapsible>
  );
}

function InlineResultValue({
  jsonTestId,
  result,
}: {
  jsonTestId?: string;
  result: NonNullable<TaskRun["result"]>;
}) {
  return typeof result === "string" ? (
    <Markdown>{result}</Markdown>
  ) : (
    <JsonViewer data-testid={jsonTestId} value={result} />
  );
}

function ExternalTaskResult({
  controller,
  resultBytes,
  resultRef,
}: {
  controller: TaskResultPageController;
  resultBytes: number;
  resultRef: string;
}) {
  const copyStatus =
    controller.copyState === "copied"
      ? "Result copied."
      : controller.copyState === "error"
        ? "Couldn't copy result. Try again."
        : controller.copyState === "copying"
          ? "Copying result."
          : "";
  return (
    <Collapsible onOpenChange={controller.onOpenChange} open={controller.open}>
      <div className="rounded-lg border border-line bg-canvas-soft">
        <div className="flex flex-wrap items-center justify-between gap-3 px-3 py-2.5">
          <span className="text-small-body text-muted tabular-nums">
            {formatByteCount(resultBytes)}
          </span>
          <CollapsibleTrigger
            className="group/result-trigger"
            render={
              <Button size="sm" type="button" variant="outline">
                {controller.open ? "Hide result" : "View result"}
                <ChevronDown
                  aria-hidden="true"
                  className="transition-transform duration-fast group-data-panel-open/result-trigger:rotate-180 motion-reduce:transition-none"
                />
              </Button>
            }
          />
        </div>
        <CollapsibleContent className="border-t border-line-soft px-3 py-3">
          {controller.isLoading ? (
            <div aria-label="Loading result" className="flex flex-col gap-2" role="status">
              <Skeleton className="h-7 w-48" />
              <Skeleton className="h-40 rounded-lg" />
            </div>
          ) : controller.errorMessage ? (
            <div className="flex flex-wrap items-center justify-between gap-3" role="alert">
              <p className="text-small-body text-danger">{controller.errorMessage}</p>
              <Button onClick={controller.onRetry} size="sm" type="button" variant="outline">
                Retry
              </Button>
            </div>
          ) : controller.page ? (
            <div className="flex min-w-0 flex-col gap-2.5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-eyebrow text-muted tabular-nums">
                  {formatPageRange(controller.page)}
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    disabled={controller.copyState === "copying"}
                    onClick={() => void controller.onCopy()}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    {controller.copyState === "copying"
                      ? "Copying result"
                      : controller.copyState === "copied"
                        ? "Copied result"
                        : "Copy result"}
                  </Button>
                  <Button
                    aria-label="Previous result page"
                    disabled={!controller.canGoPrevious}
                    onClick={controller.onPreviousPage}
                    size="icon-sm"
                    type="button"
                    variant="ghost"
                  >
                    <ChevronLeft aria-hidden="true" />
                  </Button>
                  <Button
                    aria-label="Next result page"
                    disabled={!controller.canGoNext}
                    onClick={controller.onNextPage}
                    size="icon-sm"
                    type="button"
                    variant="ghost"
                  >
                    <ChevronRight aria-hidden="true" />
                  </Button>
                </div>
              </div>
              <div className="max-h-80 overflow-auto rounded-lg">
                <CodeBlock
                  caption="Result page"
                  code={controller.pageText}
                  copyable={false}
                  data-result-ref={resultRef}
                  density="compact"
                  wrapLines
                />
              </div>
              <p aria-live="polite" className="sr-only" role="status">
                {copyStatus}
              </p>
            </div>
          ) : null}
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}

function formatPageRange(page: TaskRunResultPage): string {
  const start = page.total_bytes === 0 ? 0 : page.offset + 1;
  const end = page.offset + page.bytes;
  return `Bytes ${formatNumber(start)}–${formatNumber(end)} of ${formatNumber(page.total_bytes)}`;
}

function formatByteCount(bytes: number): string {
  return `${formatNumber(bytes)} bytes`;
}

function formatNumber(value: number): string {
  return NUMBER_FORMATTER.format(value);
}
