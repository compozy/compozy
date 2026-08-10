import { FileCode, GitFork, Layers, SlidersHorizontal, Zap } from "lucide-react";

import {
  Button,
  CodeBlock,
  Pill,
  PropertyRow,
  Spinner,
  StatusDot,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  Time,
} from "@compozy/ui";

import {
  deriveSkillCapabilities,
  deriveSkillRecentCalls,
  SkillActivationPill,
  SkillActivationReasons,
  skillSourceTone,
  type SkillPayload,
} from "@/systems/skill";

import { useMarketplaceDetailSkillManage } from "../hooks/use-marketplace-detail-skill-manage";
import { MarketplaceDetailManageFallbackBody } from "./marketplace-detail-manage-state";
import {
  MarketplaceDetailColumns,
  MarketplaceDetailRailCard,
  MarketplaceDetailSection,
} from "./marketplace-detail-shell";
import {
  MarketplaceSkillDetailsCard,
  MarketplaceSkillReadmeSection,
  MarketplaceSkillTagsCard,
  MarketplaceSkillVersionsCard,
} from "./marketplace-detail-skill";
import { MarketplaceDetailWarnings } from "./marketplace-detail-warnings";
import type { MarketplaceEntryResponse } from "../types";

interface MarketplaceDetailSkillInstalledProps {
  data: MarketplaceEntryResponse;
}

/** Installed skill detail: readme + live call history in the body, management in the rail. */
function MarketplaceDetailSkillInstalled({ data }: MarketplaceDetailSkillInstalledProps) {
  const name = data.entry.installed_name?.trim() || data.entry.name;
  const state = useMarketplaceDetailSkillManage(name);
  const skill = state.skill;

  return (
    <MarketplaceDetailColumns
      main={
        <>
          <MarketplaceSkillReadmeSection data={data} />
          {skill ? (
            <>
              <MarketplaceSkillRecentCallsSection skill={skill} />
              <MarketplaceSkillContentSection
                content={state.contentQuery.data}
                error={state.contentQuery.error}
                isLoading={state.contentRequested && state.contentQuery.isLoading}
                onRetry={() => void state.contentQuery.refetch()}
                onView={() => state.setContentRequested(true)}
              />
              <MarketplaceSkillResolutionSection
                error={state.shadowsQuery.error}
                isLoading={state.shadowsQuery.isLoading}
                shadows={state.shadowsQuery.data}
              />
            </>
          ) : null}
          <MarketplaceDetailWarnings warnings={data.entry.trust?.warnings} />
        </>
      }
      rail={
        <>
          <MarketplaceSkillManageCard state={state} />
          {skill ? <MarketplaceSkillCapabilitiesCard skill={skill} /> : null}
          <MarketplaceSkillDetailsCard data={data} />
          <MarketplaceSkillVersionsCard data={data} />
          <MarketplaceSkillTagsCard tags={data.skill?.tags} />
          {skill?.provenance ? (
            <MarketplaceSkillProvenanceCard provenance={skill.provenance} />
          ) : null}
        </>
      }
    />
  );
}

