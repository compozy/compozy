import { ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";

import {
  Button,
  CodeBlock,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  Skeleton,
} from "@compozy/ui";

import type { TaskRunResultPage } from "../types";
import type { TaskResultPageController } from "./task-result-types";

const NUMBER_FORMATTER = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 });

export function TaskExternalResult({
  controller,
  resultBytes,
  resultRef,
}: {
  controller: TaskResultPageController;
  resultBytes: number;
  resultRef: string;
}) {
  return (
    <Collapsible onOpenChange={controller.onOpenChange} open={controller.open}>
      <div className="rounded-lg bg-card shadow-card">
        <div className="flex flex-wrap items-center justify-between gap-3 px-3 py-2.5">
          <span className="text-small-body text-muted tabular-nums">
            {formatByteCount(resultBytes)}
          </span>
          <CollapsibleTrigger
            className="group/result-trigger"
            render={
              <Button size="sm" type="button" variant="secondary">
                {controller.open ? "Hide result" : "View result"}
                <ChevronDown
                  aria-hidden="true"
                  data-icon="inline-end"
                  className="transition-transform duration-fast group-data-panel-open/result-trigger:rotate-180 motion-reduce:transition-none"
                />
              </Button>
            }
          />
        </div>
        <CollapsibleContent className="border-t border-line-soft px-3 py-3">
          <TaskExternalResultBody controller={controller} resultRef={resultRef} />
        </CollapsibleContent>
      </div>
    </Collapsible>
  );
}

function TaskExternalResultBody({
  controller,
  resultRef,
}: {
  controller: TaskResultPageController;
  resultRef: string;
}) {
  if (controller.isLoading) {
    return (
      <div aria-label="Loading result" className="flex flex-col gap-2" role="status">
        <Skeleton className="h-7 w-48" />
        <Skeleton className="h-40 rounded-lg" />
      </div>
    );
  }
  if (controller.errorMessage) {
    return (
      <div className="flex flex-wrap items-center justify-between gap-3" role="alert">
        <p className="text-small-body text-danger">{controller.errorMessage}</p>
        <Button onClick={controller.onRetry} size="sm" type="button" variant="secondary">
          Retry
        </Button>
      </div>
    );
  }
  if (!controller.page) return null;
  return (
    <TaskExternalResultPage controller={controller} page={controller.page} resultRef={resultRef} />
  );
}

function TaskExternalResultPage({
  controller,
  page,
  resultRef,
}: {
  controller: TaskResultPageController;
  page: TaskRunResultPage;
  resultRef: string;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-2.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Eyebrow className="text-muted tabular-nums">{formatPageRange(page)}</Eyebrow>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            disabled={controller.copyState === "copying"}
            onClick={() => void controller.onCopy()}
            size="sm"
            type="button"
            variant="secondary"
          >
            {copyButtonLabel(controller.copyState)}
          </Button>
          <Button
            aria-label="Previous result page"
            disabled={!controller.canGoPrevious}
            onClick={controller.onPreviousPage}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <ChevronLeft aria-hidden="true" />
          </Button>
          <Button
            aria-label="Next result page"
            disabled={!controller.canGoNext}
            onClick={controller.onNextPage}
            size="icon-sm"
            type="button"
            variant="quiet"
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
        {copyStatusText(controller.copyState)}
      </p>
    </div>
  );
}

function copyButtonLabel(copyState: TaskResultPageController["copyState"]): string {
  if (copyState === "copying") return "Copying result";
  if (copyState === "copied") return "Copied result";
  return "Copy result";
}

function copyStatusText(copyState: TaskResultPageController["copyState"]): string {
  if (copyState === "copied") return "Result copied.";
  if (copyState === "error") return "Couldn't copy result. Try again.";
  if (copyState === "copying") return "Copying result.";
  return "";
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
