import {
  MetadataList,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  Pill,
} from "@compozy/ui";
import { Clock, Lock, Plug, Settings2, ShieldCheck, Tag } from "lucide-react";
import Link from "next/link";
import {
  installCommand,
  marketplaceSearchCommand,
  type ExtensionEntry,
  type MarketplaceEntry,
  type MarketplaceKind,
  type MCPEntry,
  type SkillEntry,
} from "@/lib/marketplace-catalog";
import { MarketplaceCrumbs } from "./marketplace-crumbs";
import { MarketplaceInstallCommand } from "./marketplace-install-command";
import {
  extensionTierLabel,
  formatFeedDate,
  kindMeta,
  type LucideIcon,
} from "./marketplace-kind-meta";

const TIER_HINTS: Record<ExtensionEntry["tier"], string> = {
  official: "First-party, shipped from the Compozy repository.",
  community: "Community-published; review the repository before installing.",
  unverified: "Not reviewed by Compozy; verify the artifact yourself.",
};

function SectionHead({
  id,
  icon: Icon,
  children,
}: {
  id: string;
  icon: LucideIcon;
  children: string;
}) {
  return (
    <h2
      id={id}
      className="flex items-center gap-2 text-lg font-semibold tracking-[-0.01em] text-fg"
    >
      <Icon aria-hidden className="size-4 text-muted" />
      {children}
    </h2>
  );
}

