import {
  Suspense,
  lazy,
  useRef,
  type ComponentType,
  type KeyboardEvent,
  type LazyExoticComponent,
} from "react";

import { Spinner, viewTransitionName } from "@compozy/ui";

import { useDesktop } from "../../hooks/use-desktop";
import { SettingsWindowNav } from "./settings-window-nav";
import { useProfileFlowIntent } from "./use-profile-flow-intent";
import { useSettingsNavigation } from "./use-settings-navigation";
import { SETTINGS_SECTIONS } from "@/systems/settings";
import { useDaemonConnectionStatus } from "@/systems/status";

export interface SettingsSectionPageProps {
  focusCommandId?: string;
}

type SectionComponent = ComponentType<SettingsSectionPageProps>;

/**
 * Section pages stay in their route-colocated modules (the views are rehosted
 * unchanged); each loads on demand so the settings window ships no section
 * code it is not showing. Keys mirror the nav's `SETTINGS_SECTIONS` slugs.
 */
const SECTION_LOADERS = {
  general: () =>
    import("@/routes/_app/settings/-general-settings-page").then(m => ({
      default: m.GeneralSettingsPage,
    })),
  terminal: () =>
    import("@/routes/_app/settings/-terminal-settings-page").then(m => ({
      default: m.TerminalSettingsPage,
    })),
  defaults: () =>
    import("@/routes/_app/settings/-defaults-settings-page").then(m => ({
      default: m.DefaultsSettingsPage,
    })),
  appearance: () =>
    import("./appearance-settings-pane").then(m => ({
      default: m.AppearanceSettingsPane,
    })),
  layouts: () =>
    import("@/routes/_app/settings/-layouts-settings-page").then(m => ({
      default: m.LayoutsSettingsPage,
    })),
  providers: () =>
    import("@/routes/_app/settings/-providers-settings-page").then(m => ({
      default: m.ProvidersSettingsPage,
    })),
  memory: () =>
    import("@/routes/_app/settings/-memory-settings-page").then(m => ({
      default: m.MemorySettingsPage,
    })),
  roles: () =>
    import("@/routes/_app/settings/-roles-settings-page").then(m => ({
      default: m.RolesSettingsPage,
    })),
  skills: () =>
    import("@/routes/_app/settings/-skills-settings-page").then(m => ({
      default: m.SkillsSettingsPage,
    })),
  mcp: () =>
    import("@/routes/_app/settings/-mcp-settings-page").then(m => ({
      default: m.MCPSettingsPage,
    })),
  automation: () =>
    import("@/routes/_app/settings/-automation-settings-page").then(m => ({
      default: m.AutomationSettingsPage,
    })),
  gateway: () =>
    import("@/routes/_app/settings/-gateway-settings-page").then(m => ({
      default: m.GatewaySettingsPage,
    })),
  profiles: () =>
    import("@/routes/_app/settings/-profiles-settings-page").then(m => ({
      default: m.ProfilesSettingsPage,
    })),
  palette: () =>
    import("@/routes/_app/settings/-palette-settings-page").then(m => ({
      default: m.PaletteSettingsPage,
    })),
  attention: () =>
    import("@/routes/_app/settings/-attention-settings-page").then(m => ({
      default: m.AttentionSettingsPage,
    })),
  observability: () =>
    import("@/routes/_app/settings/-observability-settings-page").then(m => ({
      default: m.ObservabilitySettingsPage,
    })),
  hooks: () =>
    import("@/routes/_app/settings/-hooks-settings-page").then(m => ({
      default: m.HooksSettingsPage,
    })),
  extensions: () =>
    import("@/routes/_app/settings/-extensions-settings-page").then(m => ({
      default: m.ExtensionsSettingsPage,
    })),
  marketplace: () =>
    import("@/routes/_app/settings/-marketplace-settings-page").then(m => ({
      default: m.MarketplaceSettingsPage,
    })),
} satisfies Record<string, () => Promise<{ default: SectionComponent }>>;

type MappedSectionSlug = keyof typeof SECTION_LOADERS;

