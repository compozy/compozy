import {
  MetadataList,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  Pill,
} from "@compozy/ui";
import { Clock, Lock, Plug, Settings2, ShieldCheck, Tag } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MarketplaceCrumbs } from "@/components/marketplace/marketplace-crumbs";
import { MarketplaceInstallCommand } from "@/components/marketplace/marketplace-install-command";
import { formatFeedDate, kindMeta } from "@/components/marketplace/marketplace-kind-meta";
import {
  findEntry,
  installCommand,
  isMarketplaceKind,
  MARKETPLACE_KINDS,
  entriesForKind,
  type ExtensionEntry,
  type MCPEntry,
  type SkillEntry,
} from "@/lib/marketplace-catalog";
import { createPageMetadata } from "@/lib/site-config";

interface PageProps {
  params: Promise<{ kind: string; entryId: string }>;
}

export function generateStaticParams() {
  return MARKETPLACE_KINDS.flatMap(kind =>
    entriesForKind(kind).map(entry => ({ kind, entryId: entry.entry_id }))
  );
}

export async function generateMetadata(props: PageProps): Promise<Metadata> {
  const { kind, entryId } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const entry = findEntry(kind, entryId);
  if (!entry) notFound();

  return createPageMetadata({
    title: `${entry.name} — Marketplace`,
    description: entry.description,
    path: `/marketplace/${kind}/${entryId}`,
  });
}

function tierLabel(tier: ExtensionEntry["tier"]): string {
  return tier.charAt(0).toUpperCase() + tier.slice(1);
}

const TIER_HINTS: Record<ExtensionEntry["tier"], string> = {
  official: "First-party, shipped from the Compozy repository.",
  community: "Community-published; review the repository before installing.",
  unverified: "Not reviewed by Compozy; verify the artifact yourself.",
};

