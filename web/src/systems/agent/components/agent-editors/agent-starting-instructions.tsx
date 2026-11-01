/** AGENT.md's own instructions, which the create screen writes and What it does sits on top of. */
import { useState } from "react";

import { Button, Disclosure, Field, Textarea } from "@compozy/ui";

import { useAgentSettingsPage } from "../../hooks/use-agent-settings-page";
import { useAgentSoul } from "../../hooks/use-agent-soul";
import type { AgentPayload } from "../../types";

export function AgentStartingInstructions({
  agent,
  workspace,
  readOnly,
}: {
  agent: AgentPayload;
  workspace: string | null;
  readOnly: boolean;
}) {
  const page = useAgentSettingsPage({ name: agent.name, section: "instructions" });
  const soul = useAgentSoul(agent.name, workspace);
  const [opened, setOpened] = useState<boolean | null>(null);
  if (!page.draft || !soul.data) return null;
  // An agent with no file yet does what these say, so they lead until there's one.
  const open = opened ?? !soul.data.present;
  return (
    <Disclosure label="Starting instructions" onOpenChange={setOpened} open={open} size="md">
      <Field>
        <Textarea
          aria-label="Starting instructions"
          className="min-h-40"
          data-testid="agent-starting-instructions"
          onChange={event => page.patchDraft({ prompt: event.target.value })}
          readOnly={readOnly}
          value={page.draft.prompt}
        />
        {page.dirty ? (
          <div className="flex justify-end gap-2">
            <Button onClick={page.onDiscard} size="sm" type="button" variant="ghost">
              Discard
            </Button>
            <Button
              disabled={page.saveBlocked || page.phase === "saving"}
              onClick={page.onSave}
              size="sm"
              type="button"
            >
              Save
            </Button>
          </div>
        ) : null}
      </Field>
    </Disclosure>
  );
}
