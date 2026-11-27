import { Laptop, Settings, UserRound, type LucideIcon } from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { cn, ConnectionIndicator, SearchInput, type ConnectionStatus } from "@compozy/ui";

import { settingsConnectionLabel } from "@/systems/os/apps/settings/settings-connection-label";
import {
  SETTINGS_NAV_ITEM_CLASS,
  settingsNavItemTone,
} from "@/systems/os/apps/settings/settings-nav-classes";
import {
  SettingsNavFrame,
  SettingsNavGroup,
  SettingsNavItemContent,
} from "@/systems/os/apps/settings/settings-nav-parts";
import {
  SettingsPageFrame,
  type SettingsPageFrameProps,
  type SettingsPageWidth,
} from "@/systems/settings/components/settings-page-frame";
import { SETTINGS_SECTION_GROUPS, SETTINGS_SECTIONS } from "@/systems/settings/lib/sections";
import type { SettingsSectionGroup } from "@/systems/settings/types";

import { DraftShell } from "../always-on-agents-shared/draft-shell";

export type RemoteSettingsSectionSlug = "account" | "computers" | (string & {});

interface RemoteNavSection {
  slug: string;
  label: string;
  icon: LucideIcon;
  group: SettingsSectionGroup;
  keywords: string;
}

const ACCOUNT_SECTION: RemoteNavSection = {
  slug: "account",
  label: "Account",
  icon: UserRound,
  group: "operator",
  keywords: "account sign in plan pro billing upgrade devices trusted admin approvals",
};

const COMPUTERS_SECTION: RemoteNavSection = {
  slug: "computers",
  label: "Computers",
  icon: Laptop,
  group: "system",
  keywords: "computers remote ssh hosted route devices run on load balancing",
};

/**
 * Settings navigation with the Remote Computers spec updates (`_uiux.md` U1, U3):
 * - Adds `Account` at the top of Personal (`operator`)
 * - Replaces `Remote access` (`gateway`) with `Computers` (`computers`) in Advanced (`system`)
 */
const REMOTE_SETTINGS_SECTIONS: readonly RemoteNavSection[] = (() => {
  const mapped: RemoteNavSection[] = [];
  let accountInserted = false;
  for (const section of SETTINGS_SECTIONS) {
    if (section.group === "operator" && !accountInserted) {
      mapped.push(ACCOUNT_SECTION);
      accountInserted = true;
    }
    if (section.slug === "gateway") {
      mapped.push(COMPUTERS_SECTION);
    } else {
      mapped.push(section);
    }
  }
  if (!accountInserted) mapped.push(ACCOUNT_SECTION);
  return mapped;
})();

function filterRemoteSettingsSections(query: string): readonly RemoteNavSection[] {
  const normalized = query.trim().toLowerCase();
  if (!normalized) return REMOTE_SETTINGS_SECTIONS;
  return REMOTE_SETTINGS_SECTIONS.filter(section =>
    `${section.label} ${section.keywords}`.toLowerCase().includes(normalized)
  );
}

export interface RemoteSettingsShellProps {
  /** Active sidebar slug, e.g. `"account"` or `"computers"`. */
  activeSection: RemoteSettingsSectionSlug;
  /** Window topbar leaf title, e.g. `"Account"` or `"Computers"`. */
  title: ReactNode;
  /** Right-aligned header actions inside the Settings window topbar (e.g. `+ Add computer`). */
  headerActions?: ReactNode;
  /** Optional quiet consequence line under the window head. */
  description?: ReactNode;
  /** Optional dot-separated meta items in the page subhead. */
  meta?: SettingsPageFrameProps["meta"];
  /** Centered column width (`"form"` 768px for Account, `"wide"` 960px for Computers). */
  width?: SettingsPageWidth;
  /** Daemon connection status at the foot of the Settings nav rail. */
  connection?: ConnectionStatus;
  /** Dialogs, sheets, or drawers rendered over the full app window. */
  overlay?: ReactNode;
  /** Optional section click callback when a reviewer clicks another nav item. */
  onSelectSection?: (slug: string) => void;
  children: ReactNode;
}

/**
 * Reusable Settings window shell for Remote Computers prototypes (Screens 01, 02, 03, 04):
 * renders the real CompozyOS menubar, dock rail, Settings window chrome, updated nav
 * (`Account` + `Computers`), and `SettingsPageFrame` with full-screen `overlay` support.
 */
export function RemoteSettingsShell({
  activeSection,
  title,
  headerActions,
  description,
  meta,
  width = "wide",
  connection = "connected",
  overlay,
  onSelectSection,
  children,
}: RemoteSettingsShellProps) {
  const [query, setQuery] = useState("");
  const visible = filterRemoteSettingsSections(query);

  return (
    <DraftShell
      app="dashboard"
      glyph={<Settings aria-hidden="true" />}
      overlay={overlay}
      title="Settings"
      topbar={{
        crumb: title,
        actions: headerActions,
      }}
    >
      <div className="@container flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row">
          <SettingsNavFrame
            footer={
              <ConnectionIndicator
                label={settingsConnectionLabel(connection)}
                status={connection}
              />
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
                value={query}
              />
            }
          >
            {SETTINGS_SECTION_GROUPS.map(group => {
              const sections = visible.filter(section => section.group === group.id);
              if (sections.length === 0) return null;
              return (
                <SettingsNavGroup key={group.id} label={group.label}>
                  {sections.map(section => {
                    const isActive = section.slug === activeSection;
                    return (
                      <button
                        aria-current={isActive ? "page" : undefined}
                        className={cn(
                          SETTINGS_NAV_ITEM_CLASS,
                          "cursor-pointer text-left transition-colors duration-base focus-visible:shadow-focus-ring focus-visible:outline-none",
                          settingsNavItemTone(isActive),
                          !isActive && "hover:bg-surface-2 hover:text-fg"
                        )}
                        data-active={isActive ? "true" : "false"}
                        data-testid={`settings-section-${section.slug}`}
                        key={section.slug}
                        onClick={() => onSelectSection?.(section.slug)}
                        type="button"
                      >
                        <SettingsNavItemContent isActive={isActive} section={section} />
                      </button>
                    );
                  })}
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

          <SettingsPageFrame
            description={description}
            meta={meta}
            slug={activeSection}
            width={width}
          >
            {children}
          </SettingsPageFrame>
        </div>
      </div>
    </DraftShell>
  );
}
