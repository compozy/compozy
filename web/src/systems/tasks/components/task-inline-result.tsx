import { useState } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  JsonViewer,
  Markdown,
} from "@compozy/ui";
import { ChevronDown } from "lucide-react";

import type { TaskRun } from "../types";

export function TaskInlineResult({
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
            <TaskInlineResultValue jsonTestId={jsonTestId} result={result} />
          </div>
        ) : null}
        <CollapsibleContent>
          <div className="max-h-96 overflow-auto px-4 py-3.5">
            <TaskInlineResultValue jsonTestId={jsonTestId} result={result} />
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

function TaskInlineResultValue({
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
