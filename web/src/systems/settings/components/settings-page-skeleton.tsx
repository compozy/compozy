import type { ReactNode } from "react";

import { Skeleton, SkeletonText } from "@compozy/ui";

import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";
import { SettingsPageScaffold, type SettingsPageWidth } from "./settings-page-frame";

// Loading shapes for settings pages. They are built from the real scaffold, `SettingsGroup` and
// `SettingRow`, so margins, panel frames, row heights and line boxes come from the same classes the
// loaded page uses; only the leaves (labels, descriptions, controls) are placeholders. The caller
// wraps the whole page in one `SkeletonRegion`, which owns the loading semantics.

/** The control a row carries, sized like the real one so the row does not grow when it arrives. */
export type SettingControlSkeleton =
  | "button"
  | "hinted"
  | "input"
  | "none"
  | "segmented"
  | "select"
  | "selector"
  | "switch"
  | "value";

const CONTROL_CLASS = {
  button: "h-button-sm w-20 rounded-pill",
  hinted: "h-11.75 w-52 rounded-md",
  input: "h-input w-52 rounded-md",
  segmented: "h-control-compact w-52 rounded-pill",
  select: "h-control-compact w-36 rounded-md",
  selector: "h-button-lg w-56 rounded-pill",
  switch: "h-switch-default w-switch-default rounded-pill",
} as const satisfies Record<Exclude<SettingControlSkeleton, "none" | "value">, string>;

const LABEL_WIDTHS = ["w-40", "w-32", "w-48", "w-36"] as const;

export interface SettingRowSkeletonSpec {
  /** Stable key for the placeholder row; it names the position, never copy. */
  id: string;
  /** Lines of consequence text under the label, as many as the real row wraps to. None by default. */
  description?: number;
  control?: SettingControlSkeleton;
}

function SettingControlPlaceholder({ control }: { control: SettingControlSkeleton }) {
  if (control === "none") return null;
  if (control === "value") return <SkeletonText className="w-24 text-small-body" />;
  return <Skeleton className={CONTROL_CLASS[control]} />;
}

export interface SettingRowSkeletonProps extends Omit<SettingRowSkeletonSpec, "id"> {
  /** Width utility for the label placeholder. */
  labelClassName?: string;
}

/** One `SettingRow` whose label, description and control are placeholders. */
export function SettingRowSkeleton({
  description = 0,
  control = "switch",
  labelClassName = "w-40",
}: SettingRowSkeletonProps) {
  return (
    <SettingRow
      control={<SettingControlPlaceholder control={control} />}
      description={description > 0 ? <SkeletonText lines={description} /> : undefined}
      label={<SkeletonText className={labelClassName} />}
    />
  );
}

export interface SettingsGroupSkeletonProps {
  /** The rows of the group's panel, in order. */
  rows?: readonly SettingRowSkeletonSpec[];
  /** Lines of consequence text under the group title; none by default. */
  description?: number;
  /** Adds a quiet header action (a small button) at the end of the title row. */
  action?: boolean;
  /** Drops the panel frame, for groups whose content frames itself. */
  bare?: boolean;
  className?: string;
  /** Extra content after the rows, inside the panel. */
  children?: ReactNode;
}

/** A titled `SettingsGroup` with placeholder rows. */
export function SettingsGroupSkeleton({
  rows = [],
  description = 0,
  action = false,
  bare,
  className,
  children,
}: SettingsGroupSkeletonProps) {
  return (
    <SettingsGroup
      action={action ? <Skeleton className="h-button-sm w-24 rounded-pill" /> : undefined}
      bare={bare}
      className={className}
      description={description > 0 ? <SkeletonText lines={description} /> : undefined}
      title={<SkeletonText className="w-28" />}
    >
      {rows.map((row, index) => (
        <SettingRowSkeleton
          control={row.control}
          description={row.description}
          key={row.id}
          labelClassName={LABEL_WIDTHS[index % LABEL_WIDTHS.length]}
        />
      ))}
      {children}
    </SettingsGroup>
  );
}

export interface SettingsPageSkeletonProps {
  /** Names the scaffold's test ids. */
  testId: string;
  width?: SettingsPageWidth;
  /** Lines in the subhead band, for pages whose loaded frame has a description or meta. None by default. */
  subhead?: number;
  className?: string;
  children: ReactNode;
}

/**
 * The scaffold of a loading settings page: same scroll body, column width and subhead band as
 * `SettingsPageFrame`, hidden from assistive technology. Put it inside the page's single
 * `SkeletonRegion`.
 */
export function SettingsPageSkeleton({
  testId,
  width,
  subhead = 0,
  className,
  children,
}: SettingsPageSkeletonProps) {
  return (
    <SettingsPageScaffold
      aria-hidden="true"
      className={className}
      subhead={
        subhead > 0 ? <SkeletonText className="w-96 max-w-full" lines={subhead} /> : undefined
      }
      testId={testId}
      width={width}
    >
      {children}
    </SettingsPageScaffold>
  );
}
