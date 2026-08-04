import { AlertCircle } from "lucide-react";

import { withOccurrenceKeys } from "@/lib/occurrence-keys";

import type { LoopValidationIssue } from "../../types";

interface LoopEditorPublishRejectedStripProps {
  message: string;
  issues: readonly LoopValidationIssue[];
  /** The version the editor still holds — a rejected publish never advances it. */
  version: number | undefined;
}

/**
 * The publish-rejected strip (`loop-editor-states.html` → `state-publish-rejected`): a danger
 * notice between the canvas and the validation dock, carrying the daemon's own issue list so
 * the fix path is visible without expanding the dock, and stating plainly that the compare-and
 * swap version was kept. Nothing was saved — the version pill is unchanged.
 */
export function LoopEditorPublishRejectedStrip({
  message,
  issues,
  version,
}: LoopEditorPublishRejectedStripProps) {
  return (
    <div
      className="flex flex-none flex-col gap-1.5 border-t border-danger/40 bg-danger-tint px-3.5 py-2"
      data-testid="loop-editor-publish-error"
      role="alert"
    >
      <p className="flex items-center gap-2 text-form-label text-danger">
        <AlertCircle aria-hidden="true" className="size-3.5 shrink-0" />
        {message}
        <span className="ml-auto font-mono text-mono-id text-subtle">
          expected_version v{version ?? "?"} kept
        </span>
      </p>
      {issues.length > 0 ? (
        <ul className="flex flex-col gap-0.5 pl-5.5" data-testid="loop-editor-publish-error-issues">
          {withOccurrenceKeys(
            issues,
            issue =>
              `${issue.node_id ?? ""}\u0000${issue.code}\u0000${issue.severity}\u0000${issue.message}`
          ).map(({ item: issue, key }) => (
            <li
              className="text-form-hint leading-snug text-fg"
              data-testid="loop-editor-publish-error-issue"
              key={key}
            >
              <span className="font-mono text-mono-id font-medium text-fg-strong">
                {issue.node_id || "graph"}
              </span>{" "}
              · {issue.message}{" "}
              <span className="font-mono text-mono-id text-subtle">code: {issue.code}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
