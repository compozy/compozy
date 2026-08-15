import { ChevronDown } from "lucide-react";

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

interface LoopEditorFoldProps {
  label: string;
  subLabel?: string;
  defaultOpen?: boolean;
  children: React.ReactNode;
}

/**
 * Shared inspector disclosure: title + optional sub-label + trailing chevron
 * that rotates when closed. Used by node-lane folds and the contract terminals fold.
 */
export function LoopEditorFold({ label, subLabel, defaultOpen, children }: LoopEditorFoldProps) {
  return (
    <Collapsible
      className="rounded-md border border-line-soft bg-canvas-soft"
      defaultOpen={defaultOpen}
    >
      <CollapsibleTrigger className="group flex w-full cursor-pointer items-center gap-2 px-3 py-2.5 text-left text-form-label font-medium text-fg-strong">
        {label}
        {subLabel ? <span className="text-badge font-normal text-faint">{subLabel}</span> : null}
        <ChevronDown
          aria-hidden="true"
          className="ml-auto size-3.5 shrink-0 text-faint transition-transform group-data-[state=closed]:-rotate-90"
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className="flex flex-col gap-3 px-3 pb-3">{children}</div>
      </CollapsibleContent>
    </Collapsible>
  );
}
