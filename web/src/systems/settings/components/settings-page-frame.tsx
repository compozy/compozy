import type { ComponentProps, ReactNode } from "react";

import { cn } from "@compozy/ui";

import type { SettingsRestartViewState } from "../lib/restart-presentation";
import type { SettingsSectionSlug } from "../types";
import { SettingsRestartNotice } from "./settings-restart-notice";

export interface SettingsPageFrameProps {
  slug: SettingsSectionSlug;
  /**
   * Visible consequence line only — inherited defaults, irreversible apply,
   * or what this save will do. Restating the page title belongs in `help`.
   */
  description?: ReactNode;
  /** Quiet dotsep meta entries after the description (live counts, freshness). */
  meta?: ReadonlyArray<{ key: string; content: ReactNode }>;
  restart?: SettingsRestartViewState;
  /**
   * Column width: forms at 768px, listings (Providers) at 960px, and the layout
   * canvas at 1040px, where a stage plus its inspector needs the extra room.
   */
  width?: SettingsPageWidth;
  /** Floating save bar (or null for per-item / immediate pages). */
  saveBar?: ReactNode;
  children: ReactNode;
}

export type SettingsPageWidth = "form" | "wide" | "canvas";

const WIDTH_CLASS: Record<SettingsPageWidth, string> = {
  form: "max-w-settings-page-form",
  wide: "max-w-settings-page-wide",
  canvas: "max-w-settings-page-canvas",
};

export interface SettingsPageScaffoldProps extends Omit<ComponentProps<"div">, "children"> {
  /** Names the scaffold for tests: it carries this id, and `<testId>-body` and `<testId>-subhead` follow. */
  testId: string;
  width?: SettingsPageWidth;
  /** The quiet band under the page head (consequence line and meta). Omit it for pages without one. */
  subhead?: ReactNode;
  children: ReactNode;
}

/**
 * The geometry every settings page shares: the scrolling body, the centered column at the page's
 * width, and the subhead band. `SettingsPageFrame` fills it with the loaded page and
 * `SettingsPageSkeleton` with the loading shape, so the two cannot drift apart.
 */
export function SettingsPageScaffold({
  testId,
  width = "form",
  subhead,
  className,
  children,
  ...props
}: SettingsPageScaffoldProps) {
  return (
    <div
      className={cn("flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      data-testid={testId}
      {...props}
    >
      <div className="min-h-0 flex-1 overflow-y-auto" data-testid={`${testId}-body`}>
        <div
          className={cn(
            "mx-auto flex w-full flex-col gap-6 px-6 pt-5 pb-settings-page-bottom",
            WIDTH_CLASS[width]
          )}
        >
          {subhead ? (
            <div
              className="flex min-w-0 flex-wrap items-center gap-2 border-b border-line pb-3.5 text-form-label text-subtle"
              data-testid={`${testId}-subhead`}
            >
              {subhead}
            </div>
          ) : null}
          {children}
        </div>
      </div>
    </div>
  );
}

/**
 * Settings page scaffold (design system §04): centered column (768px forms /
 * 960px listings), optional consequence line + quiet meta, optional restart
 * notice, groups, and the floating save bar inside the scroll region.
 */
export function SettingsPageFrame({
  slug,
  description,
  meta,
  restart,
  width,
  saveBar,
  children,
}: SettingsPageFrameProps) {
  const hasSubhead = Boolean(description || (meta && meta.length > 0));

  return (
    <SettingsPageScaffold
      subhead={
        hasSubhead ? (
          <>
            {description ? (
              <span className="max-w-settings-page-description">{description}</span>
            ) : null}
            {meta?.map((entry, index) => (
              <span className="inline-flex items-center gap-2" key={entry.key}>
                {description || index > 0 ? (
                  <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                ) : null}
                {entry.content}
              </span>
            ))}
          </>
        ) : undefined
      }
      testId={`settings-page-${slug}`}
      width={width}
    >
      {restart ? <SettingsRestartNotice restart={restart} slug={slug} /> : null}
      {children}
      {saveBar}
    </SettingsPageScaffold>
  );
}
