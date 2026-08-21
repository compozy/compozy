import { OctagonX } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Button } from "@compozy/ui";

export interface WindowManagerBindingConflictProps {
  /** What was refused, in the operator's words: a chord glyph or an alias. */
  claim: string;
  /** Registry title of the command that holds it today. */
  ownerTitle: string;
  /** What the transfer costs the current owner. */
  consequence: string;
  overwriteLabel: string;
  testId: string;
  onOverwrite: () => void;
  onCancel: () => void;
}

/**
 * A refused claim, named and reversible.
 *
 * The daemon decided this, so the culprit is named rather than described, and
 * the transfer is an explicit act — taking a binding away from another command
 * is destructive and wears that weight (US-022.AC-2, US-023.AC-2).
 */
export function WindowManagerBindingConflict({
  claim,
  ownerTitle,
  consequence,
  overwriteLabel,
  testId,
  onOverwrite,
  onCancel,
}: WindowManagerBindingConflictProps) {
  return (
    <Alert data-testid={testId} variant="danger">
      <OctagonX aria-hidden="true" />
      <AlertTitle>
        {claim} is already used by {ownerTitle}
      </AlertTitle>
      <AlertDescription>{consequence}</AlertDescription>
      <div className="col-start-2 flex items-center gap-2 pt-0.5">
        <Button size="sm" type="button" variant="destructive" onClick={onOverwrite}>
          {overwriteLabel}
        </Button>
        <Button size="sm" type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </Alert>
  );
}
