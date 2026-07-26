import {
  Button,
  cn,
  DescriptionCard,
  Empty,
  Eyebrow,
  MetadataList,
  PAGE_CONTENT_GUTTER,
  Pill,
  Skeleton,
} from "@agh/ui";
import { PackageX } from "lucide-react";
import type { ReactNode } from "react";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailExtensionManage } from "./marketplace-detail-extension-manage";
import { MarketplaceDetailMeta } from "./marketplace-detail-meta";
import { MarketplaceDetailMCPManage } from "./marketplace-detail-mcp-manage";
import { MarketplaceDetailSkillManage } from "./marketplace-detail-skill-manage";
import { formatMarketplaceCount } from "./marketplace-ui";

interface MarketplaceDetailProps {
  data: MarketplaceEntryResponse;
  managementScope?: "global" | "workspace";
  managementWorkspaceId?: string;
}

function MarketplaceDetail({
  data,
  managementScope,
  managementWorkspaceId,
}: MarketplaceDetailProps) {
  const entry = data.entry;

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto" data-testid="marketplace-detail">
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pb-20")}>
        <MarketplaceDetailMeta entry={entry} />

        <div className="grid grid-cols-1 gap-8 pt-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <main className="flex min-w-0 flex-col gap-7">
            <MarketplaceDetailMain data={data} />
          </main>
          <aside className="flex min-w-0 flex-col gap-6">
            {entry.installed ? <MarketplaceInstalledRail entry={entry} /> : null}
            {entry.installed ? (
              <MarketplaceDetailManage
                entry={entry}
                mcpScope={managementScope}
                mcpWorkspaceId={managementWorkspaceId}
              />
            ) : null}
            <MarketplaceDetailsRail data={data} />
            {entry.trust ? <MarketplaceTrustRail trust={entry.trust} /> : null}
          </aside>
        </div>
      </div>
    </div>
  );
}

function MarketplaceDetailMain({ data }: { data: MarketplaceEntryResponse }) {
  const { entry } = data;
  const content = data.skill?.readme?.trim() || entry.description;
  return (
    <>
      <section className="flex flex-col gap-2.5">
        <Eyebrow className="text-muted">Description</Eyebrow>
        <DescriptionCard>{content}</DescriptionCard>
      </section>
      {data.extension ? (
        <section className="flex flex-col gap-2.5">
          <Eyebrow className="text-muted">Provenance</Eyebrow>
          <DescriptionCard>{extensionProvenanceMarkdown(data.extension)}</DescriptionCard>
        </section>
      ) : null}
      {entry.trust?.warnings?.length ? (
        <MarketplaceWarningList warnings={entry.trust.warnings} />
      ) : null}
      {data.bundle ? <MarketplaceBundleDetails bundle={data.bundle} /> : null}
      {data.mcp ? <MarketplaceMCPDetails mcp={data.mcp} /> : null}
    </>
  );
}

function extensionProvenanceMarkdown(
  extension: NonNullable<MarketplaceEntryResponse["extension"]>
) {
  return [
    extension.repository ? `Source: [Repository](${extension.repository})` : null,
    `Artifact: [Pinned archive](${extension.artifact_url})`,
    `SHA-256: \`${extension.digest_sha256}\``,
  ]
    .filter(Boolean)
    .join("\n\n");
}

function MarketplaceBundleDetails({
  bundle,
}: {
  bundle: NonNullable<MarketplaceEntryResponse["bundle"]>;
}) {
  return (
    <section className="flex flex-col gap-2.5">
      <Eyebrow className="text-muted">Profiles</Eyebrow>
      <div className="overflow-hidden rounded-lg bg-canvas-soft">
        {bundle.profiles.map(profile => (
          <div
            className="grid gap-2 border-b border-line px-5 py-4 last:border-b-0 md:grid-cols-[minmax(0,1fr)_auto]"
            key={profile.name}
          >
            <div>
              <p className="text-small-body font-medium text-fg-strong">{profile.name}</p>
              {profile.description ? (
                <p className="mt-1 text-small-body text-muted">{profile.description}</p>
              ) : null}
            </div>
            <p className="font-mono text-mono-id text-muted">
              {profile.agents} agents · {profile.jobs} jobs · {profile.triggers} triggers ·
              {profile.bridges} bridges · {profile.channels} channels
            </p>
          </div>
        ))}
      </div>
    </section>
  );
}

