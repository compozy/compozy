import {
  Eyebrow,
  MetadataList,
  MetadataListRow,
  MetadataListTerm,
  MetadataListValue,
  Pill,
} from "@compozy/ui";
import { Bot, Clock, FileCode, PackageOpen, Repeat, ShieldCheck } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { MarketplaceCrumbs } from "@/components/marketplace/marketplace-crumbs";
import { MarketplaceInstallCommand } from "@/components/marketplace/marketplace-install-command";
import { specCycleExtension } from "@/lib/marketplace-bundled";
import { bundledSpecCycleDescription } from "@/lib/marketplace-copy";
import { docsSource } from "@/lib/source";
import { createPageMetadata } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: `${specCycleExtension.displayName} — Marketplace`,
  description: bundledSpecCycleDescription(specCycleExtension.description),
  path: "/marketplace/bundled/spec-cycle",
});

function SectionHead({
  id,
  icon: Icon,
  children,
}: {
  id: string;
  icon: typeof Repeat;
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

function InventoryLabel({ children }: { children: string }) {
  return (
    <p className="mt-6">
      <Eyebrow className="text-subtle">{children}</Eyebrow>
    </p>
  );
}

function Chip({ icon: Icon, children }: { icon: typeof FileCode; children: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-md border border-line bg-canvas-soft px-2 py-1 font-mono text-badge text-muted">
      <Icon aria-hidden className="size-3 shrink-0 opacity-70" />
      {children}
    </span>
  );
}

export default function BundledSpecCyclePage() {
  const { loops, skills, agents, tools } = specCycleExtension;
  const walkthroughs = new Map(
    loops.map(loop => [loop.name, docsSource.getPage(["examples", `${loop.name}-loop`])?.url])
  );

  return (
    <main id="main-content" className="mx-auto w-full max-w-3xl px-4 pt-12 pb-20">
      <MarketplaceCrumbs leaf={specCycleExtension.displayName} />

      <header className="mt-6 flex items-start gap-4">
        <span className="mt-1 inline-flex size-11.5 shrink-0 items-center justify-center rounded-xl border border-line-strong bg-elevated text-fg">
          <Repeat aria-hidden className="size-5" />
        </span>
        <div className="min-w-0">
          <h1 className="text-detail-h1 font-semibold tracking-detail-h1 text-fg-strong">
            {specCycleExtension.displayName}
          </h1>
          <p className="mt-2.5 text-site-doc-lead text-muted">{specCycleExtension.description}</p>
        </div>
      </header>

      <div className="mt-6 flex flex-wrap items-center gap-x-4 gap-y-2 text-small-body text-muted">
        <code className="font-mono text-badge text-subtle">v{specCycleExtension.version}</code>
        <Pill size="sm">
          <ShieldCheck aria-hidden className="size-3" />
          Official
        </Pill>
        <Pill size="sm">Bundled</Pill>
        <span className="inline-flex items-center gap-1.5">
          <Clock aria-hidden className="size-3.5 text-subtle" />
          Requires{" "}
          <strong className="font-medium text-fg">
            CompozyOS ≥ {specCycleExtension.minCompozyVersion}
          </strong>
        </span>
      </div>

      <div className="mt-7 rounded-lg border border-line bg-canvas-soft p-4">
        <p className="text-small-body leading-relaxed text-muted">
          <strong className="font-medium text-fg">No install needed.</strong> Spec Cycle is compiled
          into the compozy binary and enrolled the first time the daemon starts, so it is already
          present in your runtime. Installing your own extension under the same name takes
          precedence over the bundled copy.
        </p>
        <MarketplaceInstallCommand command={specCycleExtension.statusCommand} className="mt-3.5" />
      </div>

      <section aria-labelledby="whats-inside" className="mt-12">
        <SectionHead id="whats-inside" icon={PackageOpen}>
          What&apos;s inside
        </SectionHead>

        <InventoryLabel>{`${loops.length} loops · compozy.loop/v1`}</InventoryLabel>
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {loops.map(loop => {
            const walkthroughUrl = walkthroughs.get(loop.name);
            return (
              <article key={loop.name} className="rounded-xl border border-line bg-canvas-soft p-4">
                <p className="flex flex-wrap items-center gap-2 font-mono text-card-title font-medium text-fg">
                  <Repeat aria-hidden className="size-3.5 shrink-0 text-muted" />
                  {loop.name}
                  {loop.category ? (
                    <Pill size="sm" mono>
                      {loop.category}
                    </Pill>
                  ) : null}
                </p>
                <p className="mt-2 text-small-body leading-relaxed text-muted">
                  {loop.description}
                </p>
                {loop.useWhen ? (
                  <p className="mt-2.5 text-small-body leading-relaxed text-subtle">
                    <strong className="font-medium text-muted">Use when</strong> {loop.useWhen}
                  </p>
                ) : null}
                {walkthroughUrl ? (
                  <Link
                    href={walkthroughUrl}
                    className="mt-3.5 inline-flex text-small-body font-medium text-muted transition-colors hover:text-accent"
                  >
                    Read the walkthrough
                  </Link>
                ) : null}
              </article>
            );
          })}
        </div>

        <InventoryLabel>{`${skills.length} skills`}</InventoryLabel>
        <div className="mt-3 flex flex-wrap gap-2">
          {skills.map(skill => (
            <Chip key={skill} icon={FileCode}>
              {skill}
            </Chip>
          ))}
        </div>

        <InventoryLabel>{`${agents.length} agents`}</InventoryLabel>
        <div className="mt-3 flex flex-wrap gap-2">
          {agents.map(agent => (
            <Chip key={agent} icon={Bot}>
              {agent}
            </Chip>
          ))}
        </div>

        <InventoryLabel>{`${tools.length} ${tools.length === 1 ? "tool" : "tools"}`}</InventoryLabel>
        <MetadataList className="mt-3">
          {tools.map(tool => (
            <MetadataListRow key={tool.name}>
              <MetadataListTerm className="font-mono text-fg">{tool.name}</MetadataListTerm>
              <MetadataListValue>
                {tool.description}
                <span className="mt-2 flex flex-wrap gap-1.5">
                  <Pill size="sm" tone={tool.readOnly ? "neutral" : "warning"}>
                    {tool.readOnly ? "read-only" : tool.risk}
                  </Pill>
                  <Pill size="sm">{tool.visibility}-visible</Pill>
                  {tool.concurrencySafe ? <Pill size="sm">concurrency-safe</Pill> : null}
                </span>
              </MetadataListValue>
            </MetadataListRow>
          ))}
        </MetadataList>
      </section>

      <section aria-labelledby="provenance" className="mt-12">
        <SectionHead id="provenance" icon={ShieldCheck}>
          Provenance
        </SectionHead>
        <MetadataList className="mt-4">
          <MetadataListRow>
            <MetadataListTerm>Source</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap items-center gap-2">
              <Pill size="sm">Bundled</Pill>
              <span className="text-subtle">
                Shipped inside the binary and enrolled at first daemon boot — not downloaded, so no
                artifact URL and no digest to verify.
              </span>
            </MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>Tier</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap items-center gap-2">
              <Pill size="sm">
                <ShieldCheck aria-hidden className="size-3" />
                Official
              </Pill>
              <span className="text-subtle">First-party, from the CompozyOS repository.</span>
            </MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>Provides</MetadataListTerm>
            <MetadataListValue className="flex flex-wrap gap-1.5">
              {specCycleExtension.provides.map(capability => (
                <Pill key={capability} size="sm" mono>
                  {capability}
                </Pill>
              ))}
            </MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>Repository</MetadataListTerm>
            <MetadataListValue className="font-mono">
              <a
                href={specCycleExtension.repositoryUrl}
                target="_blank"
                rel="noreferrer noopener"
                className="break-all underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
              >
                {specCycleExtension.repositoryUrl.replace(/^https:\/\//, "")}
              </a>
            </MetadataListValue>
          </MetadataListRow>
          <MetadataListRow>
            <MetadataListTerm>Min runtime</MetadataListTerm>
            <MetadataListValue className="font-mono">
              compozy ≥ {specCycleExtension.minCompozyVersion}
            </MetadataListValue>
          </MetadataListRow>
        </MetadataList>
      </section>

      <p className="mt-12 border-t border-line pt-6 text-small-body leading-relaxed text-subtle">
        How bundled and installed extensions differ is covered in the{" "}
        <Link
          href="/docs/extensions/install"
          className="underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
        >
          extension install docs
        </Link>
        .
      </p>
    </main>
  );
}
