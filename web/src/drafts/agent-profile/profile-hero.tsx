/** The agent's header: who it is, edited in place, and what it's doing right now. */
import { Pill, StatusDot } from "@compozy/ui";
import type { ReactNode } from "react";

import { KindPill } from "../stays-on-agents-shared/agent-page";
import type { AgentState } from "../stays-on-agents-shared/agent-state";
import { AvatarEditor, InlineText, type AvatarMode } from "./identity-editor";
import type { ProfileData } from "./profile-fixtures";

export interface HeroEdit {
  /** Opens the picture picker on this tab. */
  popover?: AvatarMode;
  popoverColor?: string;
  /** Opens the words for editing. */
  editing?: boolean;
  /** Opens them already too long. */
  invalid?: boolean;
}

const TOO_LONG_NAME =
  "Release Captain for the compozy, reloaddev and advos workspaces and everything after";

export function Hero({
  data,
  state,
  line,
  action,
  edit,
}: {
  data: ProfileData;
  state: AgentState;
  line: string;
  action: ReactNode;
  edit?: HeroEdit;
}) {
  const { agent } = data;
  const readOnly = data.readOnly ?? false;
  const live = state === "working" || state === "needs-you";
  const editing = edit?.editing || edit?.invalid;
  return (
    <header className="flex flex-wrap items-center gap-5">
      <AvatarEditor
        agent={agent}
        popover={edit?.popover}
        popoverColor={edit?.popoverColor}
        readOnly={readOnly}
        size={80}
        state={state}
      />
      <div className="flex min-w-0 flex-1 basis-80 flex-col gap-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h2 className={`flex min-w-0 flex-col ${editing ? "flex-1 basis-72" : ""}`}>
            <InlineText
              draft={edit?.invalid ? TOO_LONG_NAME : undefined}
              editing={editing}
              label="Name"
              max={80}
              readOnly={readOnly}
              required
              textClassName="text-ui-title-lg font-medium text-fg"
              value={agent.label}
            />
          </h2>
          <KindPill agent={agent} />
          {data.readOnly ? (
            <Pill form="plain" size="sm">
              From {agent.owner}
            </Pill>
          ) : null}
        </div>
        <InlineText
          draft={edit?.invalid ? `${agent.description}. `.repeat(8) : undefined}
          editing={edit?.invalid}
          label="Description"
          max={280}
          multiline
          placeholder="Add what it does"
          readOnly={readOnly}
          textClassName="text-body text-muted-foreground"
          value={agent.description}
        />
        <p
          className={`flex items-center gap-2 text-body ${live ? "text-fg" : "text-muted-foreground"}`}
        >
          <StatusDot
            size="sm"
            tone={state === "working" ? "success" : state === "needs-you" ? "brand" : "faint"}
          />
          {line}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">{action}</div>
    </header>
  );
}
