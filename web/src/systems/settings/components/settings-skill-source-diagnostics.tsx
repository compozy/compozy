import { ChevronDown } from "lucide-react";
import type { ReactNode } from "react";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger } from "@compozy/ui";

import type { SkillSourceDiagnosticsView } from "../lib/skill-sources-view";

interface SettingsSkillSourceDiagnosticsProps {
  diagnostics: SkillSourceDiagnosticsView;
  testId: string;
}

/**
 * Why a root's scanned count and usable count differ, one daemon-reported field
 * per line. Closed by default — the root's own state word already tells the
 * short version; this is the long one for when it matters.
 */
export function SettingsSkillSourceDiagnostics({
  diagnostics,
  testId,
}: SettingsSkillSourceDiagnosticsProps) {
  return (
    <Collapsible className="flex min-w-0 flex-col pl-5" data-testid={testId}>
      <CollapsibleTrigger
        className={cn(
          "group/skill-root-diag flex w-full items-center gap-1.5 py-1 text-left",
          "rounded-sm text-form-hint text-subtle",
          "focus-visible:shadow-focus-ring focus-visible:outline-none"
        )}
        type="button"
      >
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 -rotate-90 text-faint",
            "transition-transform duration-base group-data-panel-open/skill-root-diag:rotate-0"
          )}
        />
        {diagnostics.summary}
      </CollapsibleTrigger>
      <CollapsibleContent>
        <dl className="flex flex-col gap-1 pt-0.5 pb-1.5 pl-4.5">
          {diagnostics.skipped.length > 0 ? (
            <DiagnosticRow label="skipped" testId={`${testId}-skipped`}>
              {diagnostics.skipped.map(skip => (
                <p key={skip.path} title={skip.path}>
                  <code className="font-mono text-badge text-muted">{skip.name}</code>{" "}
                  {skip.sentence}
                </p>
              ))}
            </DiagnosticRow>
          ) : null}
          {diagnostics.collisions.length > 0 ? (
            <DiagnosticRow label="name clash" testId={`${testId}-collisions`}>
              {diagnostics.collisions.map(collision => (
                <p key={collision.qualifiedForm}>
                  <code className="font-mono text-badge text-muted">{collision.name}</code> also
                  exists in {collision.winner} — that copy wins. This one stays available as{" "}
                  <code className="font-mono text-badge text-muted">{collision.qualifiedForm}</code>
                </p>
              ))}
            </DiagnosticRow>
          ) : null}
          {diagnostics.verification !== null ? (
            <DiagnosticRow label="checks" testId={`${testId}-verification`}>
              <p>{diagnostics.verification}</p>
            </DiagnosticRow>
          ) : null}
        </dl>
      </CollapsibleContent>
    </Collapsible>
  );
}

function DiagnosticRow({
  label,
  children,
  testId,
}: {
  label: string;
  children: ReactNode;
  testId: string;
}) {
  return (
    <div className="flex min-w-0 items-baseline gap-2" data-testid={testId}>
      <dt className="eyebrow w-20 shrink-0 text-faint">{label}</dt>
      <dd className="flex min-w-0 flex-col gap-0.5 text-form-hint text-muted">{children}</dd>
    </div>
  );
}
