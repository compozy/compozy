import {
  Activity,
  Brain,
  Cpu,
  Network,
  Palette,
  PanelTop,
  Puzzle,
  Route,
  SlidersHorizontal,
  Webhook,
  Wrench,
  Zap,
} from "lucide-react";

import type {
  SettingsSectionDescriptor,
  SettingsSectionGroup,
  SettingsSectionSlug,
} from "../types";

export const SETTINGS_ROOT_PATH = "/settings" as const;

/**
 * Daemon sections + browser Appearance, grouped so the nav reads as a mental
 * model (design system §03): Workspace = "how my work is set up", Runtime =
 * "what my agents can do", System = "how the daemon itself behaves".
 * Keywords feed the sidebar search.
 */
export const SETTINGS_SECTIONS: readonly SettingsSectionDescriptor[] = [
  {
    slug: "general",
    label: "General",
    icon: SlidersHorizontal,
    group: "workspace",
    keywords: "defaults permissions session timeout update version runtime socket",
  },
  {
    slug: "appearance",
    label: "Appearance",
    icon: Palette,
    group: "workspace",
    keywords: "wallpaper theme desktop dock icons",
  },
  {
    slug: "layouts",
    label: "Layouts",
    icon: PanelTop,
    group: "workspace",
    keywords: "window manager desktops profiles split stack geometry snap focus",
  },
  {
    slug: "providers",
    label: "Providers",
    icon: Cpu,
    group: "workspace",
    keywords: "models catalog auth claude codex openclaw hermes sign-in",
  },
  {
    slug: "memory",
    label: "Memory",
    icon: Brain,
    group: "runtime",
    keywords: "recall dream ledger persistence daily logs extractor",
  },
  {
    slug: "roles",
    label: "Roles",
    icon: Route,
    group: "runtime",
    keywords:
      "background agents coordinator dream checkpoint auto-title memory controller routing model provider fallback",
  },
  {
    slug: "skills",
    label: "Skills",
    icon: Wrench,
    group: "runtime",
    keywords: "registry marketplace policy disabled install",
  },
  {
    slug: "automation",
    label: "Automation",
    icon: Zap,
    group: "runtime",
    keywords: "jobs triggers scheduler engine timezone cron",
  },
  {
    slug: "network",
    label: "Network",
    icon: Network,
    group: "runtime",
    keywords: "peers channels listener delivery embedded port",
  },
  {
    slug: "observability",
    label: "Observability",
    icon: Activity,
    group: "system",
    keywords: "capture transcripts storage support bundle logs retention",
  },
  {
    slug: "hooks",
    label: "Hooks",
    icon: Webhook,
    group: "system",
    keywords: "lifecycle notifications presets events",
  },
  {
    slug: "extensions",
    label: "Extensions",
    icon: Puzzle,
    group: "system",
    keywords: "policy registry unverified trust",
  },
] as const;

export const SETTINGS_SECTION_GROUPS: ReadonlyArray<{
  id: SettingsSectionGroup;
  label: string;
}> = [
  { id: "workspace", label: "Workspace" },
  { id: "runtime", label: "Runtime" },
  { id: "system", label: "System" },
];

export const SETTINGS_SECTION_SLUGS: readonly SettingsSectionSlug[] = SETTINGS_SECTIONS.map(
  section => section.slug
);

export function settingsSectionPath(slug: SettingsSectionSlug): string {
  return `${SETTINGS_ROOT_PATH}/${slug}`;
}

export function findSettingsSection(
  slug: string | undefined | null
): SettingsSectionDescriptor | undefined {
  if (!slug) {
    return undefined;
  }

  return SETTINGS_SECTIONS.find(section => section.slug === slug);
}

export function filterSettingsSections(query: string): readonly SettingsSectionDescriptor[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return SETTINGS_SECTIONS;
  return SETTINGS_SECTIONS.filter(section =>
    `${section.label} ${section.keywords}`.toLowerCase().includes(normalized)
  );
}
