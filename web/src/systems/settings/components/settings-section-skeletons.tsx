import type { ComponentType } from "react";

import { ListingRowSkeleton, ListingToolbar, Section, Skeleton, SkeletonText } from "@compozy/ui";

import { ProfileSettingsListSkeleton } from "@/systems/profiles";

import type { SettingsSectionSlug } from "../types";
import { MarketplaceSourceRowSkeletons } from "./marketplace-source-row";
import { MCPServersTableSkeleton } from "./mcp-servers-table";
import { ProviderCardGrid, ProviderCardSkeleton } from "./provider-card";
import { ProvidersToolbarSkeleton } from "./providers-toolbar";
import { RoleListSkeleton } from "./role-list-skeleton";
import { SettingsAdvancedFoldSkeleton } from "./settings-advanced-fold";
import { SettingsChoiceGroupSkeleton } from "./settings-choice-group";
import { SettingsDisabledSkillsSkeleton } from "./settings-disabled-skills-section";
import { SettingsGroup } from "./settings-group";
import { SettingsHeroBoardSkeleton } from "./settings-hero-board";
import { SettingsHeroGaugeSkeleton } from "./settings-hero-gauge";
import { SettingsGroupSkeleton, SettingsPageSkeleton } from "./settings-page-skeleton";
import { SettingsSkillSourceRowSkeletons } from "./settings-skill-source-row";

// One loading shape per settings section, in the loaded page's own order of groups. Each is built
// from the real scaffold, groups and rows (see `settings-page-skeleton`), with the row and line
// counts the page typically has, so the swap to the loaded page moves nothing. The shapes live in
// the settings system rather than beside the route pages: the window shows them while a page's
// code is still loading, before the page module exists.
//
// Row ids and the id lists below are placeholder keys, not copy. A row's line count is the number
// of lines its consequence text wraps to in the loaded page.

const HOOK_ROWS = ["first", "second"] as const;
const WALLPAPER_CARDS = ["flat", "ember", "mesh", "carbon"] as const;
const SKILL_PRESET_ROWS = ["builtin", "universal", "claude"] as const;
const SKILL_CUSTOM_ROWS = ["team"] as const;

function pageTestId(slug: SettingsSectionSlug): string {
  return `settings-page-${slug}-skeleton`;
}

/** The icon well, title and description of an `Empty` state inside a group's panel. */
function EmptyPanelSkeleton({ descriptionLines }: { descriptionLines: number }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 py-8 text-center">
      <Skeleton className="size-empty-icon rounded-lg" />
      <SkeletonText className="w-56 text-empty-h1 leading-snug" />
      <SkeletonText
        className="w-80 text-small-body leading-relaxed"
        lastLineWidth={0.7}
        lines={descriptionLines}
      />
    </div>
  );
}

function GeneralSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("general")}>
      <SettingsGroupSkeleton
        description={1}
        rows={[
          { id: "agent", control: "input" },
          { id: "model", control: "selector" },
          { id: "streaming", control: "switch" },
        ]}
      />
      <SettingsGroupSkeleton>
        <SettingsChoiceGroupSkeleton />
      </SettingsGroupSkeleton>
      <Section
        divided
        label={<SkeletonText className="w-40" />}
        right={<Skeleton className="h-button-sm w-24 rounded-pill" />}
      >
        <div className="overflow-hidden rounded-lg bg-card shadow-card">
          <ListingRowSkeleton count={3} trail="pill" />
        </div>
      </Section>
      <SettingsGroupSkeleton
        rows={[
          { id: "follow-up", control: "segmented" },
          { id: "idle", control: "select" },
        ]}
      />
      <SettingsGroupSkeleton rows={[{ id: "redact", description: 2 }]} />
      <SettingsGroupSkeleton action description={1} rows={[{ id: "version", control: "value" }]} />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function AppearanceSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("appearance")}>
      <div className="flex flex-col gap-6">
        <SettingsGroupSkeleton
          rows={[
            { id: "theme", control: "segmented" },
            { id: "motion", control: "switch" },
          ]}
        />
        <SettingsGroup bare title={<SkeletonText className="w-24" />}>
          <div className="grid w-full grid-cols-[repeat(auto-fit,minmax(180px,1fr))] gap-3">
            {WALLPAPER_CARDS.map(card => (
              <div className="overflow-hidden rounded-lg border border-line bg-card" key={card}>
                <Skeleton className="aspect-video w-full rounded-none" />
                <div className="px-3 py-2 text-small-body">
                  <SkeletonText className="w-16" />
                </div>
              </div>
            ))}
          </div>
        </SettingsGroup>
      </div>
    </SettingsPageSkeleton>
  );
}

function TerminalSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("terminal")}>
      <SettingsGroupSkeleton
        rows={[
          { id: "shell", control: "input" },
          { id: "integration", description: 2 },
        ]}
      />
      <SettingsGroupSkeleton
        rows={[
          { id: "scrollback", control: "hinted" },
          { id: "reclaim", control: "input" },
          { id: "readable", control: "input" },
        ]}
      />
      <SettingsGroupSkeleton
        rows={[
          { id: "record", control: "switch" },
          { id: "days", control: "input" },
        ]}
      />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function LayoutsSkeleton() {
  return (
    <SettingsPageSkeleton subhead={2} testId={pageTestId("layouts")} width="canvas">
      <SettingsGroup
        action={
          <>
            <Skeleton className="h-button-sm w-36 rounded-pill" />
            <Skeleton className="h-button-sm w-36 rounded-pill" />
          </>
        }
        bare
        description={<SkeletonText className="w-80" />}
        title={<SkeletonText className="w-28" />}
      >
        <Skeleton className="h-137 w-full rounded-lg" />
      </SettingsGroup>
      <SettingsGroup bare title={<SkeletonText className="w-28" />}>
        <Skeleton className="h-60 w-full rounded-lg" />
      </SettingsGroup>
      <SettingsGroupSkeleton
        rows={[
          { id: "mode", control: "segmented" },
          { id: "gap", control: "input" },
          { id: "focus", control: "switch" },
        ]}
      />
    </SettingsPageSkeleton>
  );
}

function ProfilesSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("profiles")}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <SkeletonText className="w-80 flex-1 text-small-body" />
          <Skeleton className="h-button-sm w-32 rounded-pill" />
        </div>
        <ProfileSettingsListSkeleton />
        <div className="py-3">
          <SkeletonText className="w-28 text-body" />
        </div>
      </div>
    </SettingsPageSkeleton>
  );
}

function ProvidersSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("providers")} width="wide">
      <ProvidersToolbarSkeleton />
      <ProviderCardGrid aria-hidden="true">
        <ProviderCardSkeleton />
      </ProviderCardGrid>
    </SettingsPageSkeleton>
  );
}

function ProviderUsageSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("provider-usage")}>
      <SettingsGroupSkeleton
        rows={[
          { id: "enabled", description: 1 },
          { id: "interval", description: 2, control: "select" },
        ]}
      />
      <SettingsGroupSkeleton
        rows={[{ id: "openai" }, { id: "claude" }, { id: "openrouter" }, { id: "kimi" }]}
      />
      <SettingsGroupSkeleton rows={[{ id: "keychain", description: 2 }]} />
    </SettingsPageSkeleton>
  );
}

function RolesSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("roles")} width="wide">
      <RoleListSkeleton />
    </SettingsPageSkeleton>
  );
}

function SkillsSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("skills")}>
      <SettingsGroupSkeleton>
        <div className="flex flex-wrap items-center gap-2 px-4 py-3">
          <Skeleton className="h-5 w-44 rounded-pill" />
        </div>
      </SettingsGroupSkeleton>
      <SettingsGroupSkeleton description={1} rows={[{ id: "engine", control: "switch" }]} />
      <Section
        bodyClassName="gap-3"
        divided
        label={<SkeletonText className="w-24" />}
        right={<Skeleton className="h-button-sm w-40 rounded-pill" />}
      >
        <div className="overflow-hidden rounded-lg bg-card shadow-card">
          <SettingsSkillSourceRowSkeletons rows={SKILL_PRESET_ROWS} />
        </div>
        <div className="overflow-hidden rounded-lg bg-card shadow-card">
          <SettingsSkillSourceRowSkeletons rows={SKILL_CUSTOM_ROWS} />
          <div className="flex min-h-11 items-center justify-between gap-2 px-3 py-2.5">
            <SkeletonText className="w-36 text-body" />
            <Skeleton className="h-control-compact w-64 rounded-md" />
          </div>
        </div>
      </Section>
      <SettingsGroupSkeleton rows={[{ id: "scan", control: "input" }]} />
      <SettingsDisabledSkillsSkeleton />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function McpSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("mcp")} width="wide">
      <div className="flex flex-col gap-3">
        <ListingToolbar aria-hidden="true">
          <ListingToolbar.Leading>
            <Skeleton className="h-control-compact w-52 rounded-pill" />
            <Skeleton className="h-search min-w-search-input-floor shrink basis-search-input rounded-pill" />
          </ListingToolbar.Leading>
        </ListingToolbar>
        <MCPServersTableSkeleton />
      </div>
    </SettingsPageSkeleton>
  );
}

function AutomationSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("automation")}>
      <SettingsHeroBoardSkeleton />
      <SettingsGroupSkeleton
        rows={[
          { id: "engine", control: "switch" },
          { id: "timezone", control: "select" },
        ]}
      />
      <SettingsGroupSkeleton rows={[{ id: "automations", description: 1, control: "none" }]} />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function GatewaySkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("gateway")} width="wide">
      <SettingsGroupSkeleton
        rows={[
          { id: "private", description: 2 },
          { id: "public-ingress", description: 2 },
          { id: "public-operator", description: 3 },
        ]}
      />
      <SettingsGroupSkeleton>
        <EmptyPanelSkeleton descriptionLines={2} />
      </SettingsGroupSkeleton>
      <SettingsGroupSkeleton action>
        <EmptyPanelSkeleton descriptionLines={2} />
      </SettingsGroupSkeleton>
      <SettingsGroupSkeleton action>
        <EmptyPanelSkeleton descriptionLines={1} />
      </SettingsGroupSkeleton>
    </SettingsPageSkeleton>
  );
}

function PaletteSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("palette")}>
      <SettingsGroupSkeleton
        rows={[
          { id: "fallback", control: "switch" },
          { id: "personalization", control: "switch" },
          { id: "reset", description: 1, control: "button" },
        ]}
      />
    </SettingsPageSkeleton>
  );
}

function AttentionSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("attention")}>
      <SettingsGroupSkeleton
        rows={[
          { id: "toasts", control: "switch" },
          { id: "sound", control: "switch" },
          { id: "system", description: 2, control: "switch" },
        ]}
      />
      <SettingsGroupSkeleton rows={[{ id: "mute", description: 1, control: "select" }]}>
        <p className="px-4 py-3">
          <SkeletonText className="w-40 text-form-label" />
        </p>
      </SettingsGroupSkeleton>
    </SettingsPageSkeleton>
  );
}

function ObservabilitySkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("observability")}>
      <SettingsHeroGaugeSkeleton />
      <SettingsGroupSkeleton
        rows={[
          { id: "record", control: "switch" },
          { id: "retention", description: 1, control: "input" },
        ]}
      />
      <SettingsGroupSkeleton rows={[{ id: "transcripts", control: "switch" }]} />
      <SettingsGroupSkeleton
        rows={[
          { id: "bundle", control: "button" },
          { id: "log", description: 1, control: "button" },
        ]}
      />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function HooksSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("hooks")}>
      <SettingsGroup
        bare
        description={<SkeletonText className="w-96" />}
        title={<SkeletonText className="w-32" />}
      >
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <SkeletonText className="w-24 text-small-body" />
            <Skeleton className="h-search w-56 rounded-pill" />
            <Skeleton className="ml-auto h-control-compact w-36 rounded-pill" />
          </div>
          <ul className="overflow-hidden rounded-lg bg-card shadow-card">
            {HOOK_ROWS.map(row => (
              <li className="border-b border-line-soft last:border-b-0" key={row}>
                <ListingRowSkeleton className="border-b-0 py-2.5" count={1} trail="pill" />
              </li>
            ))}
          </ul>
        </div>
      </SettingsGroup>
    </SettingsPageSkeleton>
  );
}

function ExtensionsSkeleton() {
  return (
    <SettingsPageSkeleton subhead={1} testId={pageTestId("extensions")}>
      <SettingsGroupSkeleton>
        <ListingRowSkeleton count={2} meta={false} trail="none" />
      </SettingsGroupSkeleton>
      <SettingsGroupSkeleton
        rows={[
          { id: "github", control: "switch" },
          { id: "git", control: "switch" },
        ]}
      />
      <SettingsGroupSkeleton
        description={2}
        rows={[{ id: "unverified", description: 1, control: "switch" }]}
      />
      <SettingsGroupSkeleton rows={[{ id: "manage", control: "none" }]} />
      <SettingsAdvancedFoldSkeleton />
    </SettingsPageSkeleton>
  );
}

function MarketplaceSkeleton() {
  return (
    <SettingsPageSkeleton testId={pageTestId("marketplace")}>
      <SettingsGroupSkeleton
        rows={[
          { id: "feed", description: 1, control: "input" },
          { id: "ttl", description: 1, control: "input" },
          { id: "timeout", description: 2, control: "input" },
        ]}
      />
      <SettingsGroupSkeleton action description={2}>
        <MarketplaceSourceRowSkeletons />
      </SettingsGroupSkeleton>
    </SettingsPageSkeleton>
  );
}

const SECTION_SKELETONS: Record<SettingsSectionSlug, ComponentType> = {
  general: GeneralSkeleton,
  terminal: TerminalSkeleton,
  appearance: AppearanceSkeleton,
  layouts: LayoutsSkeleton,
  palette: PaletteSkeleton,
  profiles: ProfilesSkeleton,
  providers: ProvidersSkeleton,
  "provider-usage": ProviderUsageSkeleton,
  roles: RolesSkeleton,
  skills: SkillsSkeleton,
  mcp: McpSkeleton,
  automation: AutomationSkeleton,
  gateway: GatewaySkeleton,
  attention: AttentionSkeleton,
  observability: ObservabilitySkeleton,
  hooks: HooksSkeleton,
  extensions: ExtensionsSkeleton,
  marketplace: MarketplaceSkeleton,
};

/**
 * The loading shape of one settings section, for the page's own pending state and for the window
 * while the page's code loads, so the two read as one continuous skeleton. It carries no loading
 * semantics of its own: wrap it in the page's single `SkeletonRegion`.
 */
export function SettingsSectionSkeleton({ slug }: { slug: SettingsSectionSlug }) {
  const Shape = SECTION_SKELETONS[slug];
  return <Shape />;
}
