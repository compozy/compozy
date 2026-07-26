import type { ReactNode } from "react";
import { Wrench } from "lucide-react";
import {
  Empty,
  Section,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@agh/ui";

interface SettingsDisabledSkillsSectionProps {
  baselineDisabled: readonly string[];
  controls: ReactNode;
  disabled: readonly string[];
  emptyDescription: string;
  emptyTitle: string;
  note: string;
  onToggle: (name: string) => void;
}

export function SettingsDisabledSkillsSection({
  baselineDisabled,
  controls,
  disabled,
  emptyDescription,
  emptyTitle,
  note,
  onToggle,
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
        <div
          className="overflow-hidden rounded-lg border border-line"
          data-testid="settings-page-skills-disabled-list"
        >
          <Table>
            <TableHeader>
              <TableRow className="bg-elevated">
                <TableHead className="eyebrow text-muted">Skill</TableHead>
                <TableHead className="eyebrow text-muted">Identifier</TableHead>
                <TableHead className="eyebrow w-[1%] text-right text-muted">Disabled</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {candidates.map(name => (
                <TableRow key={name} data-testid={`settings-page-skills-disabled-item-${name}`}>
                  <TableCell>
                    <div className="flex min-w-0 items-center gap-2">
                      <Wrench className="size-3 text-subtle" />
                      <span className="truncate text-sm text-fg">{name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted">{name}</TableCell>
                  <TableCell className="text-right">
                    <Switch
                      data-testid={`settings-page-skills-disabled-toggle-${name}`}
                      checked={disabledSkills.has(name)}
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
