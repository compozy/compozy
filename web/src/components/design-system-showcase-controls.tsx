import { useState } from "react";
import {
  Button,
  ButtonGroup,
  ConnectionIndicator,
  Eyebrow,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
  Kbd,
  KbdGroup,
  Label,
  Metric,
  NativeSelect,
  NativeSelectOption,
  Pill,
  PillGroup,
  Progress,
  ProgressLabel,
  ProgressValue,
  SearchInput,
  Section,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Separator,
  Skeleton,
  Spinner,
  Switch,
  Textarea,
  ToggleGroup,
  ToggleGroupItem,
} from "@agh/ui";
import { PlusIcon, SearchIcon } from "lucide-react";
import { KindChip } from "@/systems/network";

import { KINDS } from "./design-system-showcase-examples";
import { SectionLink } from "./design-system-showcase-section-link";
import { sectionById } from "./design-system-showcase-sections";

export function ButtonsAndPillsSection() {
  const [pillFilter, setPillFilter] = useState("active");
  return (
    <Section
      id="buttons"
      data-testid="section-buttons"
      label={<SectionLink section={sectionById("buttons")}>Buttons & Pills</SectionLink>}
      right={
        <Pill mono tone="accent">
          action
        </Pill>
      }
    >
      <div className="flex flex-col gap-6 pt-4">
        <div className="flex flex-wrap items-center gap-3">
          <Button>Primary</Button>
          <Button variant="outline">Outline</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="destructive">Destructive</Button>
          <Button variant="link">Link</Button>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Button size="xs">XS</Button>
          <Button size="sm">Small</Button>
          <Button>Default</Button>
          <Button size="lg">Large</Button>
          <Button size="icon" aria-label="Icon action">
            <PlusIcon />
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <ButtonGroup>
            <Button variant="outline">Day</Button>
            <Button variant="outline">Week</Button>
            <Button variant="outline">Month</Button>
          </ButtonGroup>
          <KbdGroup>
            <Kbd>⌘</Kbd>
            <Kbd>K</Kbd>
          </KbdGroup>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <Pill tone="neutral">Neutral</Pill>
          <Pill tone="accent">Action</Pill>
          <Pill tone="success">Stable</Pill>
          <Pill tone="warning">Pending</Pill>
          <Pill tone="danger">Error</Pill>
          <Pill tone="info">Info</Pill>
        </div>
        <PillGroup
          value={pillFilter}
          onChange={setPillFilter}
          items={[
            { label: "Active", value: "active", badge: 3 },
            { label: "Queued", value: "queued" },
            { label: "Done", value: "done", badge: 12 },
            { label: "Archived", value: "archived", disabled: true },
          ]}
        />
      </div>
    </Section>
  );
}

