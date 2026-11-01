/** How the agent is set up: one quiet list, every row one click from the thing it describes. */
import {
  Bot,
  Brain,
  ChevronRight,
  Clock,
  Lock,
  Sparkles,
  Wrench,
  type LucideIcon,
} from "@compozy/ui/icons";

import { Panel, Section, cn } from "@compozy/ui";

import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "@/systems/dashboard";

function SetupRow({
  icon: Icon,
  title,
  summary,
  onOpen,
  locked,
  testId,
}: {
  icon: LucideIcon;
  title: string;
  summary: string;
  onOpen: () => void;
  locked: boolean;
  testId: string;
}) {
  const Trail = locked ? Lock : ChevronRight;
  return (
    <button
      className={cn(
        HOME_RUN_ROW_CLASS,
        "w-full text-left transition-colors duration-base hover:bg-surface-2 focus-visible:shadow-focus-inset focus-visible:outline-none"
      )}
      data-testid={testId}
      onClick={onOpen}
      type="button"
    >
      <span className="min-w-0">
        <span className={HOME_RUN_TITLE_CLASS}>
          <Icon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{title}</span>
        </span>
        <span className={HOME_RUN_SUBTITLE_CLASS}>
          <span className="truncate pl-6">{summary}</span>
        </span>
      </span>
      <Trail aria-hidden="true" className="size-3.5 text-faint" />
    </button>
  );
}

export interface AgentSetupSummaries {
  behavior: string;
  model: string;
  checkIns: string;
  memory: string;
  tools: string;
}

export function AgentProfileSetup({
  staysOn,
  owner,
  readOnly,
  summaries,
  onOpenBehavior,
  onOpenModel,
  onOpenCheckIns,
  onOpenMemory,
  onOpenTools,
}: {
  staysOn: boolean;
  owner?: string;
  readOnly: boolean;
  summaries: AgentSetupSummaries;
  onOpenBehavior: () => void;
  onOpenModel: () => void;
  onOpenCheckIns: () => void;
  onOpenMemory: () => void;
  onOpenTools: () => void;
}) {
  return (
    <Section
      label="How it's set up"
      right={
        readOnly && owner ? (
          <span className="text-eyebrow text-muted-foreground">From {owner}</span>
        ) : undefined
      }
    >
      <Panel bodyClassName="p-0" className="overflow-hidden">
        <div className="divide-y divide-line-soft">
          <SetupRow
            icon={Sparkles}
            locked={readOnly}
            onOpen={onOpenBehavior}
            summary={summaries.behavior}
            testId="agent-setup-behavior"
            title="How it behaves"
          />
          <SetupRow
            icon={Bot}
            locked={readOnly}
            onOpen={onOpenModel}
            summary={summaries.model}
            testId="agent-setup-model"
            title="Model"
          />
          {staysOn ? (
            <SetupRow
              icon={Clock}
              locked={readOnly}
              onOpen={onOpenCheckIns}
              summary={summaries.checkIns}
              testId="agent-setup-check-ins"
              title="Check-ins"
            />
          ) : null}
          {staysOn ? (
            <SetupRow
              icon={Brain}
              locked={false}
              onOpen={onOpenMemory}
              summary={summaries.memory}
              testId="agent-setup-memory"
              title="What it remembers"
            />
          ) : null}
          <SetupRow
            icon={Wrench}
            locked={readOnly}
            onOpen={onOpenTools}
            summary={summaries.tools}
            testId="agent-setup-tools"
            title="What it can use"
          />
        </div>
      </Panel>
    </Section>
  );
}
