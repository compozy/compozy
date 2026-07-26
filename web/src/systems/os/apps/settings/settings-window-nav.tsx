import { Link } from "@tanstack/react-router";
import { useState, type ComponentProps, type Ref } from "react";

import { cn, ConnectionIndicator, SearchInput, type ConnectionStatus } from "@agh/ui";

import {
  filterSettingsSections,
  SETTINGS_SECTION_GROUPS,
  settingsSectionPath,
  type SettingsSectionDescriptor,
} from "@/systems/settings";
import { settingsConnectionLabel } from "./settings-connection-label";

export interface SettingsWindowNavProps extends Omit<ComponentProps<"nav">, "children"> {
  activeSlug: string;
  connection: ConnectionStatus;
  searchInputRef?: Ref<HTMLInputElement>;
}

/** Window-owned Settings navigation for compact and desktop layouts. */
export function SettingsWindowNav({
  activeSlug,
  connection,
  searchInputRef,
  className,
  ...navProps
}: SettingsWindowNavProps) {
  const [query, setQuery] = useState("");
  const visible = filterSettingsSections(query);

  return (
    <nav
      aria-label="Settings navigation"
      className={cn(
        "flex w-full shrink-0 flex-row items-center gap-1 overflow-x-auto border-b border-line-soft bg-canvas-soft px-2 py-1.5",
        "@min-settings-takeover:w-settings-nav @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0 @min-settings-takeover:overflow-y-auto @min-settings-takeover:border-r @min-settings-takeover:border-b-0 @min-settings-takeover:px-2.5 @min-settings-takeover:py-0",
        className
      )}
      data-testid="settings-section-nav"
      {...navProps}
    >
      <div className="hidden @min-settings-takeover:block">
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
      </div>

      <div className="flex flex-row items-center gap-1 @min-settings-takeover:flex-1 @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0 @min-settings-takeover:pb-2">
        {SETTINGS_SECTION_GROUPS.map(group => {
          const sections = visible.filter(section => section.group === group.id);
          if (sections.length === 0) return null;
          return (
            <div
              className="flex flex-row items-center gap-1 @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0"
              key={group.id}
            >
              <span className="eyebrow hidden px-2 pt-3 pb-1 text-faint @min-settings-takeover:block">
                {group.label}
              </span>
              {sections.map(section => (
                <SettingsSectionLink
                  isActive={section.slug === activeSlug}
                  key={section.slug}
                  section={section}
                />
              ))}
            </div>
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
      </div>

      <div
        className="hidden items-center gap-2 border-t border-line-soft px-2 py-2.5 @min-settings-takeover:flex"
        data-testid="settings-daemon-status"
      >
        <ConnectionIndicator label={settingsConnectionLabel(connection)} status={connection} />
      </div>
    </nav>
  );
}

interface SettingsSectionLinkProps extends Omit<ComponentProps<typeof Link>, "children" | "to"> {
  section: SettingsSectionDescriptor;
  isActive: boolean;
}

function SettingsSectionLink({
  section,
  isActive,
  className,
  ...linkProps
}: SettingsSectionLinkProps) {
  const Icon = section.icon;

  return (
    <Link
      {...linkProps}
      aria-current={isActive ? "page" : undefined}
      className={cn(
        "flex h-11 shrink-0 items-center gap-2.5 rounded-md px-2 text-ws-name font-medium @min-settings-takeover:h-8",
        "transition-colors duration-base focus-visible:outline-none focus-visible:shadow-focus-ring",
        isActive
          ? "bg-accent-tint text-accent-strong"
          : "text-muted hover:bg-row-hover hover:text-fg",
        className
      )}
      data-active={isActive ? "true" : "false"}
      data-testid={`settings-section-${section.slug}`}
      to={settingsSectionPath(section.slug)}
    >
      <Icon
        aria-hidden="true"
        className={cn("size-4 shrink-0", isActive ? "text-accent-strong" : "text-subtle")}
      />
      <span className="whitespace-nowrap @min-settings-takeover:truncate" title={section.label}>
        {section.label}
      </span>
    </Link>
  );
}