export function InputsAndSearchSection() {
  const [textareaValue, setTextareaValue] = useState(
    "Multiline textarea — Inter 14px, 1.6 leading."
  );

  return (
    <Section
      id="inputs"
      data-testid="section-inputs"
      label={<SectionLink section={sectionById("inputs")}>Inputs & Search</SectionLink>}
      right={<Pill mono>form primitives</Pill>}
    >
      <div className="grid gap-6 pt-4 md:grid-cols-2">
        <Field>
          <FieldLabel htmlFor="showcase-name">Display name</FieldLabel>
          <Input id="showcase-name" placeholder="e.g. agh-core" />
          <FieldDescription>Used across session headers and agent metadata.</FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="showcase-notes">Notes</FieldLabel>
          <Textarea
            id="showcase-notes"
            rows={3}
            value={textareaValue}
            onChange={event => setTextareaValue(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="showcase-agent">Agent</FieldLabel>
          <Select defaultValue="claude">
            <SelectTrigger id="showcase-agent">
              <SelectValue placeholder="Select an agent" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="claude">Claude Code</SelectItem>
              <SelectItem value="codex">Codex CLI</SelectItem>
              <SelectItem value="gemini">Gemini CLI</SelectItem>
            </SelectContent>
          </Select>
        </Field>
        <Field>
          <FieldLabel htmlFor="showcase-env">Environment</FieldLabel>
          <NativeSelect id="showcase-env" defaultValue="dev">
            <NativeSelectOption value="dev">Development</NativeSelectOption>
            <NativeSelectOption value="stage">Staging</NativeSelectOption>
            <NativeSelectOption value="prod">Production</NativeSelectOption>
          </NativeSelect>
        </Field>
        <InputGroup>
          <InputGroupAddon align="inline-start">
            <SearchIcon className="size-3" />
          </InputGroupAddon>
          <InputGroupInput placeholder="Filter sessions…" />
          <InputGroupAddon align="inline-end">
            <InputGroupText>
              <Kbd>⌘</Kbd>
              <Kbd>F</Kbd>
            </InputGroupText>
          </InputGroupAddon>
        </InputGroup>
        <SearchInput
          placeholder="Search skills…"
          kbd={
            <KbdGroup>
              <Kbd>⌘</Kbd>
              <Kbd>K</Kbd>
            </KbdGroup>
          }
        />
        <div className="flex flex-col gap-3">
          <Label>Toggle group</Label>
          <ToggleGroup defaultValue={["tasks"]}>
            <ToggleGroupItem value="tasks" aria-label="Tasks">
              Tasks
            </ToggleGroupItem>
            <ToggleGroupItem value="sessions" aria-label="Sessions">
              Sessions
            </ToggleGroupItem>
            <ToggleGroupItem value="skills" aria-label="Skills">
              Skills
            </ToggleGroupItem>
          </ToggleGroup>
        </div>
        <div className="flex items-center gap-3">
          <Switch defaultChecked id="showcase-switch" />
          <Label htmlFor="showcase-switch">Autostart agents on boot</Label>
        </div>
      </div>
    </Section>
  );
}

export function StatusAndMetricSection() {
  return (
    <Section
      id="status"
      data-testid="section-status"
      label={
        <SectionLink section={sectionById("status")}>
          Status, Metric, MonoBadge, KindChip
        </SectionLink>
      }
      right={
        <Pill mono tone="info">
          signal
        </Pill>
      }
    >
      <div className="grid gap-3 pt-4 md:grid-cols-3">
        <Metric label="Active sessions" value="12" detail="+3" tone="accent" />
        <Metric
          label="Throughput"
          value="248"
          subtext="Messages / minute across all workspaces."
          tone="success"
        />
        <Metric label="Error rate" value="0.4%" subtext="Last 24h · within SLO" tone="warning" />
      </div>
      <div className="flex flex-wrap items-center gap-4 pt-4">
        <div className="inline-flex items-center gap-2">
          <Pill.Dot tone="success" />
          <span className="text-sm text-muted">Connected</span>
        </div>
        <div className="inline-flex items-center gap-2">
          <Pill.Dot tone="warning" pulse />
          <span className="text-sm text-muted">Connecting</span>
        </div>
        <div className="inline-flex items-center gap-2">
          <Pill.Dot tone="danger" />
          <span className="text-sm text-muted">Disconnected</span>
        </div>
        <ConnectionIndicator status="connected" />
        <ConnectionIndicator status="connecting" />
        <ConnectionIndicator status="disconnected" />
      </div>
      <div className="flex flex-wrap items-center gap-2 pt-4">
        <Pill mono tone="neutral">
          id_01HQ…
        </Pill>
        <Pill mono tone="neutral">
          idle
        </Pill>
        <Pill mono tone="accent">
          RUNNING
        </Pill>
        <Pill mono tone="success">
          DONE
        </Pill>
        <Pill mono tone="warning">
          PARTIAL
        </Pill>
        <Pill mono tone="danger">
          ERROR
        </Pill>
        <Pill mono tone="info">
          INFO
        </Pill>
      </div>
      <div className="flex flex-wrap items-center gap-2 pt-3">
        {KINDS.map(kind => (
          <KindChip key={kind} kind={kind} />
        ))}
      </div>
      <div className="flex flex-col gap-3 pt-6">
        <div className="flex flex-wrap items-center gap-4">
          <Spinner className="size-4 text-accent" />
          <Skeleton className="h-4 w-40" />
          <Separator orientation="vertical" className="h-6" />
          <Eyebrow className="text-subtle">spinners · skeletons · separators</Eyebrow>
        </div>
        <Progress value={64} data-testid="showcase-progress">
          <ProgressLabel>Skill index rebuild</ProgressLabel>
          <ProgressValue />
        </Progress>
      </div>
    </Section>
  );
}
