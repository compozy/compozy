import {
  CircleStop,
  ListPlus,
  LoaderCircle,
  Target,
  TriangleAlert,
  Users,
  Wrench,
} from "lucide-react";
import { Eyebrow, cn } from "@compozy/ui";
import type { SessionActivityView } from "../lib/session-activity-view";
import { SessionInspectorSection } from "./session-inspector-section";

export type { SessionActivityView } from "../lib/session-activity-view";

/** The status sentence leads with its headline ("Working for 49m 20s") before the first separator. */
function splitLead(text: string): { lead: string; rest: string } {
  const index = text.indexOf(" · ");
  return index === -1
    ? { lead: text, rest: "" }
    : { lead: text.slice(0, index), rest: text.slice(index) };
}

export function SessionActivitySection({ activity = {} }: { activity?: SessionActivityView }) {
  const counts = [activity.tools, activity.thoughts].filter(Boolean).join(" · ");
  const rows = [
    {
      key: "status",
      text: activity.status,
      lead: true,
      Icon: activity.status?.startsWith("Working") ? LoaderCircle : CircleStop,
    },
    { key: "agents", text: activity.agents, Icon: Users },
    { key: "counts", text: counts, Icon: Wrench },
    { key: "queued", text: activity.queued, Icon: ListPlus },
    { key: "goal", text: activity.goal, Icon: Target },
    { key: "warning", text: activity.warning, Icon: TriangleAlert },
  ].filter((row): row is typeof row & { text: string } => !!row.text);
  // A signal that is absent has no row; with no signal at all the section keeps its slot but draws nothing.
  return (
    <SessionInspectorSection data-testid="session-context-activity" hidden={rows.length === 0}>
      <Eyebrow>Activity</Eyebrow>
      {rows.length > 0 ? (
        <ul className="flex flex-col gap-1.5">
          {rows.map(({ key, text, lead, Icon }) => {
            const { lead: headline, rest } = lead ? splitLead(text) : { lead: "", rest: text };
            return (
              <li
                key={key}
                data-kind={key === "warning" ? "warning" : undefined}
                className="grid grid-cols-[14px_minmax(0,1fr)] items-start gap-2 text-form-label leading-[1.45] text-muted"
              >
                <Icon
                  aria-hidden="true"
                  className={cn(
                    "mt-0.5 size-3.25",
                    key === "warning" ? "text-warning" : "text-subtle"
                  )}
                />
                <span className="min-w-0 break-words">
                  {headline ? <b className="font-medium text-fg">{headline}</b> : null}
                  {rest}
                </span>
              </li>
            );
          })}
        </ul>
      ) : null}
    </SessionInspectorSection>
  );
}