function MarketplaceSkillManageCard({
  state,
}: {
  state: ReturnType<typeof useMarketplaceDetailSkillManage>;
}) {
  const skill = state.skill;
  return (
    <MarketplaceDetailRailCard
      icon={SlidersHorizontal}
      summary={skill ? (skill.enabled ? "enabled" : "disabled") : undefined}
      title="Manage"
    >
      {skill ? (
        <div className="flex flex-col gap-1 px-3.5">
          <PropertyRow
            editor={
              <Switch
                aria-labelledby="marketplace-skill-enabled-label"
                checked={skill.enabled === true}
                data-testid="skill-enabled-switch"
                disabled={state.acting}
                onCheckedChange={state.toggleEnabled}
              />
            }
            label={
              <span id="marketplace-skill-enabled-label">
                {skill.enabled ? "Enabled" : "Disabled"}
              </span>
            }
          />
          {state.toggleError ? (
            <p
              className="pb-1.5 text-form-hint text-danger"
              data-testid="marketplace-skill-toggle-error"
              role="alert"
            >
              {state.toggleError}
            </p>
          ) : null}
          <div className="flex flex-col gap-1.5 pt-1 pb-1.5" data-testid="skill-activation-section">
            <div className="flex min-w-0 items-center justify-between gap-3">
              <span className="text-form-label text-subtle">Activation</span>
              <SkillActivationPill
                active={skill.activation.active}
                data-testid="skill-detail-activation-status"
              />
            </div>
            {!skill.activation.active ? (
              <SkillActivationReasons
                data-testid="skill-detail-activation-reasons"
                reasons={skill.activation.reasons ?? []}
              />
            ) : null}
          </div>
        </div>
      ) : (
        <MarketplaceDetailManageFallbackBody
          error={state.skillQuery.error}
          errorMessage="Skill management could not be loaded."
          isLoading={state.skillQuery.isLoading}
          loadingMessage="Loading skill management…"
          onRetry={() => void state.skillQuery.refetch()}
          testId={
            state.skillQuery.isLoading
              ? "marketplace-skill-manage-loading"
              : "marketplace-skill-manage-error"
          }
        />
      )}
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceSkillCapabilitiesCard({ skill }: { skill: SkillPayload }) {
  const capabilities = deriveSkillCapabilities(skill);
  return (
    <MarketplaceDetailRailCard
      icon={Zap}
      summary={String(capabilities.length)}
      title="Capabilities"
    >
      {capabilities.length ? (
        <div className="flex flex-wrap gap-1.5 px-3.5 py-1.5" data-testid="skill-capabilities-list">
          {capabilities.map(capability => (
            <Pill mono key={capability}>
              {capability}
            </Pill>
          ))}
        </div>
      ) : (
        <p className="px-3.5 py-1.5 text-small-body text-muted">None declared.</p>
      )}
    </MarketplaceDetailRailCard>
  );
}

function MarketplaceSkillProvenanceCard({
  provenance,
}: {
  provenance: NonNullable<SkillPayload["provenance"]>;
}) {
  const rows = [
    ["Tier", provenance.precedence_tier],
    ["Extension", provenance.installed_from_extension],
    ["Registry", provenance.registry],
    ["Slug", provenance.slug],
    ["Version", provenance.version],
  ].filter((row): row is [string, string] => typeof row[1] === "string" && row[1].trim() !== "");
  if (rows.length === 0) return null;
  return (
    <MarketplaceDetailRailCard
      data-testid="skill-provenance-table"
      defaultOpen={false}
      icon={GitFork}
      summary={provenance.precedence_tier ?? undefined}
      title="Provenance"
    >
      <div className="px-3.5">
        {rows.map(([label, value]) => (
          <PropertyRow key={label} label={label} mono>
            {value}
          </PropertyRow>
        ))}
      </div>
    </MarketplaceDetailRailCard>
  );
}

const RECENT_CALL_TONE = {
  error: "danger",
  pending: "accent",
  success: "faint",
} as const;

function MarketplaceSkillRecentCallsSection({ skill }: { skill: SkillPayload }) {
  const calls = deriveSkillRecentCalls(skill);
  return (
    <MarketplaceDetailSection
      icon={Zap}
      summary={calls.length ? String(calls.length) : "none recorded"}
      title="Recent calls"
    >
      {calls.length ? (
        <div data-testid="skill-recent-calls-table">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-20">Status</TableHead>
                <TableHead>Call</TableHead>
                <TableHead className="w-30 text-right">When</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {calls.map(call => (
                <TableRow key={`${call.label}-${call.timestamp ?? call.status}`}>
                  <TableCell>
                    <StatusDot
                      label={call.status}
                      tone={RECENT_CALL_TONE[call.status]}
                      variant={call.status === "pending" ? "ring" : "solid"}
                    />
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted">{call.label}</TableCell>
                  <TableCell className="text-right font-mono text-eyebrow text-subtle">
                    {call.timestamp ? <Time iso={call.timestamp} /> : "--"}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : (
        <p className="px-4 py-3 text-small-body text-muted">No recent invocations recorded.</p>
      )}
    </MarketplaceDetailSection>
  );
}

function MarketplaceSkillContentSection({
  content,
  error,
  isLoading,
  onRetry,
  onView,
}: {
  content: string | undefined;
  error: Error | null;
  isLoading: boolean;
  onRetry: () => void;
  onView: () => void;
}) {
  return (
    <MarketplaceDetailSection
      icon={FileCode}
      summary={content ? "full skill instructions" : "loaded on demand"}
      title="Content"
    >
      {content ? (
        <div data-testid="content-body">
          <CodeBlock code={content} copyable={false} showPrompt={false} truncateLines={16} />
        </div>
      ) : isLoading ? (
        <div
          className="flex items-center gap-2 px-4 py-3 text-small-body text-muted"
          data-testid="content-loading"
          role="status"
        >
          <Spinner aria-hidden="true" className="size-4 text-subtle" />
          Loading full skill content…
        </div>
      ) : error ? (
        <div className="flex flex-col gap-2 px-4 py-3" data-testid="content-error">
          <p className="text-small-body text-danger">
            {error.message.trim() || "Failed to load full content."}
          </p>
          <div>
            <Button
              data-testid="retry-view-content-btn"
              onClick={onRetry}
              size="sm"
              type="button"
              variant="outline"
            >
              Try again
            </Button>
          </div>
        </div>
      ) : (
        <div className="flex flex-col gap-2 px-4 py-3" data-testid="content-empty">
          <p className="text-small-body leading-relaxed text-muted">
            Full skill instructions are loaded on demand.
          </p>
          <div>
            <Button
              data-testid="view-full-content-btn"
              onClick={() => onView()}
              size="sm"
              type="button"
              variant="outline"
            >
              View full content
            </Button>
          </div>
        </div>
      )}
    </MarketplaceDetailSection>
  );
}

function MarketplaceSkillResolutionSection({
  error,
  isLoading,
  shadows,
}: {
  error: Error | null;
  isLoading: boolean;
  shadows:
    | {
        shadows: {
          detected_at: string;
          path: string;
          resolved_to_winner: boolean;
          tier: string;
        }[];
      }
    | undefined;
}) {
  if (!isLoading && !error && (!shadows || shadows.shadows.length === 0)) return null;
  return (
    <MarketplaceDetailSection
      icon={Layers}
      summary={shadows?.shadows.length ? `${shadows.shadows.length} resolver paths` : undefined}
      title="Resolution"
    >
      {isLoading ? (
        <div
          className="flex items-center gap-2 px-4 py-3 text-small-body text-muted"
          data-testid="skill-shadows-loading"
        >
          <Spinner aria-hidden="true" className="size-4 text-subtle" />
          Loading resolver paths…
        </div>
      ) : error ? (
        <p className="px-4 py-3 text-small-body text-danger">
          {error.message.trim() || "Failed to load skill resolution."}
        </p>
      ) : shadows && shadows.shadows.length > 0 ? (
        <div data-testid="skill-shadow-table">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-20">Winner</TableHead>
                <TableHead className="w-28">Tier</TableHead>
                <TableHead>Path</TableHead>
                <TableHead className="w-30 text-right">Detected</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {shadows.shadows.map(entry => (
                <TableRow key={`${entry.tier}-${entry.path}`}>
                  <TableCell>
                    <StatusDot
                      label={entry.resolved_to_winner ? "winner" : "shadowed"}
                      tone={entry.resolved_to_winner ? "accent" : "faint"}
                      variant={entry.resolved_to_winner ? "solid" : "ring"}
                    />
                  </TableCell>
                  <TableCell>
                    <Pill mono tone={skillSourceTone(entry.tier)}>
                      {entry.tier}
                    </Pill>
                  </TableCell>
                  <TableCell className="max-w-0 truncate font-mono text-xs text-muted">
                    {entry.path}
                  </TableCell>
                  <TableCell className="text-right font-mono text-eyebrow text-subtle">
                    <Time iso={entry.detected_at} />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      ) : null}
    </MarketplaceDetailSection>
  );
}

export { MarketplaceDetailSkillInstalled };
export type { MarketplaceDetailSkillInstalledProps };