function SkillDetail({ entry }: { entry: SkillEntry }) {
  const published = formatFeedDate(entry.published_at);
  return (
    <section aria-labelledby="skill-details" className="mt-10">
      <SectionHead id="skill-details" icon={Tag}>
        Details
      </SectionHead>
      <MetadataList className="mt-4">
        <MetadataListRow>
          <MetadataListTerm>Install slug</MetadataListTerm>
          <MetadataListValue className="font-mono">{entry.install_slug}</MetadataListValue>
        </MetadataListRow>
        {published ? (
          <MetadataListRow>
            <MetadataListTerm>Published</MetadataListTerm>
            <MetadataListValue>{published}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        {entry.tags?.length ? (
          <MetadataListRow>
            <MetadataListTerm>Tags</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap gap-1.5">
              {entry.tags.map(tag => (
                <Pill key={tag} size="sm" className="font-mono">
                  {tag}
                </Pill>
              ))}
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
      </MetadataList>
    </section>
  );
}

function ExtensionDetail({ entry }: { entry: ExtensionEntry }) {
  return (
    <section aria-labelledby="provenance" className="mt-10">
      <SectionHead id="provenance" icon={ShieldCheck}>
        Provenance
      </SectionHead>
      <MetadataList className="mt-4">
        <MetadataListRow>
          <MetadataListTerm>Tier</MetadataListTerm>
          <MetadataListValue className="flex flex-wrap items-center gap-2">
            <Pill size="sm">
              <ShieldCheck aria-hidden className="size-3" />
              {extensionTierLabel(entry.tier)}
            </Pill>
            <span className="text-subtle">{TIER_HINTS[entry.tier]}</span>
          </MetadataListValue>
        </MetadataListRow>
        {entry.repository ? (
          <MetadataListRow>
            <MetadataListTerm>Repository</MetadataListTerm>
            <MetadataListValue className="font-mono">
              <a
                href={entry.repository}
                target="_blank"
                rel="noreferrer noopener"
                className="break-all underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
              >
                {entry.repository.replace(/^https:\/\//, "")}
              </a>
            </MetadataListValue>
          </MetadataListRow>
        ) : null}
        <MetadataListRow>
          <MetadataListTerm>Artifact</MetadataListTerm>
          <MetadataListValue className="break-all font-mono">
            {entry.artifact_url}
          </MetadataListValue>
        </MetadataListRow>
        <MetadataListRow>
          <MetadataListTerm>Digest</MetadataListTerm>
          <MetadataListValue className="break-all font-mono">
            sha256:{entry.digest_sha256}
          </MetadataListValue>
        </MetadataListRow>
      </MetadataList>
      <p className="mt-4 text-small-body leading-relaxed text-subtle">
        The runtime verifies this digest against the downloaded artifact before installing — trust
        on this page maps to these fields and nothing else.
      </p>
    </section>
  );
}

function launchHint(type: MCPEntry["launch"]["type"]): string {
  if (type === "remote") return "Hosted server the daemon connects to over Streamable HTTP.";
  return "Pinned local distribution the daemon launches as a supervised child process.";
}

function MCPDetail({ entry }: { entry: MCPEntry }) {
  const launch = entry.launch;
  const launchValue =
    launch.type === "remote"
      ? launch.url
      : launch.type === "docker"
        ? `${launch.image}@${launch.digest}${launch.args?.length ? ` ${launch.args.join(" ")}` : ""}`
        : `${launch.package}@${launch.version}${launch.args?.length ? ` ${launch.args.join(" ")}` : ""}`;
  return (
    <>
      <section aria-labelledby="runtime-config" className="mt-10">
        <SectionHead id="runtime-config" icon={Plug}>
          Runtime configuration
        </SectionHead>
        <MetadataList className="mt-4">
          <MetadataListRow>
            <MetadataListTerm>Launch</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap items-center gap-2">
              <Pill size="sm" className="font-mono">
                {launch.type}
              </Pill>
              <span className="text-subtle">{launchHint(launch.type)}</span>
            </MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>{launch.type === "remote" ? "URL" : "Distribution"}</MetadataListTerm>
            <MetadataListValue className="break-all font-mono">{launchValue}</MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>Default scope</MetadataListTerm>
            <MetadataListValue>
              {entry.default_scope === "global" ? "Global" : "Workspace"}{" "}
              <span className="ms-2 text-subtle">
                {entry.default_scope === "global"
                  ? "Available to every workspace after install."
                  : "Installed into the workspace you run the command from."}
                {" The feed sets the default; the runtime owns the scope at install."}
              </span>
            </MetadataListValue>
          </MetadataListRow>
        </MetadataList>
      </section>

      {entry.inputs?.length ? (
        <section aria-labelledby="inputs" className="mt-10">
          <SectionHead id="inputs" icon={Settings2}>
            Inputs
          </SectionHead>
          <div className="mt-4 overflow-x-auto rounded-lg border border-line">
            <table className="w-full text-start text-small-body">
              <thead>
                <tr className="border-b border-line text-start">
                  <th className="px-4 py-2.5 text-start font-medium text-muted">Input</th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">Value</th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">Required</th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">
                    <span className="sr-only">Sensitivity</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {entry.inputs.map(input => (
                  <tr key={input.id} className="border-b border-line-soft last:border-b-0">
                    <td className="px-4 py-2.5 font-mono text-fg">{input.id}</td>
                    <td className="px-4 py-2.5 text-muted">{input.prompt}</td>
                    <td className="px-4 py-2.5 text-muted">
                      {input.required ? "Required" : "Optional"}
                    </td>
                    <td className="px-4 py-2.5">
                      {input.type === "secret" ? (
                        <Pill size="sm">
                          <Lock aria-hidden className="size-3" />
                          Secret
                        </Pill>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="mt-3 text-small-body leading-relaxed text-subtle">
            When installing with <code>{installCommand("mcp", entry)}</code>, use{" "}
            <code>--secret id</code> or <code>--vault-ref id=vault:mcp/...</code> for secrets and{" "}
            <code>--set id=value</code>
            {". Secret values are never stored in the catalog or rendered on this page."}
          </p>
        </section>
      ) : null}

      {entry.auth ? (
        <section aria-labelledby="oauth" className="mt-10">
          <SectionHead id="oauth" icon={Lock}>
            OAuth
          </SectionHead>
          <MetadataList className="mt-4">
            <MetadataListRow>
              <MetadataListTerm>Registration</MetadataListTerm>
              <MetadataListValue className="font-mono">{entry.auth.registration}</MetadataListValue>
            </MetadataListRow>
            {entry.auth.scopes?.length ? (
              <MetadataListRow>
                <MetadataListTerm>Scopes</MetadataListTerm>
                <MetadataListValue className="flex flex-wrap gap-1.5">
                  {entry.auth.scopes.map(scope => (
                    <Pill key={scope} size="sm" className="font-mono">
                      {scope}
                    </Pill>
                  ))}
                </MetadataListValue>
              </MetadataListRow>
            ) : null}
          </MetadataList>
        </section>
      ) : null}
    </>
  );
}

export function MarketplaceEntryDetail({
  kind,
  entry,
}: {
  kind: MarketplaceKind;
  entry: MarketplaceEntry;
}) {
  const meta = kindMeta(kind);
  const Icon = meta.icon;
  const extension = kind === "extensions" ? (entry as ExtensionEntry) : undefined;
  const mcp = kind === "mcp" ? (entry as MCPEntry) : undefined;
  const author = "author" in entry ? entry.author : undefined;
  const date = formatFeedDate(entry.updated_at ?? entry.published_at);

  return (
    <main id="main-content" className="mx-auto w-full max-w-3xl px-4 pt-12 pb-20">
      <MarketplaceCrumbs
        trail={[{ name: meta.title, href: `/marketplace/${kind}` }]}
        leaf={entry.name}
      />

      <header className="mt-6 flex items-start gap-4">
        <span className="mt-1 inline-flex size-11.5 shrink-0 items-center justify-center rounded-xl border border-line-strong bg-elevated text-fg">
          <Icon aria-hidden className="size-5" />
        </span>
        <div className="min-w-0">
          <h1 className="text-detail-h1 font-semibold tracking-detail-h1 text-fg-strong">
            {entry.name}
          </h1>
          <p className="mt-2.5 text-site-doc-lead text-muted">{entry.description}</p>
        </div>
      </header>

      <div className="mt-6 flex flex-wrap items-center gap-x-4 gap-y-2 text-small-body text-muted">
        {entry.version ? (
          <code className="font-mono text-badge text-subtle">
            {entry.version.startsWith("v") ? entry.version : `v${entry.version}`}
          </code>
        ) : null}
        {extension ? (
          <Pill size="sm">
            <ShieldCheck aria-hidden className="size-3" />
            {extensionTierLabel(extension.tier)}
          </Pill>
        ) : null}
        {mcp ? (
          <Pill size="sm" mono>
            {mcp.launch.type}
          </Pill>
        ) : null}
        {author ? (
          <span>
            By <strong className="font-medium text-fg">{author}</strong>
          </span>
        ) : null}
        {date ? (
          <span className="inline-flex items-center gap-1.5">
            <Clock aria-hidden className="size-3.5 text-subtle" />
            {entry.updated_at ? "Updated" : "Published"} {date}
          </span>
        ) : null}
      </div>

      <MarketplaceInstallCommand
        command={marketplaceSearchCommand(kind, entry)}
        className="mt-7 border-accent/35 bg-canvas-soft"
      />

      {kind === "skills" ? <SkillDetail entry={entry as SkillEntry} /> : null}
      {kind === "extensions" ? <ExtensionDetail entry={entry as ExtensionEntry} /> : null}
      {kind === "mcp" ? <MCPDetail entry={entry as MCPEntry} /> : null}

      <p className="mt-12 border-t border-line pt-6 text-small-body leading-relaxed text-subtle">
        New to the marketplace? The{" "}
        <Link
          href="/docs/marketplace"
          className="underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
        >
          concept docs
        </Link>{" "}
        explain how installs, trust tiers, and registries fit the runtime.
      </p>
    </main>
  );
}
