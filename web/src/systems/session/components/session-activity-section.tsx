import {
  Activity,
  CircleStop,
  ListPlus,
  LoaderCircle,
  Target,
  TriangleAlert,
  Users,
  Wrench,
} from "lucide-react";
import { Empty, Eyebrow } from "@compozy/ui";
import {
  useSessionContextActivity,
  type SessionContextActivitySource,
} from "../hooks/use-session-context-activity";

export interface SessionActivityView {
  status?: string;
  agents?: string;
  tools?: string;
  thoughts?: string;
  queued?: string;
  goal?: string;
  warning?: string;
}

/** The clock and transcript subscription only mount with this sidebar leaf. */
export function SessionContextLiveActivity({ source }: { source: SessionContextActivitySource }) {
  const activity = useSessionContextActivity(
    source.session,
    source.running,
    source.live,
    source.queued,
    source.goal
  );
  return <SessionActivitySection activity={activity} />;
}

export function SessionActivitySection({ activity = {} }: { activity?: SessionActivityView }) {
  const counts = [activity.tools, activity.thoughts].filter(Boolean).join(" · ");
  const rows = [
    {
      key: "status",
      text: activity.status,
      Icon: activity.status?.startsWith("Working") ? LoaderCircle : CircleStop,
    },
    { key: "agents", text: activity.agents, Icon: Users },
    { key: "counts", text: counts, Icon: Wrench },
    { key: "queued", text: activity.queued, Icon: ListPlus },
    { key: "goal", text: activity.goal, Icon: Target },
    { key: "warning", text: activity.warning, Icon: TriangleAlert },
  ].filter(row => !!row.text);
  return (
    <section className="flex flex-col gap-3" data-testid="session-context-activity">
      <Eyebrow>Activity</Eyebrow>
      {rows.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {rows.map(({ key, text, Icon }) => (
            <li
              key={key}
              className={`flex items-start gap-2 text-small-body ${key === "warning" ? "text-warning" : "text-muted"}`}
            >
              <Icon aria-hidden="true" className="mt-0.5 size-3 shrink-0" />
              <span>{text}</span>
            </li>
          ))}
        </ul>
      ) : (
        <Empty icon={Activity} title="No activity yet" />
      )}
    </section>
  );
}
