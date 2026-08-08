import { BookOpen, Download, FileText, History, Tags } from "lucide-react";

import { DescriptionCard, Pill, PropertyRow } from "@compozy/ui";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailSkillInstalled } from "./marketplace-detail-skill-installed";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCard,
  MarketplaceDetailRailNote,
  MarketplaceDetailSection,
} from "./marketplace-detail-shell";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import { formatMarketplaceCount, formatMarketplaceVersion } from "./marketplace-ui";

interface MarketplaceDetailSkillViewProps {
  data: MarketplaceEntryResponse;
}

/**
 * Skill detail: the readme is the hero; the rail keeps short property cards.
 * Installed entries add live management on top of the same anatomy.
 */
function MarketplaceDetailSkillView({ data }: MarketplaceDetailSkillViewProps) {
  if (data.entry.installed) {
    return <MarketplaceDetailSkillInstalled data={data} />;
  }
  return (
    <MarketplaceDetailColumns
      main={
        <>
          <MarketplaceSkillReadmeSection data={data} />
          <MarketplaceDetailWarnings warnings={data.entry.trust?.warnings} />
        </>
      }
      rail={
        <>
          <MarketplaceSkillDetailsCard data={data} />
          <MarketplaceSkillVersionsCard data={data} />
          <MarketplaceSkillTagsCard tags={data.skill?.tags} />
          <MarketplaceSkillOnInstallCard />
        </>
      }
    />
  );
}

function MarketplaceSkillReadmeSection({ data }: { data: MarketplaceEntryResponse }) {
  const readme = data.skill?.readme?.trim();
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-skill-readme"
      icon={BookOpen}
      summary={readme ? "from the skill package" : "catalog description"}
      title="Readme"
    >
      <DescriptionCard bare className="px-5 py-4.5">
        {readme || data.entry.description}
      </DescriptionCard>
    </MarketplaceDetailSection>
  );
}

function MarketplaceSkillDetailsCard({ data }: { data: MarketplaceEntryResponse }) {
  const entry = data.entry;
  const version = formatMarketplaceVersion(entry.version);
  const license = data.skill?.license?.trim();
  const slug = data.skill?.install_slug?.trim() || entry.install_slug?.trim();
  const summary = [version, license].filter(Boolean).join(" · ") || entry.source;
  return (
    <MarketplaceDetailRailCard icon={FileText} summary={summary} title="Details">
      <div className="px-3.5">
        {version ? (
          <PropertyRow label="Version" mono>
            {version}
          </PropertyRow>
        ) : null}
        {slug ? (
          <PropertyRow label="Slug" mono>
            {slug}
          </PropertyRow>
        ) : null}
        {entry.author ? <PropertyRow label="Author">{entry.author}</PropertyRow> : null}
        {license ? <PropertyRow label="License">{license}</PropertyRow> : null}
        {entry.downloads !== undefined && entry.downloads !== null ? (
          <PropertyRow label="Downloads" mono>
            {formatMarketplaceCount(entry.downloads)}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Source" mono>
          {entry.source}
        </PropertyRow>
        <MarketplaceRepositoryRow repository={data.skill?.repository} />
      </div>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceRepositoryRow({ repository }: { repository: string | undefined }) {
  const url = repository?.trim();
  if (!url) return null;
  return (
    <PropertyRow label="Repository" valueTitle={url}>
      <a
        className="min-w-0 truncate font-mono text-eyebrow text-muted transition-colors duration-base hover:text-fg-strong"
        href={url}
        rel="noreferrer"
        target="_blank"
      >
        {formatRepositorySlug(url)} ↗
      </a>
    </PropertyRow>
  );
}

function formatRepositorySlug(url: string): string {
  try {
    const parsed = new URL(url);
    const path = parsed.pathname.replace(/^\/+|\/+$/g, "");
    return path || parsed.host;
  } catch {
    return url;
  }
}

function MarketplaceSkillVersionsCard({ data }: { data: MarketplaceEntryResponse }) {
  const versions = data.skill?.versions ?? [];
  if (versions.length === 0) return null;
  const current = formatMarketplaceVersion(data.entry.version);
  return (
    <MarketplaceDetailRailCard
      data-testid="marketplace-skill-versions"
      icon={History}
      summary={`${versions.length} ${versions.length === 1 ? "release" : "releases"}`}
      title="Versions"
    >
      <ul className="px-3.5">
        {versions.map(version => {
          const formatted = formatMarketplaceVersion(version);
          return (
            <li className="flex min-h-property-row items-center gap-2" key={version}>
              <span className="font-mono text-transcript-meta text-fg tabular-nums">
                {formatted}
              </span>
              {formatted !== null && formatted === current ? (
                <Pill mono size="xs" tone="success">
                  current
                </Pill>
              ) : null}
            </li>
          );
        })}
      </ul>
      <MarketplaceDetailRailNote>
        Install pins the selected version and a directory hash. Updates are explicit — never silent.
      </MarketplaceDetailRailNote>
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceSkillTagsCard({ tags }: { tags: readonly string[] | undefined }) {
  if (!tags || tags.length === 0) return null;
  return (
    <MarketplaceDetailRailCard icon={Tags} summary={String(tags.length)} title="Tags">
      <div className="flex flex-wrap gap-1.5 px-3.5 py-1.5">
        {tags.map(tag => (
          <Pill key={tag}>{tag}</Pill>
        ))}
      </div>
    </MarketplaceDetailRailCard>
  );
}

/** What installing does — the marketplace skill install contract, stated plainly. */
function MarketplaceSkillOnInstallCard() {
  return (
    <MarketplaceDetailRailCard
      defaultOpen={false}
      icon={Download}
      summary="hash-pinned · user scope"
      title="On install"
    >
      <div className="px-3.5">
        <PropertyRow label="Copies to">your skills directory</PropertyRow>
        <PropertyRow label="Integrity">directory hash pinned</PropertyRow>
        <PropertyRow label="Precedence">marketplace tier</PropertyRow>
      </div>
      <MarketplaceDetailRailNote>
        Workspace and agent-local skills with the same name shadow this copy.
      </MarketplaceDetailRailNote>
    </MarketplaceDetailRailCard>
  );
}

export {
  MarketplaceDetailSkillView,
  MarketplaceRepositoryRow,
  MarketplaceSkillDetailsCard,
  MarketplaceSkillReadmeSection,
  MarketplaceSkillTagsCard,
  MarketplaceSkillVersionsCard,
};
export type { MarketplaceDetailSkillViewProps };
