import {
  MetadataList,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  Pill,
} from "@compozy/ui";
import { ChevronRight, Lock, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
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

function SkillDetail({ entry }: { entry: SkillEntry }) {
  const published = formatFeedDate(entry.published_at);
  const updated = formatFeedDate(entry.updated_at);
  return (
    <section aria-labelledby="skill-details" className="mt-10">
      <h2 id="skill-details" className="text-lg font-semibold tracking-[-0.01em] text-fg">
        Details
      </h2>
      <MetadataList className="mt-4">
        <MetadataListRow>
          <MetadataListTerm>Install slug</MetadataListTerm>
          <MetadataListValue className="font-mono">{entry.install_slug}</MetadataListValue>
        </MetadataListRow>
        {entry.author ? (
          <MetadataListRow>
            <MetadataListTerm>Author</MetadataListTerm>
            <MetadataListValue>{entry.author}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        {published ? (
          <MetadataListRow>
            <MetadataListTerm>Published</MetadataListTerm>
            <MetadataListValue>{published}</MetadataListValue>
          </MetadataListRow>
        ) : null}
        {updated ? (
          <MetadataListRow>
            <MetadataListTerm>Updated</MetadataListTerm>
            <MetadataListValue>{updated}</MetadataListValue>
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
  const published = formatFeedDate(entry.published_at);
  return (
    <section aria-labelledby="provenance" className="mt-10">
      <h2
        id="provenance"
        className="flex items-center gap-2 text-lg font-semibold tracking-[-0.01em] text-fg"
      >
        <ShieldCheck aria-hidden className="size-4 text-muted" />
        Provenance
      </h2>
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
        {entry.author ? (
          <MetadataListRow>
            <MetadataListTerm>Author</MetadataListTerm>
            <MetadataListValue>{entry.author}</MetadataListValue>
          </MetadataListRow>
        ) : null}
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
        {published ? (
          <MetadataListRow>
            <MetadataListTerm>Published</MetadataListTerm>
            <MetadataListValue>{published}</MetadataListValue>
          </MetadataListRow>
        ) : null}
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
        <h2 id="runtime-config" className="text-lg font-semibold tracking-[-0.01em] text-fg">
          Runtime configuration
        </h2>
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
          <h2 id="env-vars" className="text-lg font-semibold tracking-[-0.01em] text-fg">
            Environment
          </h2>
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
          <h2 id="oauth" className="text-lg font-semibold tracking-[-0.01em] text-fg">
            OAuth
          </h2>
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

  return (
    <main id="main-content" className="mx-auto w-full max-w-3xl px-4 pt-12 pb-20">
      <nav aria-label="Breadcrumb" className="flex items-center gap-1.5 text-small-body text-muted">
        <Link href="/marketplace" className="transition-colors hover:text-accent">
          Marketplace
        </Link>
        <ChevronRight aria-hidden className="size-3 text-subtle" />
        <Link href={`/marketplace/${kind}`} className="transition-colors hover:text-accent">
          {meta.title}
        </Link>
        <ChevronRight aria-hidden className="size-3 text-subtle" />
        <span aria-current="page" className="text-fg">
          {entry.name}
        </span>
      </nav>

      <header className="mt-8 flex items-start gap-4">
        <span className="mt-1 inline-flex size-10 shrink-0 items-center justify-center rounded-lg bg-canvas-soft text-muted">
          <Icon aria-hidden className="size-5" />
        </span>
        <div className="min-w-0">
          <h1 className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-3xl font-semibold tracking-[-0.02em] text-fg">
            {entry.name}
            {entry.version ? (
              <span className="font-mono text-base font-normal text-subtle">v{entry.version}</span>
            ) : null}
          </h1>
          <p className="mt-3 text-base leading-relaxed text-muted">{entry.description}</p>
        </div>
      </header>

      <MarketplaceInstallCommand command={installCommand(kind, entry)} className="mt-8" />

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
