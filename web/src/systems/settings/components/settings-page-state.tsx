import { AlertCircle } from "lucide-react";

import { Button, Empty, SkeletonRegion } from "@compozy/ui";

import { findSettingsSection } from "../lib/sections";
import type { SettingsSectionSlug } from "../types";
import { SettingsSectionSkeleton } from "./settings-section-skeletons";

export interface SettingsPageStateProps {
  slug: SettingsSectionSlug;
  state: "loading" | "error";
  /** Raw failure; shown only behind the "Details" fold, never as the headline. */
  error?: unknown;
  onRetry?: () => void;
  /** Overrides the section label used in the error title ("Couldn't load <label>"). */
  sectionLabel?: string;
}

function errorCause(error: unknown): string | undefined {
  if (error instanceof Error) return error.message || undefined;
  if (typeof error === "string") return error || undefined;
  return undefined;
}

/**
 * Whole-page loading or load-failure state for a settings section. Loading is the section's own
 * content-shaped skeleton under one status region; keeps the `settings-page-<slug>-loading` /
 * `-error` test ids every page exposes.
 */
export function SettingsPageState({
  slug,
  state,
  error,
  onRetry,
  sectionLabel,
}: SettingsPageStateProps) {
  const label = sectionLabel ?? findSettingsSection(slug)?.label ?? "this section";

  if (state === "loading") {
    return (
      <SkeletonRegion
        className="flex min-h-0 flex-1 flex-col overflow-hidden"
        data-testid={`settings-page-${slug}-loading`}
        label={`Loading ${label}`}
      >
        <SettingsSectionSkeleton slug={slug} />
      </SkeletonRegion>
    );
  }

  return (
    <div
      className="flex flex-1 items-center justify-center p-6"
      data-testid={`settings-page-${slug}-error`}
    >
      <Empty
        action={
          onRetry ? (
            <Button onClick={onRetry} size="sm" type="button" variant="secondary">
              Try again
            </Button>
          ) : undefined
        }
        cause={errorCause(error)}
        description="Check that CompozyOS is running, then try again."
        icon={AlertCircle}
        title={`Couldn't load ${label}`}
        titleAs="h3"
      />
    </div>
  );
}
