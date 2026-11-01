import {
  ArrowDownAZ,
  CalendarPlus,
  ChevronsDownUp,
  ChevronsUpDown,
  CircleAlert,
  Clock,
  FolderGit2,
  FolderOpen,
  GripVertical,
  Rows3,
  SlidersHorizontal,
  UserRound,
  type LucideIcon,
} from "@compozy/ui/icons";
import type * as React from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
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

const SORT_ITEMS: Record<SessionListSort, { label: string; icon: LucideIcon }> = {
  created: { label: "Created", icon: CalendarPlus },
  name: { label: "Name", icon: ArrowDownAZ },
  last_activity: { label: "Last activity", icon: Clock },
  attention: { label: "Attention first", icon: CircleAlert },
  manual: { label: "Manual", icon: GripVertical },
};

const GROUP_BY_ITEMS: Record<SessionListGroupBy, { label: string; icon: LucideIcon }> = {
  none: { label: "None", icon: Rows3 },
  project: { label: "Project", icon: FolderGit2 },
  profile: { label: "Profile", icon: UserRound },
};

/** Choices read quieter than the chosen one: the check and the ink mark it together. */
const CHOICE_ITEM =
  "text-muted-foreground *:[svg]:text-subtle focus:*:[svg]:text-fg data-checked:font-medium data-checked:text-fg data-checked:*:[svg]:text-fg";

/** Sections read as blocks: the rule is the line token, not the softer one between rows. */
const SECTION_DIVIDER = "my-1.5 bg-line";

interface ChoiceSectionProps<T extends string> {
  label: string;
  value: T;
  options: readonly T[];
  items: Record<T, { label: string; icon: LucideIcon }>;
  onValueChange: (value: T) => void;
  testId: string;
  children?: React.ReactNode;
}

/** One titled radio section; what belongs to the choice sits under it, in the same section. */
function ChoiceSection<T extends string>({
  label,
  value,
  options,
  items,
  onValueChange,
  testId,
  children,
}: ChoiceSectionProps<T>) {
  return (
    <DropdownMenuGroup>
      <DropdownMenuLabel>{label}</DropdownMenuLabel>
      <DropdownMenuRadioGroup value={value} onValueChange={next => onValueChange(next as T)}>
        {options.map(option => (
          <DropdownMenuRadioItem
            className={CHOICE_ITEM}
            data-testid={`${testId}-${option}`}
            key={option}
            value={option}
          >
            <Icon as={items[option].icon} size="sm" />
            {items[option].label}
          </DropdownMenuRadioItem>
        ))}
      </DropdownMenuRadioGroup>
      {children}
    </DropdownMenuGroup>
  );
}

export interface SessionViewOptionsMenuProps {
  sort: SessionListSort;
  onSortChange: (sort: SessionListSort) => void;
  /** Defaults to every list sort; the board leaves out `manual`. */
  sortOptions?: readonly SessionListSort[];
  groupBy: SessionListGroupBy;
  onGroupByChange: (groupBy: SessionListGroupBy) => void;
  /** Groupings available now; with only `none` the section has nothing to offer. */
  groupByOptions: readonly SessionListGroupBy[];
  /** Project grouping's "Show empty projects" (#1085), shared by the window's List and Kanban. */
  showEmptyProjects: boolean;
  onShowEmptyProjectsChange: (show: boolean) => void;
  allCollapsed: boolean;
  /** Absent where nothing folds: the action is left out. */
  onAllCollapsedChange?: (collapsed: boolean) => void;
  /** Prefix of every item's test id. */
  testId: string;
  className?: string;
}

/**
 * How a window shows its sessions, in one menu: order, grouping and the folds. The List and the
 * Kanban render the same menu over the same window state.
 */
export function SessionViewOptionsMenu({
  sort,
  onSortChange,
  sortOptions = SESSION_LIST_SORTS,
  groupBy,
  onGroupByChange,
  groupByOptions,
  showEmptyProjects,
  onShowEmptyProjectsChange,
  allCollapsed,
  onAllCollapsedChange,
  testId,
  className,
}: SessionViewOptionsMenuProps) {
  const marked = groupBy !== "none" || allCollapsed;
  return (
    <DropdownMenu>
      {/* No tooltip: focus returns here when the menu closes, and an open tooltip would take
          the next Escape that should dismiss the surrounding dialog. The name says it all. */}
      <DropdownMenuTrigger
        aria-label="View options"
        data-testid={`${testId}-view-options-trigger`}
        render={
          <Button
            className={cn("shrink-0", marked && "bg-surface-2 text-fg", className)}
            size="icon-sm"
            type="button"
            variant="quiet"
          />
        }
      >
        <Icon as={SlidersHorizontal} size="sm" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <ChoiceSection
          items={SORT_ITEMS}
          label="Sort by"
          onValueChange={onSortChange}
          options={sortOptions}
          testId={`${testId}-sort`}
          value={sort}
        />
        {groupByOptions.length > 1 ? (
          <>
            <DropdownMenuSeparator className={SECTION_DIVIDER} />
            <ChoiceSection
              items={GROUP_BY_ITEMS}
              label="Group by"
              onValueChange={onGroupByChange}
              options={groupByOptions}
              testId={`${testId}-group-by`}
              value={groupBy}
            >
              {groupBy === "project" ? (
                <DropdownMenuCheckboxItem
                  checked={showEmptyProjects}
                  className={CHOICE_ITEM}
                  data-testid={`${testId}-show-empty-projects`}
                  onCheckedChange={checked => onShowEmptyProjectsChange(checked === true)}
                >
                  <Icon as={FolderOpen} size="sm" />
                  Show empty projects
                </DropdownMenuCheckboxItem>
              ) : null}
            </ChoiceSection>
          </>
        ) : null}
        {onAllCollapsedChange ? (
          <>
            <DropdownMenuSeparator className={SECTION_DIVIDER} />
            <DropdownMenuGroup>
              <DropdownMenuItem
                data-testid={`${testId}-collapse-all`}
                onClick={() => onAllCollapsedChange(!allCollapsed)}
              >
                <Icon
                  as={allCollapsed ? ChevronsUpDown : ChevronsDownUp}
                  className="text-subtle"
                  size="sm"
                />
                {allCollapsed ? "Expand all" : "Collapse all"}
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
