import type { ReactNode } from "react";
import { Wrench } from "@compozy/ui/icons";
import {
  Empty,
  Section,
  Skeleton,
  SkeletonText,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableRowSkeleton,
} from "@compozy/ui";

const DISABLED_LIST_CLASS = "overflow-hidden rounded-lg bg-card shadow-card";
const SKILL_COLUMN_CLASS = "eyebrow text-muted";
const DISABLED_COLUMN_CLASS = "eyebrow w-[1%] text-right text-muted";
const SKELETON_COLUMNS = [{ className: "w-32" }, { align: "end", className: "w-10" }] as const;

interface SettingsDisabledSkillsSectionProps {
  baselineDisabled: readonly string[];
  controls: ReactNode;
  disabled: readonly string[];
  emptyDescription: string;
  emptyTitle: string;
  note: string;
  onToggle: (name: string) => void;
  readOnly?: boolean;
}

export function SettingsDisabledSkillsSection({
  baselineDisabled,
  controls,
  disabled,
  emptyDescription,
  emptyTitle,
  note,
  onToggle,
  readOnly = false,
}: SettingsDisabledSkillsSectionProps) {
  const disabledSkills = new Set(disabled);
  const candidates = Array.from(new Set([...baselineDisabled, ...disabled])).sort();

  return (
    <Section divided label="Disabled skills" note={note} right={controls}>
      {candidates.length === 0 ? (
        <Empty
          icon={Wrench}
          title={emptyTitle}
          description={emptyDescription}
          data-testid="settings-page-skills-disabled-empty"
        />
      ) : (
        <div className={DISABLED_LIST_CLASS} data-testid="settings-page-skills-disabled-list">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className={SKILL_COLUMN_CLASS}>Skill</TableHead>
                <TableHead className={DISABLED_COLUMN_CLASS}>Disabled</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {candidates.map(name => (
                <TableRow key={name} data-testid={`settings-page-skills-disabled-item-${name}`}>
                  <TableCell>
                    <div className="flex min-w-0 items-center gap-2">
                      <Wrench className="size-3.5 text-subtle" />
                      <span className="truncate text-body text-fg">{name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="text-right">
                    <Switch
                      data-testid={`settings-page-skills-disabled-toggle-${name}`}
                      checked={disabledSkills.has(name)}
                      disabled={readOnly}
                      onCheckedChange={() => onToggle(name)}
                      aria-label={`Toggle ${name}`}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </Section>
  );
}

/**
 * The disabled-skills section in skeleton form: the real column headers over placeholder rows, with
 * a placeholder where the title, note and controls go.
 */
export function SettingsDisabledSkillsSkeleton() {
  return (
    <Section
      aria-hidden="true"
      divided
      label={<SkeletonText className="w-32" />}
      note={<SkeletonText className="w-56" />}
      right={<Skeleton className="h-button-sm w-40 rounded-pill" />}
    >
      <div className={DISABLED_LIST_CLASS}>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className={SKILL_COLUMN_CLASS}>Skill</TableHead>
              <TableHead className={DISABLED_COLUMN_CLASS}>Disabled</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRowSkeleton columns={SKELETON_COLUMNS} count={2} />
          </TableBody>
        </Table>
      </div>
    </Section>
  );
}
