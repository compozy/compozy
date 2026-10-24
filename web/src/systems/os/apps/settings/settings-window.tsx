import {
  Suspense,
  ViewTransition,
  lazy,
  useDeferredValue,
  type ComponentType,
  type LazyExoticComponent,
} from "react";

import { VIEW_TRANSITION_CLASS } from "@compozy/ui";
import { shallowEqual } from "@xstate/store";

import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";
import { useDesktop } from "../../hooks/use-desktop";
import { useOsShell } from "../../hooks/use-os-shell";
import { SettingsWindowNav } from "./settings-window-nav";
import { useProfileFlowIntent } from "./use-profile-flow-intent";
import { SettingsPageState, settingsSectionSlugFromPathname } from "@/systems/settings";
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
  "provider-usage": () =>
    import("@/routes/_app/settings/-provider-usage-settings-page").then(m => ({
      default: m.ProviderUsageSettingsPage,
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

/** Warms a section chunk (link hover/focus) so the switch has no wait. */
function preloadSettingsSection(slug: string): void {
  if (!(slug in SECTION_LOADERS)) return;
  const load: () => Promise<{ default: SectionComponent }> =
    SECTION_LOADERS[slug as MappedSectionSlug];
  void load().catch(() => undefined);
}

const SECTION_PAGES = Object.fromEntries(
  (Object.keys(SECTION_LOADERS) as MappedSectionSlug[]).map(slug => {
    const load: () => Promise<{ default: SectionComponent }> = SECTION_LOADERS[slug];
    return [slug, lazy(load)];
  })
) as Record<MappedSectionSlug, LazyExoticComponent<SectionComponent>>;

const DEFAULT_SETTINGS_ROUTE = { pathname: "/settings", search: {} } as const;

function focusCommandFromSearch(search: Record<string, unknown>): string | undefined {
  const command = search.command;
  if (typeof command !== "string") return undefined;
  const normalized = command.trim();
  return normalized === "" ? undefined : normalized;
}

/**
 * The section page area. The window's route lives in an external store, whose updates always
 * render urgently and so never animate; the deferred slug is the one React can move in a
 * transition. The nav follows the route at once, while the page keeps showing the previous section
 * (never a loading frame) until the next one's code is ready, then cross-fades to it. The boundary
 * is keyed by section and listens for enter and exit only: an `update` boundary here would also
 * fade the whole page for every transition-lane change inside it (a dialog opening, a deferred
 * value settling), which is a large snapshot for no swap.
 */
function SettingsSectionOutlet({
  activeSlug,
  search,
}: {
  activeSlug: MappedSectionSlug;
  search: Record<string, unknown>;
}) {
  const shownSlug = useDeferredValue(activeSlug);
  const SectionPage = SECTION_PAGES[shownSlug];
  const focusCommandId = shownSlug === "layouts" ? focusCommandFromSearch(search) : undefined;

  return (
    <div
      aria-busy={activeSlug === shownSlug ? undefined : true}
      className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-testid="settings-shell-outlet"
    >
      <Suspense fallback={<SettingsPageState slug={shownSlug} state="loading" />}>
        <ViewTransition
          key={shownSlug}
          default="none"
          enter={VIEW_TRANSITION_CLASS.fade}
          exit={VIEW_TRANSITION_CLASS.fade}
        >
          <SectionPage focusCommandId={focusCommandId} />
        </ViewTransition>
      </Suspense>
    </div>
  );
}

/**
 * The settings window: section nav + the active section page, driven by the
 * window's WM location (not router matches) so an unfocused settings window
 * keeps showing its own section (ADR-002 rule 6).
 */
export function SettingsWindow({ windowId }: { windowId: string }) {
  const { route, searchEnabled } = useDesktop(state => {
    const win = state.windows[windowId];
    return {
      route: win?.route ?? DEFAULT_SETTINGS_ROUTE,
      searchEnabled:
        !!win &&
        state.focusedId === windowId &&
        win.desktopId === state.activeDesktopId &&
        !win.minimized &&
        win.stackActive,
    };
  }, shallowEqual);
  // Neutral content clicks focus the desktop ancestor, outside this window's
  // bubbling path. Reuse the document shortcut while this window owns focus.
  const searchInputRef = useListingSearchShortcut(searchEnabled);
  const connection = useDaemonConnectionStatus();
  const { coordinator } = useOsShell();
  const activeSlug = settingsSectionSlugFromPathname(route.pathname);
  useProfileFlowIntent(windowId, activeSlug === "profiles" ? route : undefined);

  // The container declaration lives on the outer wrapper: an element cannot
  // resolve a container query against itself, so the flex switch sits one
  // level down where the Settings takeover container query reads this wrapper's inline size.
  return (
    <div className="@container flex min-h-0 flex-1 flex-col overflow-hidden">
      {/* Section switches are sibling navigation: the outlet's <ViewTransition> owns their
          animation and the nav rail stays put, so the window-manager drill must not also
          snapshot and slide the whole body (tokens-runtime.css). History is unchanged. */}
      <div
        className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row"
        data-owns-view-transition=""
        data-testid="settings-shell"
      >
        <SettingsWindowNav
          activeSlug={activeSlug}
          connection={connection}
          onNavigate={next => coordinator.userNavigate(next)}
          onPreload={preloadSettingsSection}
          searchInputRef={searchInputRef}
        />
        <SettingsSectionOutlet activeSlug={activeSlug} search={route.search} />
      </div>
    </div>
  );
}
