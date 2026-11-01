/** How the agent is set up: one quiet list, every row one click from the thing it describes. */
import { protoHref } from "@compozy/proto";
import {
  Bot,
  Brain,
  ChevronRight,
  Clock,
  Fingerprint,
  Lock,
  Sparkles,
  Wrench,
  type LucideIcon,
} from "lucide-react";

import { ListingRow, Panel } from "@compozy/ui";

import type { DraftAgent } from "../stays-on-agents-shared/fixtures";

export type ProfilePanel = "identity" | "model" | "tools";

function SetupRow({
  icon: Icon,
  title,
  summary,
  onOpen,
  locked,
}: {
  icon: LucideIcon;
  title: string;
  summary: string;
  onOpen: () => void;
  locked: boolean;
}) {
  return (
    <ListingRow>
      <ListingRow.Link
        render={<button aria-label={`${title}: ${summary}`} onClick={onOpen} type="button" />}
      >
        <ListingRow.Icon>
          <Icon aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{title}</ListingRow.Title>
          </ListingRow.Name>
          <ListingRow.Meta>
            <span className="truncate">{summary}</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        {locked ? (
          <Lock aria-hidden="true" className="size-3.5 text-faint" />
        ) : (
          <ChevronRight aria-hidden="true" className="size-4 text-faint" />
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}

function editor(state: string) {
  return () => window.location.assign(protoHref("agent-editors", state));
}

export function SetupPanel({
  agent,
  readOnly = false,
  onOpen,
}: {
  agent: DraftAgent;
  readOnly?: boolean;
  onOpen: (panel: ProfilePanel) => void;
}) {
  const staysOn = agent.kind === "stays_on";
  return (
    <Panel
      bodyClassName="p-0"
      meta={readOnly ? `From ${agent.owner}` : undefined}
      title="How it's set up"
    >
      <div className="flex flex-col divide-y divide-line-soft">
        <SetupRow
          icon={Fingerprint}
          locked={readOnly}
          onOpen={() => onOpen("identity")}
          summary="Name, picture and description"
          title="Name and look"
        />
        <SetupRow
          icon={Sparkles}
          locked={readOnly}
          onOpen={editor("what")}
          summary="Direct, and flags what blocks the work"
          title="How it behaves"
        />
        <SetupRow
          icon={Bot}
          locked={readOnly}
          onOpen={() => onOpen("model")}
          summary={agent.provider === "codex" ? "GPT 6 Astra" : "Claude Opus 5.5"}
          title="Model"
        />
        {staysOn ? (
          <SetupRow
            icon={Clock}
            locked={readOnly}
            onOpen={editor("checkins")}
            summary="Every 30 min, 9:00 to 19:00"
            title="Check-ins"
          />
        ) : null}
        {staysOn ? (
          <SetupRow
            icon={Brain}
            locked={readOnly}
            onOpen={editor("memory")}
            summary="14 notes"
            title="What it remembers"
          />
        ) : null}
        <SetupRow
          icon={Wrench}
          locked={readOnly}
          onOpen={() => onOpen("tools")}
          summary="12 tools in this project"
          title="What it can use"
        />
      </div>
    </Panel>
  );
}
