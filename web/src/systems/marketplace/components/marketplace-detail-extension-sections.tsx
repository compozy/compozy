import { CircleAlert } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, Pill } from "@compozy/ui";

import { MarketplaceTrustWarningList } from "./marketplace-trust-warning-list";

/**
 * Three distinct daemon facts, never collapsed: a name is `bound` ("Set") when a stored binding satisfies
 * it, `missing` when the daemon reports it unresolved, and `available` ("From your system") when it resolves
 * from the process environment. A binding whose name the manifest no longer declares is listed as
 * stale — the daemon keeps it but never injects it, so the panel says so instead of implying use.
 */
function ExtensionEnvironmentState({
  required,
  missing,
  bound,
}: {
  required: string[];
  missing: string[];
  bound: string[];
}) {
  const boundValues = new Set(bound);
  const declaredValues = new Set(required);
  const stale = bound.filter(value => !declaredValues.has(value));
  if (!required.length && !stale.length)
    return <p className="text-small-body text-muted">Nothing to set up.</p>;
  const missingValues = new Set(missing);
  return (
    <div className="space-y-2" data-testid="extension-environment-state">
      {required.map(value => (
        <div className="flex items-center justify-between gap-3" key={value}>
          <code className="font-mono text-eyebrow text-fg">{value}</code>
          {boundValues.has(value) ? (
            <span className="text-form-label text-muted">Set</span>
          ) : missingValues.has(value) ? (
            <Pill form="plain" tone="warning">
              <Pill.Dot />
              Missing
            </Pill>
          ) : (
            <span className="text-form-label text-muted">From your system</span>
          )}
        </div>
      ))}
      {stale.map(value => (
        <div className="flex items-center justify-between gap-3" key={value}>
          <code className="font-mono text-eyebrow text-fg">{value}</code>
          <Pill form="plain" tone="warning">
            <Pill.Dot />
            Set · no longer used
          </Pill>
        </div>
      ))}
    </div>
  );
}

function ExtensionDiagnostics({
  diagnostics,
  lastError,
}: {
  diagnostics: Array<{ id: string; title: string; message: string; severity: string }>;
  lastError?: string;
}) {
  if (!diagnostics.length && !lastError) return null;
  return (
    <div className="flex flex-col gap-2">
      {lastError ? (
        <Alert data-testid="extension-last-error" role="note" variant="danger">
          <CircleAlert aria-hidden="true" />
          <AlertTitle>Last error</AlertTitle>
          <AlertDescription>{lastError}</AlertDescription>
        </Alert>
      ) : null}
      <MarketplaceTrustWarningList items={diagnostics} />
    </div>
  );
}

export { ExtensionDiagnostics, ExtensionEnvironmentState };
