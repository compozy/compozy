import { Check, TriangleAlert } from "lucide-react";

import { EntityDialogToolbar, Pill } from "@compozy/ui";

import type { AutomationSentence } from "../../lib/automation-sentence";
import { AutomationSentenceText } from "../automation-row-parts";

interface AutomationEditorSentenceBarProps {
  sentence: AutomationSentence;
  ready: boolean;
}

/**
 * The live sentence under the editor head: `describeAutomation(draft)` with
 * missing parts dashed, and the status pill "Ready" or "Needs a fix".
 */
export function AutomationEditorSentenceBar({ sentence, ready }: AutomationEditorSentenceBarProps) {
  return (
    <EntityDialogToolbar
      className="flex-nowrap border-b border-line-soft bg-sunken"
      data-testid="automation-editor-sentence-bar"
      leading={
        <p
          aria-live="polite"
          className="min-w-0 flex-1 text-small-body leading-snug text-muted"
          data-testid="automation-editor-sentence"
        >
          <AutomationSentenceText sentence={sentence} />
        </p>
      }
      trailing={
        <Pill data-testid="automation-editor-status" tone={ready ? "success" : "warning"}>
          {ready ? <Check aria-hidden="true" /> : <TriangleAlert aria-hidden="true" />}
          {ready ? "Ready" : "Needs a fix"}
        </Pill>
      }
    />
  );
}
