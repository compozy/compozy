/** The focused side panels: each edits one thing with the product's own control. */
import { useState } from "react";

import {
  Button,
  Field,
  FieldLabel,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@compozy/ui";

import { TokenListField } from "@/systems/agent/components/token-list-field";
import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector/runtime-selector";
import type { RuntimeSelectorValue } from "@/systems/runtime/components/runtime-selector/types";
import {
  runtimeSelectorStoryModels,
  runtimeSelectorStoryProviders,
} from "@/systems/runtime/components/stories/runtime-selector-story-fixtures";

import { DraftUnsavedPill } from "../stays-on-agents-shared/draft-shell";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";
import { PlanCard } from "./profile-activity";
import type { ProfilePanel } from "./profile-setup";

function PanelFoot({ dirty, onClose }: { dirty: boolean; onClose: () => void }) {
  return (
    <SheetFooter className="flex-row items-center justify-end gap-2">
      {dirty ? (
        <span className="mr-auto">
          <DraftUnsavedPill count={1} />
        </span>
      ) : null}
      <Button onClick={onClose} size="sm" type="button" variant="ghost">
        {dirty ? "Discard" : "Close"}
      </Button>
      <Button disabled={!dirty} onClick={onClose} size="sm" type="button">
        Save
      </Button>
    </SheetFooter>
  );
}

function ModelBody({
  agent,
  readOnly,
  onDirty,
}: {
  agent: DraftAgent;
  readOnly: boolean;
  onDirty: () => void;
}) {
  const [value, setValue] = useState<RuntimeSelectorValue>({
    provider: agent.provider,
    model: agent.provider === "codex" ? "gpt-5.6-sol" : "claude-fable-5",
    reasoning_effort: "",
  });
  return (
    <div className="flex flex-col gap-5">
      <Field>
        <FieldLabel>Model</FieldLabel>
        <RuntimeSelector
          models={runtimeSelectorStoryModels}
          onChange={next => {
            setValue(next);
            onDirty();
          }}
          providers={runtimeSelectorStoryProviders}
          readOnly={readOnly}
          value={value}
        />
      </Field>
      <PlanCard />
    </div>
  );
}

const TOOLS = ["Read files", "Edit files", "Run commands", "Search the web", "GitHub"];

function ToolsBody({ readOnly, onDirty }: { readOnly: boolean; onDirty: () => void }) {
  const [tools, setTools] = useState<string[]>(TOOLS);
  return (
    <TokenListField
      label="Tools"
      onChange={next => {
        setTools(next);
        onDirty();
      }}
      placeholder="Add a tool"
      readOnly={readOnly}
      testId="agent-tools"
      values={tools}
    />
  );
}

const TITLES: Record<ProfilePanel, string> = {
  model: "Model",
  tools: "What it can use",
};

export function ProfilePanels({
  agent,
  panel,
  readOnly,
  onClose,
}: {
  agent: DraftAgent;
  panel: ProfilePanel | null;
  readOnly: boolean;
  onClose: () => void;
}) {
  const [dirty, setDirty] = useState(false);
  const title = panel ? TITLES[panel] : "";
  const close = () => {
    setDirty(false);
    onClose();
  };
  return (
    <Sheet onOpenChange={open => (open ? undefined : close())} open={panel !== null}>
      <SheetContent className="flex flex-col sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>
            {readOnly
              ? `${agent.owner} looks after this agent, so this can't be changed here.`
              : agent.label}
          </SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4">
          {panel === "model" ? (
            <ModelBody agent={agent} onDirty={() => setDirty(true)} readOnly={readOnly} />
          ) : (
            <ToolsBody onDirty={() => setDirty(true)} readOnly={readOnly} />
          )}
        </div>
        {readOnly ? null : <PanelFoot dirty={dirty} onClose={close} />}
      </SheetContent>
    </Sheet>
  );
}
