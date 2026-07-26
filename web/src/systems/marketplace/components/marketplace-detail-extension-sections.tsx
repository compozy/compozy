import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Button, MonoId, Pill, Section, Spinner } from "@agh/ui";

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

function ExtensionEnvironmentState({
  required,
  missing,
}: {
  required: string[];
  missing: string[];
}) {
  if (!required.length)
    return <p className="text-small-body text-muted">No environment variables required.</p>;
  const missingValues = new Set(missing);
  return (
    <div className="space-y-2">
      {required.map(value => (
        <div className="flex items-center justify-between gap-3" key={value}>
          <code className="font-mono text-xs text-fg">{value}</code>
          {missingValues.has(value) ? (
            <Pill tone="warning">missing</Pill>
          ) : (
            <Pill tone="success">available</Pill>
          )}
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

function ExtensionBundlesProvided({
  active,
  error,
  extensionName,
  isLoading,
  onRetry,
  provided,
}: {
  active: Array<{ id: string; bundle_name: string; extension_name: string }> | undefined;
  error: Error | null;
  extensionName: string;
  isLoading: boolean;
  onRetry: () => void;
  provided: Array<{ name: string; description?: string; profiles?: string[] }>;
}) {
  const activations = (active ?? []).filter(item => item.extension_name === extensionName);
  return (
    <Section label="Bundles provided">
      <div className="divide-y divide-line-soft overflow-hidden rounded-lg bg-canvas-soft">
        {isLoading ? (
          <div
            aria-label="Checking active bundles"
            className="flex items-center gap-2 px-4 py-3 text-small-body text-muted"
            role="status"
          >
            <Spinner className="size-3.5" />
            Checking active bundles
          </div>
        ) : error ? (
          <div className="space-y-2 px-4 py-3">
            <p className="text-small-body font-medium text-danger">
              Bundle activity could not be loaded
            </p>
            <p className="text-xs text-muted">{error.message}</p>
            <Button onClick={onRetry} size="sm" type="button" variant="outline">
              Retry bundle activity
            </Button>
          </div>
        ) : provided.length ? (
          provided.map(bundle => {
            const activation = activations.find(item => item.bundle_name === bundle.name);
            return (
              <div className="flex items-center justify-between gap-3 px-4 py-3" key={bundle.name}>
                <div className="min-w-0">
                  <MonoId value={bundle.name} />
                  <p className="mt-1 truncate text-xs text-muted">
                    {bundle.description ?? (bundle.profiles ?? []).join(", ")}
                  </p>
                </div>
                {activation ? (
                  <Button
                    render={
                      <Link
                        params={{ id: activation.id }}
                        to="/marketplace/bundles/activations/$id"
                      />
                    }
                    nativeButton={false}
                    size="sm"
                    variant="ghost"
                  >
                    Open active bundle →
                  </Button>
                ) : (
                  <Pill size="xs">inactive</Pill>
                )}
              </div>
            );
          })
        ) : (
          <div className="px-4 py-3 text-small-body text-muted">None.</div>
        )}
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
  ExtensionBundlesProvided,
  ExtensionDetailBlock,
  ExtensionDiagnostics,
  ExtensionEnvironmentState,
  ExtensionRailBlock,
  ExtensionTokenBlock,
};