/** Icon-headed sections, matching the reference detail anatomy. */
function SectionHead({
  id,
  icon: Icon,
  children,
}: {
  id: string;
  icon: typeof ShieldCheck;
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

/** Author and the newest date already read in the header strip; this list carries the rest. */
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
        {published && entry.updated_at ? (
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

/** The tier pill and author read in the header strip; this list explains and evidences them. */
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
              {tierLabel(entry.tier)}
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

function transportHint(transport: MCPEntry["transport"]): string {
  if (transport === "stdio") return "Runs locally as a child process the daemon supervises.";
  return "Remote server the daemon connects to over HTTP.";
}

function MCPDetail({ entry }: { entry: MCPEntry }) {
  const command = [entry.command, ...(entry.args ?? [])].filter(Boolean).join(" ");
  return (
    <>
      <section aria-labelledby="runtime-config" className="mt-10">
        <SectionHead id="runtime-config" icon={Plug}>
          Runtime configuration
        </SectionHead>
        <MetadataList className="mt-4">
          <MetadataListRow>
            <MetadataListTerm>Transport</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap items-center gap-2">
              <Pill size="sm" className="font-mono">
                {entry.transport}
              </Pill>
              <span className="text-subtle">{transportHint(entry.transport)}</span>
            </MetadataListValue>
          </MetadataListRow>
          {command ? (
            <MetadataListRow>
              <MetadataListTerm>Command</MetadataListTerm>
              <MetadataListValue className="break-all font-mono">{command}</MetadataListValue>
            </MetadataListRow>
          ) : null}
          {entry.url ? (
            <MetadataListRow>
              <MetadataListTerm>URL</MetadataListTerm>
              <MetadataListValue className="break-all font-mono">{entry.url}</MetadataListValue>
            </MetadataListRow>
          ) : null}
          {entry.default_scope ? (
            <MetadataListRow>
              <MetadataListTerm>Default scope</MetadataListTerm>
              <MetadataListValue>
                {entry.default_scope === "global" ? "Global" : "Workspace"}
                <span className="ms-2 text-subtle">
                  {entry.default_scope === "global"
                    ? "Available to every workspace after install."
                    : "Installed into the workspace you run the command from."}
                  {" The feed sets the default; the runtime owns the scope at install."}
                </span>
              </MetadataListValue>
            </MetadataListRow>
          ) : null}
        </MetadataList>
      </section>

      {entry.env?.length ? (
        <section aria-labelledby="env-vars" className="mt-10">
          <SectionHead id="env-vars" icon={Settings2}>
            Environment
          </SectionHead>
          <div className="mt-4 overflow-x-auto rounded-lg border border-line">
            <table className="w-full text-start text-small-body">
              <thead>
                <tr className="border-b border-line text-start">
                  <th className="px-4 py-2.5 text-start font-medium text-muted">Variable</th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">
                    Prompt at install
                  </th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">Required</th>
                  <th className="px-4 py-2.5 text-start font-medium text-muted">
                    <span className="sr-only">Sensitivity</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {entry.env.map(field => (
                  <tr key={field.name} className="border-b border-line-soft last:border-b-0">
                    <td className="px-4 py-2.5 font-mono text-fg">{field.name}</td>
                    <td className="px-4 py-2.5 text-muted">{field.prompt ?? "—"}</td>
                    <td className="px-4 py-2.5 text-muted">
                      {field.required ? "Required" : "Optional"}
                    </td>
                    <td className="px-4 py-2.5">
                      {field.secret ? (
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
            Secret values are prompted at install — never stored in the catalog, never rendered on
            this page.
          </p>
        </section>
      ) : null}

      {entry.oauth ? (
        <section aria-labelledby="oauth" className="mt-10">
          <SectionHead id="oauth" icon={Lock}>
            OAuth
          </SectionHead>
          <MetadataList className="mt-4">
            <MetadataListRow>
              <MetadataListTerm>Client ID</MetadataListTerm>
              <MetadataListValue className="break-all font-mono">
                {entry.oauth.client_id}
              </MetadataListValue>
            </MetadataListRow>
            {entry.oauth.issuer_url ? (
              <MetadataListRow>
                <MetadataListTerm>Issuer</MetadataListTerm>
                <MetadataListValue className="break-all font-mono">
                  {entry.oauth.issuer_url}
                </MetadataListValue>
              </MetadataListRow>
            ) : null}
            {entry.oauth.authorization_url ? (
              <MetadataListRow>
                <MetadataListTerm>Authorization URL</MetadataListTerm>
                <MetadataListValue className="break-all font-mono">
                  {entry.oauth.authorization_url}
                </MetadataListValue>
              </MetadataListRow>
            ) : null}
            {entry.oauth.token_url ? (
              <MetadataListRow>
                <MetadataListTerm>Token URL</MetadataListTerm>
                <MetadataListValue className="break-all font-mono">
                  {entry.oauth.token_url}
                </MetadataListValue>
              </MetadataListRow>
            ) : null}
            {entry.oauth.scopes?.length ? (
              <MetadataListRow>
                <MetadataListTerm>Scopes</MetadataListTerm>
                <MetadataListValue className="flex flex-wrap gap-1.5">
                  {entry.oauth.scopes.map(scope => (
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

export default async function MarketplaceEntryPage(props: PageProps) {
  const { kind, entryId } = await props.params;
  if (!isMarketplaceKind(kind)) notFound();
  const entry = findEntry(kind, entryId);
  if (!entry) notFound();

  const meta = kindMeta(kind);
  const Icon = meta.icon;
  const extension = kind === "extensions" ? (entry as ExtensionEntry) : undefined;
  const mcp = kind === "mcp" ? (entry as MCPEntry) : undefined;
  const author = "author" in entry ? entry.author : undefined;

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
          <code className="font-mono text-badge text-subtle">v{entry.version}</code>
        ) : null}
        {extension ? (
          <Pill size="sm">
            <ShieldCheck aria-hidden className="size-3" />
            {tierLabel(extension.tier)}
          </Pill>
        ) : null}
        {mcp ? (
          <Pill size="sm" mono>
            {mcp.transport}
          </Pill>
        ) : null}
        {author ? (
          <span>
            By <strong className="font-medium text-fg">{author}</strong>
          </span>
        ) : null}
        {formatFeedDate(entry.updated_at ?? entry.published_at) ? (
          <span className="inline-flex items-center gap-1.5">
            <Clock aria-hidden className="size-3.5 text-subtle" />
            {entry.updated_at ? "Updated" : "Published"}{" "}
            {formatFeedDate(entry.updated_at ?? entry.published_at)}
          </span>
        ) : null}
      </div>

      {/* The one accent action on the screen: the site has no daemon, so the command is the CTA. */}
      <MarketplaceInstallCommand
        command={installCommand(kind, entry)}
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
