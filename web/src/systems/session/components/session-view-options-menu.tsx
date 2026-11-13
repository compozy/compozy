import {
  ArrowDownAZ,
  CalendarPlus,
  CircleAlert,
  Clock,
  FolderGit2,
  FolderOpen,
  GripVertical,
  ListTree,
  Rows3,
  SlidersHorizontal,
  UserRound,
  Zap,
  type LucideIcon,
} from "@compozy/ui/icons";
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
  DEFAULT_SESSION_LIST_PREFERENCES,
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
  /** The board's "Hide child sessions" filter; absent where a surface has no such filter. */
  hideChildren?: boolean;
  onHideChildrenChange?: (hide: boolean) => void;
  /** "Show automation runs" (#1355); absent where a surface has no such filter. */
  showAutomation?: boolean;
  onShowAutomationChange?: (show: boolean) => void;
  /** Prefix of every item's test id. */
  testId: string;
  className?: string;
}

/**
 * How a window shows its sessions, in one menu: order, grouping and filters (the folds have
 * their own toolbar button, `SessionFoldAllButton`). The List and the
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
  hideChildren = true,
  onHideChildrenChange,
  showAutomation = false,
  onShowAutomationChange,
  testId,
  className,
}: SessionViewOptionsMenuProps) {
  // The filter starts on, so showing child sessions is the choice worth marking.
  const showsChildren = onHideChildrenChange !== undefined && !hideChildren;
  const showsAutomation = onShowAutomationChange !== undefined && showAutomation;
  // The default grouping renders where the breadth offers it and flat elsewhere; only a choice
  // that differs from it is worth marking.
  const defaultGroupBy = groupByOptions.includes(DEFAULT_SESSION_LIST_PREFERENCES.groupBy)
    ? DEFAULT_SESSION_LIST_PREFERENCES.groupBy
    : "none";
  const marked = groupBy !== defaultGroupBy || showsChildren || showsAutomation;
  return (
    <DropdownMenu>
      {/* No tooltip: focus returns here when the menu closes, and an open tooltip would take
          the next Escape that should dismiss the surrounding dialog. The name says it all. */}
      <DropdownMenuTrigger
        aria-label="View options"
        data-testid={`${testId}-view-options-trigger`}
        render={
          <Button
            className={cn(
              "shrink-0",
              // A non-default view reads as an active toggle: brand ink, no plate (DESIGN.md §9).
              marked && "text-brand-strong hover:text-brand-strong",
              className
            )}
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
        <FilterSection
          hideChildren={hideChildren}
          onHideChildrenChange={onHideChildrenChange}
          onShowAutomationChange={onShowAutomationChange}
          showAutomation={showAutomation}
          testId={testId}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The Filter section: each item shows only where its surface passes a handler. */
function FilterSection({
  hideChildren,
  onHideChildrenChange,
  showAutomation,
  onShowAutomationChange,
  testId,
}: Pick<
  SessionViewOptionsMenuProps,
  "onHideChildrenChange" | "onShowAutomationChange" | "testId"
> & { hideChildren: boolean; showAutomation: boolean }) {
  if (!onHideChildrenChange && !onShowAutomationChange) return null;
  return (
    <>
      <DropdownMenuSeparator className={SECTION_DIVIDER} />
      <DropdownMenuGroup>
        <DropdownMenuLabel>Filter</DropdownMenuLabel>
        {onHideChildrenChange ? (
          <DropdownMenuCheckboxItem
            checked={hideChildren}
            className={CHOICE_ITEM}
            data-testid={`${testId}-hide-children`}
            onCheckedChange={checked => onHideChildrenChange(checked === true)}
          >
            <Icon as={ListTree} size="sm" />
            Hide child sessions
          </DropdownMenuCheckboxItem>
        ) : null}
        {onShowAutomationChange ? (
          <DropdownMenuCheckboxItem
            checked={showAutomation}
            className={CHOICE_ITEM}
            data-testid={`${testId}-show-automation`}
            onCheckedChange={checked => onShowAutomationChange(checked === true)}
          >
            <Icon as={Zap} size="sm" />
            Show automation runs
          </DropdownMenuCheckboxItem>
        ) : null}
      </DropdownMenuGroup>
    </>
  );
}
