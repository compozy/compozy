import { Link } from "@tanstack/react-router";
import { createElement, Fragment, type ReactNode } from "react";

import { Pill, Time } from "@compozy/ui";

import type { MarketplaceEntryResponse, MarketplaceKind } from "../types";
import {
  formatMarketplaceCount,
  formatMarketplaceVersion,
  marketplaceKindIcon,
  MARKETPLACE_KIND_SINGULAR,
  stripMarketplaceUrlScheme,
} from "./marketplace-ui";
import { ExtensionFormatBadge } from "@/systems/extensions";

interface MarketplaceDetailLedeProps {
  data: MarketplaceEntryResponse;
}

/**
 * Page lede: icon well, entry name with kind/version tags, one metadata line,
 * and the one-sentence description. Window identity and the primary action
 * stay in the OS head.
 */
function MarketplaceDetailLede({ data }: MarketplaceDetailLedeProps) {
  const entry = data.entry;
  const kind = entry.kind as MarketplaceKind;
  const icon = marketplaceKindIcon(kind);
  const blocked = kind === "extension" && entry.trust?.decision === "blocked";
  const description = entry.description?.trim();
  const meta = ledeMeta(data);

  return (
    <header
      className="mb-6 flex items-start gap-3.5 border-b border-line pb-5"
      data-testid="marketplace-detail-lede"
    >
      <span
        aria-hidden="true"
        className="grid size-(--size-provider-logo-well) shrink-0 place-items-center rounded-lg border border-line bg-elevated text-muted"
      >
        {createElement(icon, { className: "size-5" })}
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h1 className="text-detail-h1 leading-tight font-semibold tracking-detail-h1 text-fg-strong">
            {entry.name}
          </h1>
          <Pill mono size="xs">
            {MARKETPLACE_KIND_SINGULAR[kind]}
          </Pill>
          {ledeSecondTag(data)}
          <ExtensionFormatBadge format={entry.format} />
        </div>
        {meta.length > 0 ? (
          <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-2 text-form-label text-subtle">
            {meta.map((item, index) => (
              <Fragment key={item.key}>
                {index > 0 ? (
                  <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                ) : null}
                {item.node}
              </Fragment>
            ))}
          </div>
        ) : null}
        {description ? (
          <p className="mt-2.5 max-w-[70ch] text-small-body leading-relaxed text-muted">
            {description}
          </p>
        ) : null}
        {blocked ? (
          <p className="mt-2 text-form-hint text-danger">
            Blocked by extensions policy.
            <Link className="ml-1 underline underline-offset-3" to="/settings/extensions">
              Settings › Extensions
            </Link>
          </p>
        ) : null}
      </div>
    </header>
  );
}

function ledeSecondTag(data: MarketplaceEntryResponse): ReactNode {
  const entry = data.entry;
  if (entry.kind === "mcp") {
    const launchType = data.mcp?.launch.type;
    const tag = launchType === "remote" ? "remote" : (entry.transport ?? launchType);
    return tag ? (
      <Pill mono size="xs">
        {tag}
      </Pill>
    ) : null;
  }
  const version = formatMarketplaceVersion(entry.version);
  return version ? (
    <Pill mono size="xs">
      {version}
    </Pill>
  ) : null;
}

interface LedeMetaItem {
  key: string;
  node: ReactNode;
}

function ledeMeta(data: MarketplaceEntryResponse): LedeMetaItem[] {
  const entry = data.entry;
  const items: LedeMetaItem[] = [];
  const slug = entry.install_slug?.trim();
  if (entry.kind === "skill" && slug) {
    items.push({ key: "slug", node: <span className="font-mono text-form-hint">{slug}</span> });
  }
  if (entry.kind === "mcp") {
    const endpoint = data.mcp?.launch.type === "remote" ? data.mcp.launch.url : undefined;
    items.push({ key: "source", node: <span>{entry.source}</span> });
    if (endpoint) {
      items.push({
        key: "endpoint",
        node: (
          <span className="font-mono text-form-hint">{stripMarketplaceUrlScheme(endpoint)}</span>
        ),
      });
    }
    if (data.mcp?.default_scope) {
      items.push({ key: "scope", node: <span>{data.mcp.default_scope} scope</span> });
    }
  }
  if (entry.author) items.push({ key: "author", node: <span>{entry.author}</span> });
  if (entry.kind !== "mcp") {
    items.push({ key: "src", node: <span>{entry.source}</span> });
  }
  if (entry.downloads !== undefined && entry.downloads !== null) {
    items.push({
      key: "downloads",
      node: <span>{formatMarketplaceCount(entry.downloads)} downloads</span>,
    });
  }
  const license = data.skill?.license?.trim();
  if (license) items.push({ key: "license", node: <span>{license}</span> });
  if (entry.tier) items.push({ key: "tier", node: <span>{entry.tier} tier</span> });
  if (entry.updated_at) {
    items.push({
      key: "updated",
      node: (
        <span>
          updated <Time iso={entry.updated_at} />
        </span>
      ),
    });
  }
  return items;
}

export { MarketplaceDetailLede };
export type { MarketplaceDetailLedeProps };