function MarketplaceMCPDetails({ mcp }: { mcp: NonNullable<MarketplaceEntryResponse["mcp"]> }) {
  const command = [mcp.command, ...(mcp.args ?? [])].filter(Boolean).join(" ");
  return (
    <>
      <section className="flex flex-col gap-2.5">
        <Eyebrow className="text-muted">Configuration</Eyebrow>
        <div className="flex flex-col gap-4 rounded-lg bg-canvas-soft p-5">
          <div className="min-w-0 overflow-x-auto rounded-md bg-canvas px-3 py-2 font-mono text-form-input text-fg">
            {mcp.url || command || mcp.transport}
          </div>
          <MetadataList>
            <MetadataList.Row label="Transport">{mcp.transport}</MetadataList.Row>
            {mcp.default_scope ? (
              <MetadataList.Row label="Default scope">{mcp.default_scope}</MetadataList.Row>
            ) : null}
          </MetadataList>
          {mcp.env?.length ? (
            <div className="flex flex-col gap-2">
              <p className="text-small-body font-medium text-fg-strong">Required configuration</p>
              {mcp.env.map(field => (
                <div
                  className="flex items-start justify-between gap-4 border-t border-line pt-2"
                  key={field.name}
                >
                  <code className="font-mono text-form-input text-fg">{field.name}</code>
                  <span className="text-right text-form-hint text-muted">
                    {field.prompt || (field.required ? "Required" : "Optional")}
                  </span>
                </div>
              ))}
            </div>
          ) : null}
        </div>
      </section>
      {mcp.oauth ? (
        <section className="flex flex-col gap-2.5">
          <Eyebrow className="text-muted">Authorization</Eyebrow>
          <MetadataList className="rounded-lg bg-canvas-soft px-4 py-3">
            <MetadataList.Row
              className="grid-cols-1 items-start sm:grid-cols-[7.5rem_minmax(0,1fr)] sm:items-center"
              label="Type"
            >
              OAuth 2 PKCE
            </MetadataList.Row>
            {mcp.oauth.issuer_url ? (
              <MetadataList.Row
                className="grid-cols-1 items-start sm:grid-cols-[7.5rem_minmax(0,1fr)] sm:items-center"
                label="Issuer"
                valueProps={{ className: "break-all" }}
              >
                {mcp.oauth.issuer_url}
              </MetadataList.Row>
            ) : null}
            {mcp.oauth.scopes?.length ? (
              <MetadataList.Row
                className="grid-cols-1 items-start sm:grid-cols-[7.5rem_minmax(0,1fr)] sm:items-center"
                label="Scopes"
                valueProps={{ className: "break-words" }}
              >
                {mcp.oauth.scopes.join(", ")}
              </MetadataList.Row>
            ) : null}
          </MetadataList>
        </section>
      ) : null}
    </>
  );
}

type TrustWarnings = NonNullable<
  NonNullable<MarketplaceEntryResponse["entry"]["trust"]>["warnings"]
>;

