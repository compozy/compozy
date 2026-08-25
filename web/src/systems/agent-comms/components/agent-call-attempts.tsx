/**
 * Both tries on an invalid result, with the checker's words unedited.
 *
 * The UI never paraphrases validator output. Two reasons: a paraphrase loses the
 * JSON pointer the operator needs to fix the contract, and the text is already
 * safe to show — secret-shaped values are hash-redacted *before* validation
 * runs, so everything downstream, error strings included, is sanitized at the
 * source.
 *
 * One repair round is the entire budget. "Try 1 of 1" is not a truncation; the
 * second failure is final and the call settles `invalid-result`.
 */
import { X } from "lucide-react";

import { Eyebrow, Panel } from "@compozy/ui";

export interface AgentCallAttemptsProps {
  repairAttempts: number;
  /** The first try's validator output, once the contract projects it. */
  firstIssueText?: string | null;
  /** The second try's validator output, from `failure_detail`. */
  secondIssueText: string | null;
  "data-testid"?: string;
}

/**
 * Validator output arrives as one block that may hold several issues, one per
 * line. Splitting on newlines keeps each pointer on its own row without
 * reformatting any of them.
 */
function issueLines(text: string | null | undefined): string[] {
  if (!text) return [];
  return text
    .split("\n")
    .map(line => line.trim())
    .filter(line => line.length > 0);
}

function Attempt({
  heading,
  outcome,
  issues,
  testId,
}: {
  heading: string;
  outcome: string;
  issues: string[];
  testId: string;
}) {
  return (
    <div data-testid={testId} className="border-t border-line-soft py-2 first:border-t-0">
      <p className="flex items-baseline gap-2">
        <b className="text-small-body text-fg-strong">{heading}</b>
        <span className="font-mono text-form text-muted">{outcome}</span>
      </p>
      {issues.length > 0 ? (
        <ul className="mt-1.5 flex flex-col gap-1">
          {issues.map(issue => (
            <li key={issue} className="flex items-start gap-1.5 text-form text-danger">
              <X className="mt-0.5 size-3 shrink-0" aria-hidden="true" />
              <span className="font-mono break-words">{issue}</span>
            </li>
          ))}
        </ul>
      ) : (
        <p className="mt-1.5 text-form text-muted">The runtime kept no issue text for this try.</p>
      )}
    </div>
  );
}

export function AgentCallAttempts({
  repairAttempts,
  firstIssueText,
  secondIssueText,
  "data-testid": testId,
}: AgentCallAttemptsProps) {
  // No repair round means there was only ever one answer to reject.
  const showFirst = repairAttempts > 0;
  return (
    <Panel data-testid={testId} title={<Eyebrow>Attempts</Eyebrow>}>
      {showFirst ? (
        <Attempt
          testId="agent-call-attempt-1"
          heading="Try 1"
          outcome="rejected — sent back for one retry"
          issues={issueLines(firstIssueText)}
        />
      ) : null}
      <Attempt
        testId="agent-call-attempt-2"
        heading={showFirst ? "Try 2" : "Try 1"}
        outcome={
          showFirst
            ? "rejected again — call ended invalid-result"
            : "rejected — call ended invalid-result"
        }
        issues={issueLines(secondIssueText)}
      />
    </Panel>
  );
}
