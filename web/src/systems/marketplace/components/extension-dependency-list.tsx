import { Eyebrow, MonoId, Pill } from "@compozy/ui";

import type { ExtensionDependency } from "@/systems/extensions";

import { extensionDependencySourceLabel } from "./extension-dependency-model";
import { formatMarketplaceVersion } from "./marketplace-ui";

/**
 * Add's **Also installs** section: every dependency the install must add or turn on, in install
 * order, with its own trust word. A version that is too old blocks Add with update guidance.
 */
export function ExtensionDependencyList({
  dependencies,
}: {
  dependencies: readonly ExtensionDependency[];
}) {
  if (dependencies.length === 0) return null;
  return (
    <section aria-label="Also installs" data-testid="extension-install-dependencies">
      <Eyebrow className="text-subtle">Also installs</Eyebrow>
      <ul className="mt-2 divide-y divide-line-soft border-y border-line-soft">
        {dependencies.map(dependency => (
          <ExtensionDependencyRow dependency={dependency} key={dependency.name} />
        ))}
      </ul>
    </section>
  );
}

function ExtensionDependencyRow({ dependency }: { dependency: ExtensionDependency }) {
  const version = formatMarketplaceVersion(dependency.version);
  return (
    <li
      className="flex flex-col gap-1 py-2.5 text-small-body"
      data-testid={`extension-install-dependency-${dependency.name}`}
    >
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-medium text-fg">
          {dependency.name}
          {version ? <span className="ml-1.5 text-muted-foreground">{version}</span> : null}
        </span>
        {dependency.state === "disabled" ? (
          <Pill size="xs" tone="neutral">
            Turns on
          </Pill>
        ) : null}
        {dependency.state === "version_unmet" ? null : (
          <Pill size="xs" tone={dependency.consent_required ? "warning" : "neutral"}>
            {dependency.consent_required ? "Not verified" : "Checked by CompozyOS"}
          </Pill>
        )}
      </div>
      <span className="text-form-hint text-muted-foreground">
        {extensionDependencySourceLabel(dependency.source)} · {dependency.ref}
      </span>
      {dependency.gateway_requirement_digest ? (
        <span className="flex items-center gap-2 text-form-hint text-muted-foreground">
          Remote access
          <MonoId size="sm" value={dependency.gateway_requirement_digest} />
        </span>
      ) : null}
      {dependency.state === "version_unmet" ? (
        <span className="text-form-hint text-warning" role="alert">
          Update {dependency.name} to {dependency.min_version ?? "the required version"} or later
          first
        </span>
      ) : null}
    </li>
  );
}
