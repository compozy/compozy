import {
  Suspense,
  lazy,
  useRef,
  type ComponentType,
  type KeyboardEvent,
  type LazyExoticComponent,
} from "react";

import { Spinner } from "@agh/ui";

import { SETTINGS_SECTIONS } from "@/systems/settings";
import { useDaemonConnectionStatus } from "@/systems/status";

import { useDesktop } from "../../hooks/use-desktop";
import { SettingsWindowNav } from "./settings-window-nav";

/**
 * Section pages stay in their route-colocated modules (the views are rehosted
 * unchanged); each loads on demand so the settings window ships no section
 * code it is not showing. Keys mirror the nav's `SETTINGS_SECTIONS` slugs.
 */
const SECTION_PAGES = {
  general: lazy(() =>
    import("@/routes/_app/settings/-general-settings-page").then(m => ({
      default: m.GeneralSettingsPage,
    }))
  ),
  appearance: lazy(() =>
    import("./appearance-settings-pane").then(m => ({
      default: m.AppearanceSettingsPane,
    }))
  ),
  layouts: lazy(() =>
    import("@/routes/_app/settings/-layouts-settings-page").then(m => ({
      default: m.LayoutsSettingsPage,
    }))
  ),
  providers: lazy(() =>
    import("@/routes/_app/settings/-providers-settings-page").then(m => ({
      default: m.ProvidersSettingsPage,
    }))
  ),
  memory: lazy(() =>
    import("@/routes/_app/settings/-memory-settings-page").then(m => ({
      default: m.MemorySettingsPage,
    }))
  ),
  roles: lazy(() =>
    import("@/routes/_app/settings/-roles-settings-page").then(m => ({
      default: m.RolesSettingsPage,
    }))
  ),
  skills: lazy(() =>
    import("@/routes/_app/settings/-skills-settings-page").then(m => ({
      default: m.SkillsSettingsPage,
    }))
  ),
  automation: lazy(() =>
    import("@/routes/_app/settings/-automation-settings-page").then(m => ({
      default: m.AutomationSettingsPage,
    }))
  ),
  network: lazy(() =>
    import("@/routes/_app/settings/-network-settings-page").then(m => ({
      default: m.NetworkSettingsPage,
    }))
  ),
  observability: lazy(() =>
    import("@/routes/_app/settings/-observability-settings-page").then(m => ({
      default: m.ObservabilitySettingsPage,
    }))
  ),
  hooks: lazy(() =>
    import("@/routes/_app/settings/-hooks-settings-page").then(m => ({
      default: m.HooksSettingsPage,
    }))
  ),
  extensions: lazy(() =>
    import("@/routes/_app/settings/-extensions-settings-page").then(m => ({
      default: m.ExtensionsSettingsPage,
    }))
  ),
} satisfies Partial<Record<string, LazyExoticComponent<ComponentType>>>;

type MappedSectionSlug = keyof typeof SECTION_PAGES;

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

export function SettingsWindow({ windowId }: { windowId: string }) {
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? "/settings");
  const searchInputRef = useRef<HTMLInputElement>(null);
  const connection = useDaemonConnectionStatus();
  const activeSlug = sectionSlugFromPathname(pathname);
  const SectionPage = SECTION_PAGES[activeSlug];

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
    <div className="@container flex min-h-full flex-col" onKeyDown={handleKeyDown}>
      <div
        className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row"
        data-testid="settings-shell"
      >
        <SettingsWindowNav
          activeSlug={activeSlug}
          connection={connection}
          searchInputRef={searchInputRef}
        />
        <div className="relative flex min-w-0 flex-1 flex-col" data-testid="settings-shell-outlet">
          <Suspense
            fallback={
              <div className="flex flex-1 items-center justify-center py-12">
                <Spinner className="size-4 text-subtle" />
              </div>
            }
          >
            <SectionPage />
          </Suspense>
        </div>
      </div>
    </div>
  );
}
