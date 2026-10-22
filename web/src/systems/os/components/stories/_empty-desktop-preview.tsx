import { useEffect, useState } from "react";
import { fn } from "storybook/test";

import { storyAgentNames, storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { agentFixtures } from "@/systems/agent/mocks";
import {
  sessionStore,
  SessionDraftRuntimeProvider,
  SessionPromptRuntimeSelector,
} from "@/systems/session";

import { emptyDesktopDraftId } from "../../hooks/use-empty-desktop-prompt-runtime";
import { shortcutLabel } from "../../lib/window-manager-shortcuts";
import { OsEmptyDesktopView } from "../os-empty-desktop";

// Agents carry a category path, as seeded workspaces do: the composer trigger
// must still lead with the full agent name and leave the path to the list.
const PREVIEW_AGENTS = agentFixtures.map(agent => ({
  ...agent,
  category_path: ["Operations", "Launch week"],
}));

const submitPrompt = fn().mockName("onSubmit");

export interface EmptyDesktopPreviewProps {
  /** All projects: New session asks for the project and agent, so the composer has no agent control. */
  asksTarget?: boolean;
  /** A session is starting from the sent prompt. */
  starting?: boolean;
  /** Text in the composer (the sent prompt while starting); empty by default. */
  prompt?: string;
}

/**
 * Story-only empty desktop: the production view and session composer on a
 * transcript-less runtime, with the send recorded as an action instead of
 * reaching a daemon.
 */
export function EmptyDesktopPreview({
  asksTarget = false,
  starting = false,
  prompt = "",
}: EmptyDesktopPreviewProps) {
  const draftId = emptyDesktopDraftId(asksTarget ? null : storyDefaultWorkspaceId);
  const [agentName, setAgentName] = useState<string | null>(storyAgentNames.product);
  // Every story owns its draft slot's text, so one story's prompt never leaks into the next.
  useEffect(() => {
    if (prompt === "") sessionStore.trigger.composerDraftDiscarded({ sessionId: draftId });
    else sessionStore.trigger.composerDraftChanged({ sessionId: draftId, text: prompt });
  }, [draftId, prompt]);
  return (
    <SessionDraftRuntimeProvider
      draftId={draftId}
      workspaceId={storyDefaultWorkspaceId}
      agentName={agentName ?? ""}
      canPrompt={!starting}
    >
      <OsEmptyDesktopView
        desktopName="Desktop 1"
        paletteShortcutLabel={shortcutLabel("meta+KeyK")}
        canStart
        asksTarget={asksTarget}
        starting={starting}
        draftId={draftId}
        onSubmit={text => submitPrompt(text)}
        agents={PREVIEW_AGENTS}
        agentName={agentName}
        onAgentChange={setAgentName}
        runtimeControl={
          asksTarget ? undefined : <SessionPromptRuntimeSelector canPrompt={!starting} />
        }
      />
    </SessionDraftRuntimeProvider>
  );
}
