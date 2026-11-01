/** The agent saved while the person was editing: both versions, side by side, and one choice. */
import { CircleAlert } from "lucide-react";
import type { ReactNode } from "react";

import { Button, Eyebrow } from "@compozy/ui";

import { EditorCard } from "../stays-on-agents-shared/rich-controls";

function Version({
  name,
  action,
  onChoose,
  children,
}: {
  name: string;
  action: string;
  onChoose: () => void;
  children: ReactNode;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-3 rounded-md bg-sunken p-4">
      <Eyebrow>{name}</Eyebrow>
      <div className="min-w-0 flex-1 text-small-body leading-relaxed text-fg-2">{children}</div>
      <Button onClick={onChoose} size="sm" type="button" variant="secondary">
        {action}
      </Button>
    </div>
  );
}

export function ConflictCard({
  agentLabel,
  title,
  mine,
  theirs,
  onKeepMine,
  onUseTheirs,
}: {
  agentLabel: string;
  title: string;
  mine: ReactNode;
  theirs: ReactNode;
  onKeepMine: () => void;
  onUseTheirs: () => void;
}) {
  return (
    <EditorCard className="gap-4">
      <div className="flex items-start gap-3">
        <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-warning" />
        <div className="flex min-w-0 flex-col gap-0.5">
          <h2 className="text-item-title font-medium text-fg">{title}</h2>
          <p className="text-small-body text-muted">
            Nothing is lost yet. Pick the version you want to keep.
          </p>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <Version action="Keep mine" name="Your version" onChoose={onKeepMine}>
          {mine}
        </Version>
        <Version action="Use theirs" name={`${agentLabel}'s version`} onChoose={onUseTheirs}>
          {theirs}
        </Version>
      </div>
    </EditorCard>
  );
}