function MarketplaceWarningList({ warnings }: { warnings: TrustWarnings }) {
  return (
    <section className="flex flex-col gap-2.5">
      <Eyebrow className="text-muted">Trust warnings</Eyebrow>
      <div className="flex flex-col gap-2">
        {warnings.map(warning => {
          const tone =
            warning.severity === "error"
              ? "danger"
              : warning.severity === "info"
                ? "info"
                : "warning";
          const surface =
            tone === "danger"
              ? "border-line bg-danger-tint"
              : tone === "info"
                ? "border-line bg-info-tint"
                : "border-line bg-warning-tint";
          return (
            <div className={`rounded-md border px-4 py-3 ${surface}`} key={warning.id}>
              <div className="flex items-center gap-2">
                <Pill mono tone={tone}>
                  {warning.severity}
                </Pill>
                <p className="text-small-body font-medium text-fg-strong">{warning.title}</p>
              </div>
              <p className="mt-1.5 text-small-body text-muted">{warning.message}</p>
              {warning.suggested_command ? (
                <code className="mt-2 inline-flex rounded bg-canvas px-2 py-1 font-mono text-form-hint text-fg">
                  {warning.suggested_command}
                </code>
              ) : null}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function MarketplaceInstalledRail({ entry }: { entry: MarketplaceEntryResponse["entry"] }) {
  return (
    <section className="flex flex-col gap-2.5">
      <Eyebrow className="text-muted">Installed</Eyebrow>
      <MarketplaceKvRail>
        <MarketplaceKvRow term="Status">
          <Pill mono tone="success">
            installed
          </Pill>
        </MarketplaceKvRow>
        {entry.installed_version ? (
          <MarketplaceKvRow term="Version">{`v${entry.installed_version}`}</MarketplaceKvRow>
        ) : null}
      </MarketplaceKvRail>
    </section>
  );
}

function MarketplaceDetailManage({
  entry,
  mcpScope,
  mcpWorkspaceId,
}: {
  entry: MarketplaceEntryResponse["entry"];
  mcpScope?: "global" | "workspace";
  mcpWorkspaceId?: string;
}) {
  const name = entry.installed_name?.trim() || entry.name;
  if (entry.kind === "skill") return <MarketplaceDetailSkillManage name={name} />;
  if (entry.kind === "extension") return <MarketplaceDetailExtensionManage name={name} />;
  if (entry.kind === "mcp") {
    return (
      <MarketplaceDetailMCPManage entry={entry} scope={mcpScope} workspaceId={mcpWorkspaceId} />
    );
  }
  return null;
}

function MarketplaceDetailsRail({ data }: { data: MarketplaceEntryResponse }) {
  const { entry } = data;
  return (
    <section className="flex flex-col gap-2.5">
      <Eyebrow className="text-muted">Details</Eyebrow>
      <MarketplaceKvRail>
        {entry.version ? (
          <MarketplaceKvRow term="Version">{`v${entry.version}`}</MarketplaceKvRow>
        ) : null}
        {entry.author ? <MarketplaceKvRow term="Author">{entry.author}</MarketplaceKvRow> : null}
        <MarketplaceKvRow term="Source">{entry.source}</MarketplaceKvRow>
        <MarketplaceKvRow term="Entry ID">{entry.entry_id}</MarketplaceKvRow>
        {entry.transport ? (
          <MarketplaceKvRow term="Transport">{entry.transport}</MarketplaceKvRow>
        ) : null}
        {data.skill?.license ? (
          <MarketplaceKvRow term="License">{data.skill.license}</MarketplaceKvRow>
        ) : null}
        {entry.downloads !== undefined && entry.downloads !== null ? (
          <MarketplaceKvRow term="Downloads">
            {formatMarketplaceCount(entry.downloads)}
          </MarketplaceKvRow>
        ) : null}
      </MarketplaceKvRail>
    </section>
  );
}

function MarketplaceTrustRail({
  trust,
}: {
  trust: NonNullable<MarketplaceEntryResponse["entry"]["trust"]>;
}) {
  return (
    <section className="flex flex-col gap-2.5">
      <Eyebrow className="text-muted">Trust</Eyebrow>
      <MarketplaceKvRail>
        <MarketplaceKvRow term="Decision">{trust.decision.replaceAll("_", " ")}</MarketplaceKvRow>
        <MarketplaceKvRow term="Registry tier">{trust.registry_tier}</MarketplaceKvRow>
        <MarketplaceKvRow term="Checksum">
          {trust.checksum_verified ? "verified" : "checked during install"}
        </MarketplaceKvRow>
        <MarketplaceKvRow term="Warnings">{String(trust.warnings?.length ?? 0)}</MarketplaceKvRow>
      </MarketplaceKvRail>
    </section>
  );
}

function MarketplaceKvRail({ children }: { children: ReactNode }) {
  return (
    <dl className="divide-y divide-line-soft overflow-hidden rounded-lg bg-canvas-soft">
      {children}
    </dl>
  );
}

function MarketplaceKvRow({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 px-4 py-3">
      <dt className="eyebrow text-muted">{term}</dt>
      <dd className="min-w-0 break-words text-xs text-fg">{children}</dd>
    </div>
  );
}

function MarketplaceDetailSkeleton() {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col gap-6 pb-20")} role="status">
      <div className="flex items-start gap-3.5 border-b border-line py-5">
        <Skeleton className="size-(--size-provider-logo-well) shrink-0 rounded-lg" />
        <div className="flex min-w-0 flex-1 flex-col gap-3">
          <Skeleton className="h-5.5 w-55 max-w-full" />
          <Skeleton className="h-3 w-75 max-w-full" />
        </div>
        <Skeleton className="hidden h-8 w-25 shrink-0 rounded-md sm:block" />
      </div>
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="flex flex-col gap-2.5 rounded-lg border border-line bg-canvas-soft p-5">
          {[78, 91, 66, 82, 94, 72].map(width => (
            <Skeleton className="h-2.5" key={width} style={{ width: `${width}%` }} />
          ))}
        </div>
        <aside className="flex flex-col gap-2.5">
          <Eyebrow className="text-muted">Details</Eyebrow>
          <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
            {[38, 54, 42, 58].map(width => (
              <div className="border-t border-line-soft p-4 first:border-t-0" key={width}>
                <Skeleton className="mb-2 h-2.5" style={{ width: `${width}%` }} />
                <Skeleton className="h-2.5" style={{ width: `${Math.min(width + 32, 82)}%` }} />
              </div>
            ))}
          </div>
        </aside>
      </div>
      <span className="sr-only">Loading marketplace entry</span>
    </div>
  );
}

function MarketplaceDetailNotFound({ onBack }: { onBack: () => void }) {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col pb-20")}>
      <Empty
        fill={false}
        className="py-14"
        titleAs="h2"
        icon={PackageX}
        title="This item is no longer in the marketplace"
        description="The catalog source no longer returns this entry."
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to marketplace
          </Button>
        }
      />
    </div>
  );
}

export { MarketplaceDetail, MarketplaceDetailNotFound, MarketplaceDetailSkeleton };
export type { MarketplaceDetailProps };
