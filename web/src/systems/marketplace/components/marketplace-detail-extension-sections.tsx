import type { ReactNode } from "react";

import { MonoId, Pill, Section } from "@compozy/ui";

function ExtensionDetailBlock({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Section label={label}>
      <div className="rounded-lg bg-canvas-soft px-4 py-3">{children}</div>
    </Section>
  );
}

function ExtensionTokenBlock({ label, values }: { label: string; values: string[] }) {
  return (
    <ExtensionDetailBlock label={`${label} ${values.length}`}>
      {values.length ? (
        <div className="flex flex-wrap gap-2">
          {values.map(value => (
            <MonoId key={value} value={value} />
          ))}
        </div>
      ) : (
        <p className="text-small-body text-muted">None registered.</p>
      )}
    </ExtensionDetailBlock>
  );
}

/**
 * Three distinct daemon facts, never collapsed: a name is `bound` when a stored binding satisfies
 * it, `missing` when the daemon reports it unresolved, and `available` when it already resolves
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
    return <p className="text-small-body text-muted">No environment variables required.</p>;
  const missingValues = new Set(missing);
  return (
    <div className="space-y-2" data-testid="extension-environment-state">
      {required.map(value => (
        <div className="flex items-center justify-between gap-3" key={value}>
          <code className="font-mono text-xs text-fg">{value}</code>
          {boundValues.has(value) ? (
            <Pill tone="success">bound</Pill>
          ) : missingValues.has(value) ? (
            <Pill tone="warning">missing</Pill>
          ) : (
            <Pill tone="success">available</Pill>
          )}
        </div>
      ))}
      {stale.map(value => (
        <div className="flex items-center justify-between gap-3" key={value}>
          <code className="font-mono text-xs text-fg">{value}</code>
          <Pill tone="warning">bound · not declared</Pill>
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
  if (!diagnostics.length && !lastError)
    return <p className="text-small-body text-muted">No diagnostics.</p>;
  return (
    <div className="space-y-3">
      {lastError ? (
        <div
          className="rounded-md border border-line bg-danger-tint px-3 py-2"
          data-testid="extension-last-error"
        >
          <div className="flex items-center gap-2">
            <Pill mono size="xs" tone="danger">
              error
            </Pill>
            <b className="text-sm text-danger">Last runtime error</b>
          </div>
          <p className="mt-1 text-xs text-muted">{lastError}</p>
        </div>
      ) : null}
      {diagnostics.map(item => {
        const tone =
          item.severity === "error" ? "danger" : item.severity === "info" ? "info" : "warning";
        const surface =
          tone === "danger"
            ? "border-line bg-danger-tint"
            : tone === "info"
              ? "border-line bg-info-tint"
              : "border-line bg-warning-tint";
        return (
          <div className={`rounded-md border px-3 py-2 ${surface}`} key={item.id}>
            <div className="flex items-center gap-2">
              <Pill mono size="xs" tone={tone}>
                {item.severity}
              </Pill>
              <b className="text-sm text-fg">{item.title}</b>
            </div>
            <p className="mt-1 text-xs text-muted">{item.message}</p>
          </div>
        );
      })}
    </div>
  );
}

/**
 * The enable action result, not a status projection: the daemon enumerates exactly the automation
 * definitions that became runnable in the committed operation, and nothing mirrors that list into
 * the extension row afterwards. Shown only while it is this session's own outcome.
 */
function ExtensionAutomationStarted({ started }: { started: string[] }) {
  if (!started.length) return null;
  return (
    <Section count={started.length} label="Automation started">
      <div
        className="divide-y divide-line-soft overflow-hidden rounded-lg bg-canvas-soft"
        data-testid="extension-automation-started"
      >
        {[...started].sort().map(name => (
          <div className="px-4 py-3" key={name}>
            <MonoId value={name} />
          </div>
        ))}
      </div>
    </Section>
  );
}

function ExtensionRailBlock({
  label,
  rows,
}: {
  label: string;
  rows: Array<{ term: string; value: ReactNode }>;
}) {
  return (
    <Section label={label}>
      <dl className="divide-y divide-line-soft overflow-hidden rounded-lg bg-canvas-soft">
        {rows.length ? (
          rows.map(row => (
            <div className="grid gap-1 px-4 py-3" key={row.term}>
              <dt className="eyebrow text-muted">{row.term}</dt>
              <dd className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-fg">
                {row.value}
              </dd>
            </div>
          ))
        ) : (
          <div className="px-4 py-3 text-small-body text-muted">None.</div>
        )}
      </dl>
    </Section>
  );
}

export {
  ExtensionAutomationStarted,
  ExtensionDetailBlock,
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
  ExtensionRailBlock,
  ExtensionTokenBlock,
};
