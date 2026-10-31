import { ArrowDownUp, Rows3, type LucideIcon } from "lucide-react";
import type * as React from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Icon,
  cn,
} from "@compozy/ui";

import {
  SESSION_LIST_SORTS,
  type SessionListGroupBy,
  type SessionListSort,
} from "../lib/session-list-preferences";

const SORT_LABELS: Record<SessionListSort, string> = {
  created: "Created",
  name: "Name",
  last_activity: "Last activity",
  attention: "Attention first",
  manual: "Manual",
};

const GROUP_BY_LABELS: Record<SessionListGroupBy, string> = {
  none: "None",
  project: "Project",
  profile: "Profile",
};

interface OrderMenuProps<T extends string> {
  label: string;
  icon: LucideIcon;
  value: T;
  options: readonly T[];
  labels: Record<T, string>;
  onValueChange: (value: T) => void;
  testId: string;
  /** Marks the trigger as set: a non-default choice reads at a glance. */
  active?: boolean;
  className?: string;
  /** Options of the chosen value, below the choices. */
  children?: React.ReactNode;
}

/** One icon-only radio menu: the trigger names its menu, the items its choices. */
function OrderMenu<T extends string>({
  label,
  icon,
  value,
  options,
  labels,
  onValueChange,
  testId,
  active = false,
  className,
  children,
}: OrderMenuProps<T>) {
  return (
    <DropdownMenu>
      {/* No tooltip: focus returns here when the menu closes, and an open tooltip would take
          the next Escape that should dismiss the surrounding dialog. The name says it all. */}
      <DropdownMenuTrigger
        aria-label={`${label}: ${labels[value]}`}
        data-testid={`${testId}-trigger`}
        render={
          <Button
            className={cn("shrink-0", active && "bg-surface-2 text-fg", className)}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        <Icon as={icon} size="sm" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-44">
        <DropdownMenuGroup>
          <DropdownMenuLabel>{label}</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={value} onValueChange={next => onValueChange(next as T)}>
            {options.map(option => (
              <DropdownMenuRadioItem
                data-testid={`${testId}-${option}`}
                key={option}
                value={option}
              >
                {labels[option]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export interface SessionSortMenuProps {
  value: SessionListSort;
  onValueChange: (sort: SessionListSort) => void;
  /** Defaults to every list sort; the board leaves out `manual`. */
  options?: readonly SessionListSort[];
  testId: string;
  className?: string;
}

/** How sessions are ordered — inside each list group or each board column. */
export function SessionSortMenu({
  options = SESSION_LIST_SORTS,
  ...props
}: SessionSortMenuProps): React.ReactElement {
  return (
    <OrderMenu
      {...props}
      icon={ArrowDownUp}
      label="Sort by"
      labels={SORT_LABELS}
      options={options}
    />
  );
}

export interface SessionGroupByMenuProps {
  value: SessionListGroupBy;
  onValueChange: (groupBy: SessionListGroupBy) => void;
  /** Groupings available now; with only `none` there is nothing to offer and no control renders. */
  options: readonly SessionListGroupBy[];
  /**
   * Project grouping's "Show empty projects" (#1085). Offered only while grouped
   * by project, and only where a handler is given.
   */
  showEmptyProjects?: boolean;
  onShowEmptyProjectsChange?: (show: boolean) => void;
  testId: string;
  className?: string;
}

/** Splits sessions by project or profile: list groups, or board swimlanes. */
export function SessionGroupByMenu({
  options,
  value,
  showEmptyProjects = false,
  onShowEmptyProjectsChange,
  ...props
}: SessionGroupByMenuProps) {
  if (options.length < 2) return null;
  return (
    <OrderMenu
      {...props}
      active={value !== "none"}
      icon={Rows3}
      label="Group by"
      labels={GROUP_BY_LABELS}
      options={options}
      value={value}
    >
      {value === "project" && onShowEmptyProjectsChange ? (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuCheckboxItem
            checked={showEmptyProjects}
            className="whitespace-nowrap"
            data-testid={`${props.testId}-show-empty-projects`}
            onCheckedChange={checked => onShowEmptyProjectsChange(checked === true)}
          >
            Show empty projects
          </DropdownMenuCheckboxItem>
        </>
      ) : null}
    </OrderMenu>
  );
}