const loadedSections = new Set<MappedSectionSlug>();

function loadSection(slug: MappedSectionSlug): Promise<{ default: SectionComponent }> {
  const load: () => Promise<{ default: SectionComponent }> = SECTION_LOADERS[slug];
  return load().then(module => {
    loadedSections.add(slug);
    return module;
  });
}

/** Warms a section chunk (link hover/focus) so the switch has no spinner frame. */
function preloadSettingsSection(slug: string): void {
  if (!(slug in SECTION_LOADERS)) return;
  void loadSection(slug as MappedSectionSlug).catch(() => undefined);
}

const SECTION_PAGES = Object.fromEntries(
  (Object.keys(SECTION_LOADERS) as MappedSectionSlug[]).map(slug => [
    slug,
    lazy(() => loadSection(slug)),
  ])
) as Record<MappedSectionSlug, LazyExoticComponent<SectionComponent>>;

function sectionSlugFromPathname(pathname: string): MappedSectionSlug {
  const segment = pathname.split("/")[2] ?? "";
  return segment in SECTION_PAGES
    ? (segment as MappedSectionSlug)
    : (SETTINGS_SECTIONS[0].slug as MappedSectionSlug);
}

/**
 * The settings window: section nav + the active section page, driven by the
 * window's WM location (not router matches) so an unfocused settings window
 * keeps showing its own section (ADR-002 rule 6).
 */
const TYPING_TAGS = /^(INPUT|SELECT|TEXTAREA)$/;
const DEFAULT_SETTINGS_ROUTE = { pathname: "/settings", search: {} } as const;

function focusCommandFromSearch(search: Record<string, unknown>): string | undefined {
  const command = search.command;
  if (typeof command !== "string") return undefined;
  const normalized = command.trim();
  return normalized === "" ? undefined : normalized;
}

export function SettingsWindow({ windowId }: { windowId: string }) {
  const route = useDesktop(state => state.windows[windowId]?.route ?? DEFAULT_SETTINGS_ROUTE);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const connection = useDaemonConnectionStatus();
  const activeSlug = sectionSlugFromPathname(route.pathname);
  const navigate = useSettingsNavigation(next => {
    // Unloaded sections switch immediately to avoid snapshotting their spinner.
    const nextSlug = sectionSlugFromPathname(next.pathname);
    return nextSlug !== activeSlug && loadedSections.has(nextSlug);
  });
  const SectionPage = SECTION_PAGES[activeSlug];
  const focusCommandId =
    activeSlug === "layouts" ? focusCommandFromSearch(route.search) : undefined;
  useProfileFlowIntent(windowId, activeSlug === "profiles" ? route : undefined);

  // Window-scoped `/` shortcut: focus the sidebar search unless the user is
  // already typing in a field.
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "/" || event.defaultPrevented) return;
    const target = event.target as HTMLElement | null;
    if (target && (TYPING_TAGS.test(target.tagName) || target.isContentEditable)) return;
    event.preventDefault();
    searchInputRef.current?.focus();
  };

  // The container declaration lives on the outer wrapper: an element cannot
  // resolve a container query against itself, so the flex switch sits one
  // level down where the Settings takeover container query reads this wrapper's inline size.
  return (
    <div
      className="@container flex min-h-0 flex-1 flex-col overflow-hidden"
      onKeyDown={handleKeyDown}
    >
      <div
        className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row"
        data-testid="settings-shell"
      >
        <SettingsWindowNav
          activeSlug={activeSlug}
          connection={connection}
          onNavigate={navigate}
          onPreload={preloadSettingsSection}
          searchInputRef={searchInputRef}
        />
        <div
          className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
          data-testid="settings-shell-outlet"
          style={{ viewTransitionName: viewTransitionName("settings-content", windowId) }}
        >
          <Suspense
            fallback={
              <div className="flex flex-1 items-center justify-center py-12">
                <Spinner className="size-4 text-subtle" />
              </div>
            }
          >
            <SectionPage focusCommandId={focusCommandId} />
          </Suspense>
        </div>
      </div>
    </div>
  );
}
