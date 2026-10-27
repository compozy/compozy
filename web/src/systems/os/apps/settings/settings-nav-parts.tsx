import type { ComponentProps, ReactNode } from "react";

import { cn } from "@compozy/ui";

import type { SettingsSectionDescriptor } from "@/systems/settings";

// The frame of the Settings navigation, shared by the live nav and by the window's loading
// fallback so the two hold the same geometry: a horizontal strip in a narrow window and a
// `w-settings-nav` rail once the window passes the takeover width.

export interface SettingsNavFrameProps extends Omit<ComponentProps<"nav">, "children"> {
  /** The search field; it only shows in the rail. */
  search: ReactNode;
  /** The daemon status line at the foot of the rail. */
  footer: ReactNode;
  /** The section groups. */
  children: ReactNode;
}

export function SettingsNavFrame({
  search,
  footer,
  children,
  className,
  ...navProps
}: SettingsNavFrameProps) {
  return (
    <nav
      aria-label="Settings navigation"
      className={cn(
        "flex w-full shrink-0 flex-row items-center gap-1 overflow-x-auto border-b border-line-soft bg-sunken px-2 py-1.5",
        "@min-settings-takeover:w-settings-nav @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0 @min-settings-takeover:overflow-y-auto @min-settings-takeover:border-r @min-settings-takeover:border-b-0 @min-settings-takeover:px-2.5 @min-settings-takeover:py-0",
        className
      )}
      data-testid="settings-section-nav"
      {...navProps}
    >
      <div className="hidden @min-settings-takeover:block">{search}</div>

      <div className="flex flex-row items-center gap-1 @min-settings-takeover:flex-1 @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0 @min-settings-takeover:pb-2">
        {children}
      </div>

      <div
        className="hidden items-center gap-2 border-t border-line-soft px-2 py-2.5 @min-settings-takeover:flex"
        data-testid="settings-daemon-status"
      >
        {footer}
      </div>
    </nav>
  );
}

/** One group of sections under its eyebrow, which only shows in the rail. */
export function SettingsNavGroup({ label, children }: { label: ReactNode; children: ReactNode }) {
  return (
    <div className="flex flex-row items-center gap-1 @min-settings-takeover:flex-col @min-settings-takeover:items-stretch @min-settings-takeover:gap-0">
      <span className="eyebrow hidden px-2 pt-3 pb-1 text-faint @min-settings-takeover:block">
        {label}
      </span>
      {children}
    </div>
  );
}

/** A section's glyph and name. */
export function SettingsNavItemContent({
  section,
  isActive,
}: {
  section: SettingsSectionDescriptor;
  isActive: boolean;
}) {
  const Icon = section.icon;
  return (
    <>
      <Icon
        aria-hidden="true"
        className={cn("size-4 shrink-0", isActive ? "text-fg" : "text-subtle")}
      />
      <span className="whitespace-nowrap @min-settings-takeover:truncate" title={section.label}>
        {section.label}
      </span>
    </>
  );
}
