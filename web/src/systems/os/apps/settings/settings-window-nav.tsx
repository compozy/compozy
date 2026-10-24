import { Link } from "@tanstack/react-router";
import { useState, type ComponentProps, type MouseEvent, type Ref } from "react";

import { cn, ConnectionIndicator, SearchInput, type ConnectionStatus } from "@compozy/ui";

import type { OsWindowRoute } from "../../lib/os-types";
import { settingsConnectionLabel } from "./settings-connection-label";
import { SETTINGS_NAV_ITEM_CLASS, settingsNavItemTone } from "./settings-nav-classes";
import { SettingsNavFrame, SettingsNavGroup, SettingsNavItemContent } from "./settings-nav-parts";
import {
  filterSettingsSections,
  SETTINGS_SECTION_GROUPS,
  type SettingsSectionDescriptor,
  settingsSectionPath,
} from "@/systems/settings";

export interface SettingsWindowNavProps extends Omit<ComponentProps<"nav">, "children"> {
  activeSlug: string;
  connection: ConnectionStatus;
  /**
   * Receives a plain click on a section link. Section links are window
   * navigation, not page navigation: the shell writes the URL and steers the
   * window, which a router `<Link>` cannot do when the location already reads
   * the chosen section. Modified clicks keep native link behavior.
   */
  onNavigate: (route: OsWindowRoute) => void;
  /** Warms a section's code on link hover/focus so switching has no loading frame. */
  onPreload?: (slug: string) => void;
  searchInputRef?: Ref<HTMLInputElement>;
}

/** Window-owned Settings navigation for compact and desktop layouts. */
export function SettingsWindowNav({
  activeSlug,
  connection,
  onNavigate,
  onPreload,
  searchInputRef,
  className,
  ...navProps
}: SettingsWindowNavProps) {
  const [query, setQuery] = useState("");
  const visible = filterSettingsSections(query);

  return (
    <SettingsNavFrame
      className={className}
      footer={
        <ConnectionIndicator label={settingsConnectionLabel(connection)} status={connection} />
      }
      search={
        <SearchInput
          aria-label="Search settings"
          autoComplete="off"
          containerClassName="mt-2.5 w-full min-w-0 bg-canvas"
          data-testid="settings-nav-search"
          kbd="/"
          onChange={setQuery}
          placeholder="Search settings"
          ref={searchInputRef}
          value={query}
        />
      }
      {...navProps}
    >
      {SETTINGS_SECTION_GROUPS.map(group => {
        const sections = visible.filter(section => section.group === group.id);
        if (sections.length === 0) return null;
        return (
          <SettingsNavGroup key={group.id} label={group.label}>
            {sections.map(section => (
              <SettingsSectionLink
                isActive={section.slug === activeSlug}
                key={section.slug}
                onNavigate={onNavigate}
                onPreload={onPreload}
                section={section}
              />
            ))}
          </SettingsNavGroup>
        );
      })}
      {visible.length === 0 ? (
        <p
          className="hidden px-2 py-3 text-form-label text-subtle @min-settings-takeover:block"
          data-testid="settings-nav-empty"
        >
          No settings match “{query.trim()}”
        </p>
      ) : null}
    </SettingsNavFrame>
  );
}

interface SettingsSectionLinkProps extends Omit<ComponentProps<typeof Link>, "children" | "to"> {
  section: SettingsSectionDescriptor;
  isActive: boolean;
  onNavigate: (route: OsWindowRoute) => void;
  onPreload?: (slug: string) => void;
}

function isPlainLeftClick(event: MouseEvent<HTMLAnchorElement>): boolean {
  return event.button === 0 && !event.metaKey && !event.altKey && !event.ctrlKey && !event.shiftKey;
}

function SettingsSectionLink({
  section,
  isActive,
  onNavigate,
  onPreload,
  className,
  onClick,
  onFocus,
  onPointerEnter,
  ...linkProps
}: SettingsSectionLinkProps) {
  const handleClick = (event: MouseEvent<HTMLAnchorElement>) => {
    onClick?.(event);
    if (event.defaultPrevented || !isPlainLeftClick(event)) return;
    event.preventDefault();
    onNavigate({ pathname: settingsSectionPath(section.slug), search: {} });
  };

  return (
    <Link
      {...linkProps}
      onClick={handleClick}
      onFocus={event => {
        onFocus?.(event);
        onPreload?.(section.slug);
      }}
      onPointerEnter={event => {
        onPointerEnter?.(event);
        onPreload?.(section.slug);
      }}
      aria-current={isActive ? "page" : undefined}
      className={cn(
        SETTINGS_NAV_ITEM_CLASS,
        "transition-colors duration-base focus-visible:outline-none focus-visible:shadow-focus-ring",
        settingsNavItemTone(isActive),
        !isActive && "hover:bg-surface-2 hover:text-fg",
        className
      )}
      data-active={isActive ? "true" : "false"}
      data-testid={`settings-section-${section.slug}`}
      to={settingsSectionPath(section.slug)}
    >
      <SettingsNavItemContent isActive={isActive} section={section} />
    </Link>
  );
}
